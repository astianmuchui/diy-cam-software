package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/astianmuchui/mobilesonar/external/nasa"
	"github.com/astianmuchui/mobilesonar/internal/detection"
	"github.com/astianmuchui/mobilesonar/internal/ml"
	"github.com/astianmuchui/mobilesonar/internal/models"
	"github.com/astianmuchui/mobilesonar/internal/postgres"
	rdb "github.com/astianmuchui/mobilesonar/internal/redis"
	"github.com/astianmuchui/mobilesonar/internal/stream"
)

func StartReadingWorker(ctx context.Context, db *gorm.DB, eng *detection.Engine) {
	log.Println("worker: reading worker started")
	for {
		select {
		case <-ctx.Done():
			log.Println("worker: reading worker stopped")
			return
		default:
		}

		messages, err := stream.ReadGroup(ctx, stream.StreamReadings, stream.ReadingGroup, stream.ReadingConsumer, 10, 5*time.Second)
		if err != nil {
			continue
		}

		for _, msg := range messages {
			if stream.RetryCount(ctx, stream.StreamReadings, stream.ReadingGroup, msg.ID) > stream.MaxRetries {
				log.Printf("worker: dead-letter [%s] after %d retries", msg.ID, stream.MaxRetries)
				stream.Ack(ctx, stream.StreamReadings, stream.ReadingGroup, msg.ID)
				continue
			}

			if err := processReading(ctx, db, eng, msg.Values); err != nil {
				log.Printf("worker: reading failed [%s]: %v", msg.ID, err)
				continue
			}
			stream.Ack(ctx, stream.StreamReadings, stream.ReadingGroup, msg.ID)
		}
	}
}

