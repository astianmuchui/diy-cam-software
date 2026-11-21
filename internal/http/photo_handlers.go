package http

import (
	"io"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"github.com/astianmuchui/mobilesonar/internal/env"
	"github.com/astianmuchui/mobilesonar/internal/ml"
	"github.com/astianmuchui/mobilesonar/internal/models"
	"github.com/astianmuchui/mobilesonar/internal/postgres"
)

const photoDir = "./data/photos"

func maxPhotoSize() int64 {
	return int64(env.C.MaxPhotoSizeKB) * 1024
}

func PhotoUploadHandler(c *fiber.Ctx) error {
	readingIDStr := c.FormValue("reading_id")
	if readingIDStr == "" {
		readingIDStr = c.Query("reading_id")
	}
	if readingIDStr == "" {
		readingIDStr = c.Get("X-Reading-ID")
	}
	if readingIDStr == "" {
		log.Printf("photo upload: rejected, missing reading_id (ip=%s ct=%q clen=%d)",
			c.IP(), c.Get("Content-Type"), c.Request().Header.ContentLength())
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "missing reading_id"})
	}

	safeID := filepath.Base(readingIDStr)
	if safeID == "." || safeID == "/" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid reading_id"})
	}

	declaredLen := c.Request().Header.ContentLength()
	contentType := c.Get("Content-Type")
	transferEnc := c.Get("Transfer-Encoding")
	limit := maxPhotoSize()

	var (
		jpeg   []byte
		source string
	)

	file, err := c.FormFile("photo")
	if err == nil {
		source = "multipart"
		log.Printf("photo upload [%s]: src=multipart ip=%s ct=%q declaredLen=%d te=%q multipartSize=%d",
			safeID, c.IP(), contentType, declaredLen, transferEnc, file.Size)

		if file.Size > limit {
			log.Printf("photo upload [%s]: rejected, multipart size %d > limit %d", safeID, file.Size, limit)
			return c.Status(fiber.StatusRequestEntityTooLarge).JSON(fiber.Map{
				"error": "photo exceeds size limit",
				"limit": limit,
				"size":  file.Size,
			})
		}
		f, openErr := file.Open()
		if openErr != nil {
			log.Printf("photo upload [%s]: failed to open multipart file: %v", safeID, openErr)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to open photo"})
		}
		defer f.Close()
		jpeg, err = io.ReadAll(f)
		if err != nil {
			log.Printf("photo upload [%s]: failed reading multipart stream after %d bytes: %v", safeID, len(jpeg), err)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to read photo"})
		}
		if int64(len(jpeg)) != file.Size {
			log.Printf("photo upload [%s]: WARNING incomplete multipart read: declared multipartSize=%d but read %d bytes",
				safeID, file.Size, len(jpeg))
		}
	} else {
		source = "raw-body"
		jpeg = c.Body()
		log.Printf("photo upload [%s]: src=raw-body ip=%s ct=%q declaredLen=%d te=%q bodyLen=%d",
			safeID, c.IP(), contentType, declaredLen, transferEnc, len(jpeg))

		if len(jpeg) == 0 {
			log.Printf("photo upload [%s]: rejected, empty body (declaredLen=%d)", safeID, declaredLen)
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "no photo provided"})
		}
		if int64(len(jpeg)) > limit {
			log.Printf("photo upload [%s]: rejected, raw body %d > limit %d", safeID, len(jpeg), limit)
			return c.Status(fiber.StatusRequestEntityTooLarge).JSON(fiber.Map{
				"error": "photo exceeds size limit",
				"limit": limit,
				"size":  len(jpeg),
			})
		}

		if declaredLen > 0 && len(jpeg) != declaredLen {
			log.Printf("photo upload [%s]: WARNING body length mismatch: Content-Length=%d but received %d bytes (truncated upload?)",
				safeID, declaredLen, len(jpeg))
		}
	}

	if len(jpeg) < 2 || jpeg[0] != 0xFF || jpeg[1] != 0xD8 {
		log.Printf("photo upload [%s]: WARNING payload does not start with JPEG SOI marker (src=%s, first bytes=%x)",
			safeID, source, firstBytes(jpeg, 8))
	}

	if err := os.MkdirAll(photoDir, 0755); err != nil {
		log.Printf("photo upload [%s]: failed to create photo dir: %v", safeID, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to create photo directory"})
	}

	photoPath := filepath.Join(photoDir, safeID+".jpg")
	if err := os.WriteFile(photoPath, jpeg, 0644); err != nil {
		log.Printf("photo upload [%s]: failed to write %s: %v", safeID, photoPath, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to save photo"})
	}

	absPath, _ := filepath.Abs(photoPath)
	cwd, _ := os.Getwd()

	if fi, statErr := os.Stat(photoPath); statErr != nil {
		log.Printf("photo upload [%s]: WARNING could not stat written file: %v (abs=%s cwd=%s)",
			safeID, statErr, absPath, cwd)
	} else if fi.Size() != int64(len(jpeg)) {
		log.Printf("photo upload [%s]: WARNING on-disk size %d != buffered size %d (abs=%s)",
			safeID, fi.Size(), len(jpeg), absPath)
	} else {
		log.Printf("photo upload [%s]: saved ok src=%s bytes=%d relpath=%s abspath=%s cwd=%s",
			safeID, source, len(jpeg), photoPath, absPath, cwd)
	}

	photo := &models.Photo{
		CorrelationID: safeID,
		FilePath:      photoPath,
		SizeBytes:     len(jpeg),
		CapturedAt:    time.Now().UTC(),
	}

	if active, sErr := postgres.GetActiveSession(postgres.DB); sErr == nil && active != nil {
		photo.SessionID = &active.ID
		photo.DeviceID = active.DeviceID
		log.Printf("photo upload [%s]: associated with active session %s (device %s)",
			safeID, active.ID, active.DeviceID)
	} else {
		log.Printf("photo upload [%s]: no active session, photo stored unassociated", safeID)
	}

	if rid, parseErr := uuid.Parse(safeID); parseErr == nil {
		if reading, _ := postgres.GetReadingByID(postgres.DB, rid); reading != nil {
			photo.ReadingID = &reading.ID
			if photo.SessionID == nil {
				photo.SessionID = reading.SessionID
			}
			if photo.DeviceID == "" {
				photo.DeviceID = reading.DeviceID
			}
			_ = postgres.UpdateReadingPhoto(postgres.DB, reading.ID, photoPath)
		}
	}

	if dbErr := postgres.CreatePhoto(postgres.DB, photo); dbErr != nil {
		log.Printf("photo upload [%s]: failed to create photo row: %v", safeID, dbErr)
	} else {
		log.Printf("photo upload [%s]: photo row %s created (session=%v reading=%v)",
			safeID, photo.ID, photo.SessionID, photo.ReadingID)
	}

	jpegCopy := make([]byte, len(jpeg))
	copy(jpegCopy, jpeg)
	photoID := photo.ID
	readingID := photo.ReadingID
	sessionID := photo.SessionID
	go func() {
		result, classErr := ml.ClassifyPhoto(safeID, jpegCopy)
		if classErr != nil {
			log.Printf("photo cnn [photo %s]: %v", photoID, classErr)
			return
		}
		if dbErr := postgres.UpdatePhotoCNNResult(
			postgres.DB, photoID,
			result.IsFish, result.FishConfidence, result.SceneClass,
			result.Species, result.SpeciesConfidence, result.FishCount,
		); dbErr != nil {
			log.Printf("photo cnn [photo %s]: update failed: %v", photoID, dbErr)
		}
		if readingID != nil {
			if dbErr := postgres.UpdateReadingCNNResult(
				postgres.DB, *readingID,
				result.IsFish, result.FishConfidence, result.SceneClass,
				result.Species, result.SpeciesConfidence, result.FishCount,
			); dbErr != nil {
				log.Printf("photo cnn [reading %s]: update failed: %v", *readingID, dbErr)
			}
		}
		if sessionID != nil {
			if total, recErr := postgres.RecomputeSessionTotalFish(postgres.DB, *sessionID); recErr != nil {
				log.Printf("photo cnn [photo %s]: session total recompute failed: %v", photoID, recErr)
			} else {
				log.Printf("photo cnn [photo %s]: session %s total_fish=%d (fish_count=%d species=%v)",
					photoID, *sessionID, total, result.FishCount, result.SpeciesBreakdown)
			}
		}
	}()

	return c.JSON(fiber.Map{
		"status":     "saved",
		"reading_id": safeID,
		"size":       len(jpeg),
	})
}

func firstBytes(b []byte, n int) []byte {
	if len(b) < n {
		return b
	}
	return b[:n]
}

func PhotoRetrieveHandler(c *fiber.Ctx) error {
	readingID, err := uuid.Parse(c.Params("reading_id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid reading_id"})
	}

	cwd, _ := os.Getwd()

	if photo, pErr := postgres.GetPhotoByID(postgres.DB, readingID); pErr == nil && photo != nil && photo.FilePath != "" {
		abs, _ := filepath.Abs(photo.FilePath)
		if _, statErr := os.Stat(photo.FilePath); statErr == nil {
			log.Printf("photo retrieve [%s]: served from photos.file_path=%q (abs=%s)",
				readingID, photo.FilePath, abs)
			c.Set("Content-Type", "image/jpeg")
			return c.SendFile(photo.FilePath)
		}
		log.Printf("photo retrieve [%s]: photo row found but file missing at %q (abs=%s, cwd=%s)",
			readingID, photo.FilePath, abs, cwd)
	}

	photoPath := filepath.Join(photoDir, readingID.String()+".jpg")
	if _, statErr := os.Stat(photoPath); statErr == nil {
		abs, _ := filepath.Abs(photoPath)
		log.Printf("photo retrieve [%s]: serving %s (abs=%s)", readingID, photoPath, abs)
		c.Set("Content-Type", "image/jpeg")
		return c.SendFile(photoPath)
	}

	if reading, dbErr := postgres.GetReadingByID(postgres.DB, readingID); dbErr == nil && reading != nil {
		if reading.PhotoPath != "" {
			abs, _ := filepath.Abs(reading.PhotoPath)
			if _, statErr := os.Stat(reading.PhotoPath); statErr == nil {
				log.Printf("photo retrieve [%s]: primary %q missing, served from db photo_path=%q (abs=%s)",
					readingID, photoPath, reading.PhotoPath, abs)
				c.Set("Content-Type", "image/jpeg")
				return c.SendFile(reading.PhotoPath)
			}
			log.Printf("photo retrieve [%s]: NOT FOUND. primary=%q (missing) db photo_path=%q (abs=%s, missing) cwd=%s",
				readingID, photoPath, reading.PhotoPath, abs, cwd)
		} else {
			log.Printf("photo retrieve [%s]: NOT FOUND. primary=%q missing and reading has empty photo_path (cwd=%s)",
				readingID, photoPath, cwd)
		}
	} else {
		log.Printf("photo retrieve [%s]: NOT FOUND. primary=%q missing and no reading row (cwd=%s)",
			readingID, photoPath, cwd)
	}

	return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "photo not found"})
}

func AdminPhotoHandler(c *fiber.Ctx) error {
	return PhotoRetrieveHandler(c)
}