func processReading(ctx context.Context, db *gorm.DB, eng *detection.Engine, v map[string]any) error {
	accelX := stream.ParseFloat32(v, "accel_x")
	accelY := stream.ParseFloat32(v, "accel_y")
	accelZ := stream.ParseFloat32(v, "accel_z")

	tiltAngle := float32(math.Atan2(
		math.Sqrt(float64(accelX*accelX+accelY*accelY)),
		math.Abs(float64(accelZ)),
	) * 180 / math.Pi)

	distanceCM := stream.ParseFloat32(v, "distance_cm")
	tiltRad := float64(tiltAngle) * math.Pi / 180
	waterDepthM := float32(float64(distanceCM) * math.Cos(tiltRad) / 100.0)

	lat := stream.ParseFloat64(v, stream.KeyLat)
	lon := stream.ParseFloat64(v, stream.KeyLon)

	reading := models.SensorReading{
		ID:               uuid.New(),
		DeviceID:         stream.ParseString(v, stream.KeyDeviceID),
		Latitude:         lat,
		Longitude:        lon,
		GPSAccuracy:      stream.ParseFloat32(v, "gps_accuracy"),
		DistanceCM:       distanceCM,
		WaterDepthM:      waterDepthM,
		AccelX:           accelX,
		AccelY:           accelY,
		AccelZ:           accelZ,
		GyroX:            stream.ParseFloat32(v, "gyro_x"),
		GyroY:            stream.ParseFloat32(v, "gyro_y"),
		GyroZ:            stream.ParseFloat32(v, "gyro_z"),
		TiltAngle:        tiltAngle,
		EchoAmplitude:    stream.ParseFloat32(v, "echo_amplitude"),
		BatteryVoltage:   stream.ParseFloat32(v, "battery_voltage"),
		SignalStrength:   stream.ParseInt(v, "signal_strength"),
		TransmissionType: stream.ParseString(v, "transmission_type"),
		RecordedAt:       time.Unix(stream.ParseInt64(v, "timestamp"), 0),
	}

	weather := fetchWeatherFromCache(ctx, lat, lon)
	det := eng.Process(&reading, weather)

	reading.FishDetected = det.FishDetected
	reading.DetectionConfidence = float32(det.Confidence)
	reading.FishCount = 0
	reading.FishType = "unknown"
	if det.FishDetected {
		reading.FishCount = 1
	}

	postgres.AttachReadingToActiveSession(db, &reading)
	if reading.SessionID == nil {
		autoSession := &models.FishingSession{
			DeviceID:  reading.DeviceID,
			Name:      fmt.Sprintf("Session-%06X", rand.Intn(0xFFFFFF)),
			StartedAt: reading.RecordedAt,
		}
		if createErr := postgres.CreateSession(db, autoSession); createErr == nil {
			reading.SessionID = &autoSession.ID
		}
	}

	if err := postgres.CreateReading(db, &reading); err != nil {
		return err
	}

	if correlationID := stream.ParseString(v, stream.KeyCameraReadingID); correlationID != "" {
		photoPath := filepath.Join("./data/photos", correlationID+".jpg")
		absPath, _ := filepath.Abs(photoPath)
		cwd, _ := os.Getwd()
		if _, statErr := os.Stat(photoPath); statErr == nil {
			log.Printf("reading photo link [reading %s]: correlationID=%s linking photo_path=%s (abs=%s)",
				reading.ID, correlationID, photoPath, absPath)
			_ = postgres.UpdateReadingPhoto(db, reading.ID, photoPath)

			if p, perr := postgres.GetPhotoByCorrelationID(db, correlationID); perr == nil && p != nil {
				if lerr := postgres.LinkPhotoToReading(db, p.ID, reading.ID, reading.SessionID); lerr != nil {
					log.Printf("reading photo link [reading %s]: failed to enrich photo %s: %v",
						reading.ID, p.ID, lerr)
				} else {
					log.Printf("reading photo link [reading %s]: enriched photo row %s (session=%v)",
						reading.ID, p.ID, reading.SessionID)
				}
			}

			if jpegBytes, readErr := os.ReadFile(photoPath); readErr == nil {
				insertedID := reading.ID
				sessID := reading.SessionID
				go func() {
					result, classErr := ml.ClassifyPhoto(insertedID.String(), jpegBytes)
					if classErr != nil {
						log.Printf("deferred cnn [reading %s]: %v", insertedID, classErr)
						return
					}
					if dbErr := postgres.UpdateReadingCNNResult(
						db, insertedID,
						result.IsFish, result.FishConfidence, result.SceneClass,
						result.Species, result.SpeciesConfidence, result.FishCount,
					); dbErr != nil {
						log.Printf("deferred cnn [reading %s]: update failed: %v", insertedID, dbErr)
					}
					if sessID != nil {
						if total, recErr := postgres.RecomputeSessionTotalFish(db, *sessID); recErr != nil {
							log.Printf("deferred cnn [reading %s]: session total recompute failed: %v", insertedID, recErr)
						} else {
							log.Printf("deferred cnn [reading %s]: session %s total_fish=%d (image fish_count=%d)",
								insertedID, *sessID, total, result.FishCount)
						}
					}
				}()
			} else {
				log.Printf("reading photo link [reading %s]: linked but failed to read %s for CNN: %v",
					reading.ID, photoPath, readErr)
			}
		} else {
			log.Printf("reading photo link [reading %s]: correlationID=%s but no photo file at %s (abs=%s, cwd=%s) — photo not uploaded yet or wrong CWD",
				reading.ID, correlationID, photoPath, absPath, cwd)
		}
	}

	postgres.UpdateDeviceHeartbeat(db, reading.DeviceID)

	var wg sync.WaitGroup
	maybeEmit := func(deviceID, eventType, severity string, payload map[string]any) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			emitEvent(ctx, deviceID, eventType, severity, reading.ID, reading.Latitude, reading.Longitude, payload)
		}()
	}

	if det.Classification == "likely_fish" || det.Classification == "possible" {
		maybeEmit(reading.DeviceID, "fish_detected", "info", map[string]any{
			"fish_count":      reading.FishCount,
			"fish_type":       reading.FishType,
			"depth_m":         waterDepthM,
			"confidence":      det.Confidence,
			"classification":  det.Classification,
			"corrected_depth": det.CorrectedDepth,
			"baseline":        det.Baseline,
		})
	}
	if reading.BatteryVoltage > 0 && reading.BatteryVoltage < 3.3 {
		maybeEmit(reading.DeviceID, "low_battery", "warning", map[string]any{
			"battery_voltage": reading.BatteryVoltage,
		})
	}
	if tiltAngle > 30 {
		maybeEmit(reading.DeviceID, "motion_alert", "warning", map[string]any{
			"tilt_angle": tiltAngle,
		})
	}
	wg.Wait()

	return nil
}

func fetchWeatherFromCache(ctx context.Context, lat, lon float64) []nasa.WeatherInsight {
	key := fmt.Sprintf("weather:%.2f:%.2f", lat, lon)
	b, err := rdb.Client.Get(ctx, key).Bytes()
	if err == nil {
		var insights []nasa.WeatherInsight
		if json.Unmarshal(b, &insights) == nil {
			return insights
		}
	}

	insights, err := nasa.Fetch(lat, lon)
	if err != nil {
		log.Printf("worker: nasa fetch failed for %.2f,%.2f: %v", lat, lon, err)
		return nil
	}
	if b, err := json.Marshal(insights); err == nil {
		rdb.Client.Set(ctx, key, b, time.Hour)
	}
	return insights
}

func emitEvent(ctx context.Context, deviceID, eventType, severity string, readingID uuid.UUID, lat, lon float64, payload map[string]any) {
	vals := map[string]any{
		stream.KeyDeviceID:   deviceID,
		stream.KeyEventType:  eventType,
		stream.KeySeverity:   severity,
		stream.KeyReadingID:  readingID.String(),
		stream.KeyLat:        lat,
		stream.KeyLon:        lon,
		stream.KeyOccurredAt: time.Now().Unix(),
	}
	for k, v := range payload {
		vals[k] = v
	}
	if err := stream.PublishEvent(ctx, vals); err != nil {
		log.Printf("worker: failed to publish event [%s]: %v", eventType, err)
	}
}
