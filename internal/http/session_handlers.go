package http

import (
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"github.com/astianmuchui/mobilesonar/internal/ml"
	"github.com/astianmuchui/mobilesonar/internal/models"
	"github.com/astianmuchui/mobilesonar/internal/postgres"
)

func StartSessionHandler(c *fiber.Ctx) error {
	var req struct {
		DeviceID  string    `json:"device_id"`
		Name      string    `json:"name"`
		Notes     string    `json:"notes"`
		StartedAt time.Time `json:"started_at"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	if req.DeviceID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "device_id is required"})
	}
	if req.Name == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "name is required"})
	}
	if req.StartedAt.IsZero() {
		req.StartedAt = time.Now()
	}

	s := &models.FishingSession{
		DeviceID:  req.DeviceID,
		Name:      req.Name,
		Notes:     req.Notes,
		StartedAt: req.StartedAt,
	}
	if err := postgres.CreateSession(postgres.DB, s); err != nil {
		if err == postgres.ErrActiveSessionExists {
			return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": err.Error()})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"data": s})
}

func ListSessionsHandler(c *fiber.Ctx) error {
	page := c.QueryInt("page", 1)
	pageSize := c.QueryInt("page_size", 20)
	if pageSize > 100 {
		pageSize = 100
	}
	sessions, total, err := postgres.GetAllSessions(postgres.DB, page, pageSize)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{
		"data":      sessions,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

func ListActiveSessionsHandler(c *fiber.Ctx) error {
	sessions, err := postgres.GetAllActiveSessions(postgres.DB)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"data": sessions})
}

func SessionMapCoverageHandler(c *fiber.Ctx) error {
	rows, err := postgres.GetSessionCoveragePoints(postgres.DB)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	type sessionOut struct {
		ID           string       `json:"id"`
		Name         string       `json:"name"`
		DeviceID     string       `json:"device_id"`
		Status       string       `json:"status"`
		ReadingCount int64        `json:"reading_count"`
		Points       [][2]float64 `json:"points"`
	}

	type acc struct {
		out sessionOut
	}
	order := []string{}
	byID := map[string]*acc{}

	for _, row := range rows {
		a, ok := byID[row.SessionID]
		if !ok {
			a = &acc{out: sessionOut{
				ID: row.SessionID, Name: row.Name,
				DeviceID: row.DeviceID, Status: row.Status,
				ReadingCount: row.ReadingCount,
			}}
			byID[row.SessionID] = a
			order = append(order, row.SessionID)
		}
		a.out.Points = append(a.out.Points, [2]float64{row.Latitude, row.Longitude})
	}

	result := make([]sessionOut, 0, len(order))
	for _, id := range order {
		result = append(result, byID[id].out)
	}
	return c.JSON(fiber.Map{"data": result})
}

func GetSessionHandler(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid session id"})
	}
	s, err := postgres.GetSessionByID(postgres.DB, id)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	if s == nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "session not found"})
	}
	catches, _ := postgres.GetSessionCatchLogs(postgres.DB, id)
	return c.JSON(fiber.Map{"data": s, "catches": catches})
}

func GetSessionReadingsHandler(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid session id"})
	}
	readings, err := postgres.GetSessionReadings(postgres.DB, id)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"data": readings, "count": len(readings)})
}

type analysisPoint struct {
	RecordedAt          time.Time `json:"RecordedAt"`
	Latitude            float64   `json:"Latitude"`
	Longitude           float64   `json:"Longitude"`
	DistanceCM          float32   `json:"DistanceCM"`
	WaterDepthM         float64   `json:"WaterDepthM"`
	TiltAngle           float32   `json:"TiltAngle"`
	AccelX              float32   `json:"AccelX"`
	AccelY              float32   `json:"AccelY"`
	AccelZ              float32   `json:"AccelZ"`
	DetectionConfidence float64   `json:"DetectionConfidence"`
	FishDetected        bool      `json:"FishDetected"`
	FishCount           int       `json:"FishCount"`
	FishType            string    `json:"FishType"`
	GateValid           bool      `json:"GateValid"`
}

func GetSessionAnalysisHandler(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid session id"})
	}
	readings, err := postgres.GetSessionReadings(postgres.DB, id)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	out := make([]analysisPoint, 0, len(readings))
	if len(readings) == 0 {
		return c.JSON(fiber.Map{"data": out, "count": 0, "source": "ml"})
	}

	catchLogs, _ := postgres.GetSessionCatchLogs(postgres.DB, id)
	source := "ml"
	results, mlErr := ml.SessionAnalyze(id.String(), readings, catchLogs, nil)
	if mlErr != nil || len(results) != len(readings) {
		if mlErr != nil {
			log.Printf("session analysis [%s]: ml unavailable, using stored values: %v", id, mlErr)
		}
		results = nil
		source = "stored"
	}

	for i, r := range readings {
		p := analysisPoint{
			RecordedAt: r.RecordedAt,
			Latitude:   r.Latitude,
			Longitude:  r.Longitude,
			DistanceCM: r.DistanceCM,
			TiltAngle:  r.TiltAngle,
			AccelX:     r.AccelX,
			AccelY:     r.AccelY,
			AccelZ:     r.AccelZ,
		}
		if results != nil {
			det := results[i]
			p.WaterDepthM = det.CorrectedDepth
			if p.WaterDepthM == 0 {
				p.WaterDepthM = float64(r.WaterDepthM)
			}
			p.DetectionConfidence = det.Confidence
			p.FishDetected = det.FishDetected
			p.FishCount = det.FishCount
			if p.FishCount == 0 && det.FishDetected {
				p.FishCount = 1
			}
			p.FishType = det.FishType
			p.GateValid = det.GateValid
		} else {
			p.WaterDepthM = float64(r.WaterDepthM)
			p.DetectionConfidence = float64(r.DetectionConfidence)
			p.FishDetected = r.FishDetected
			p.FishCount = r.FishCount
			p.FishType = r.FishType
			p.GateValid = r.TiltAngle < 30 && r.WaterDepthM > 0.1
		}
		out = append(out, p)
	}

	return c.JSON(fiber.Map{"data": out, "count": len(out), "source": source})
}

type catchEntry struct {
	FishType string `json:"fish_type"`
	Count    int    `json:"count"`
	Notes    string `json:"notes"`
}

func EndSessionHandler(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid session id"})
	}

	var req struct {
		Catches []catchEntry `json:"catches"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}

	s, err := postgres.GetSessionByID(postgres.DB, id)
	if err != nil || s == nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "session not found"})
	}
	if s.Status != "active" {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": "session is already ended"})
	}

	if err := postgres.EndSession(postgres.DB, id); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	readings, _ := postgres.GetSessionReadings(postgres.DB, id)
	var lastLat, lastLon float64
	if len(readings) > 0 {
		last := readings[len(readings)-1]
		lastLat, lastLon = last.Latitude, last.Longitude
	}

	totalFish := 0
	caughtAt := time.Now()
	for _, entry := range req.Catches {
		if entry.Count <= 0 {
			continue
		}
		cl := &models.CatchLog{
			ID:        uuid.New(),
			SessionID: &id,
			DeviceID:  s.DeviceID,
			Latitude:  lastLat,
			Longitude: lastLon,
			FishType:  entry.FishType,
			Count:     entry.Count,
			Notes:     entry.Notes,
			CaughtAt:  caughtAt,
		}
		if err := postgres.CreateCatchLog(postgres.DB, cl); err != nil {
			log.Printf("session end: catch log create failed: %v", err)
		}
		totalFish += entry.Count
	}

	if err := postgres.UpdateSessionTotalFish(postgres.DB, id, totalFish); err != nil {
		log.Printf("session end: update total_fish failed: %v", err)
	}

	go runMLBatchForSession(id, s.DeviceID, readings)

	s, _ = postgres.GetSessionByID(postgres.DB, id)
	return c.JSON(fiber.Map{"data": s, "total_fish": totalFish})
}

func runMLBatchForSession(sessionID uuid.UUID, deviceID string, readings []models.SensorReading) {
	if len(readings) == 0 {
		return
	}

	catchLogs, _ := postgres.GetSessionCatchLogs(postgres.DB, sessionID)

	results, err := ml.SessionAnalyze(sessionID.String(), readings, catchLogs, nil)
	if err != nil {
		log.Printf("ml batch [session %s]: %v", sessionID, err)
		return
	}
	if len(results) != len(readings) {
		log.Printf("ml batch [session %s]: result count mismatch %d vs %d", sessionID, len(results), len(readings))
		return
	}

	for i, det := range results {
		fishCount := det.FishCount
		if fishCount == 0 && det.FishDetected {
			fishCount = 1
		}
		if err := postgres.BulkUpdateReadingDetection(
			postgres.DB, readings[i].ID,
			det.FishDetected, float32(det.Confidence), fishCount, det.FishType,
		); err != nil {
			log.Printf("ml batch [session %s] update reading %s: %v", sessionID, readings[i].ID, err)
		}
	}
	log.Printf("ml batch [session %s]: corrected %d readings", sessionID, len(readings))
}

func MigrateDetectionsHandler(c *fiber.Ctx) error {
	devices, err := postgres.GetAllDevices(postgres.DB)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	workers := c.QueryInt("workers", 6)
	if workers < 1 {
		workers = 1
	}
	if workers > 32 {
		workers = 32
	}

	go func() {
		var correctedCount int64
		var markedReadCount int64

		jobs := make(chan string)
		var wg sync.WaitGroup

		processDevice := func(deviceID string) {
			readings, err := postgres.GetAllReadingsForDevice(postgres.DB, deviceID)
			if err != nil {
				log.Printf("migrate-detections [%s]: load readings: %v", deviceID, err)
				return
			}
			if len(readings) == 0 {
				return
			}

			results, err := ml.BatchDetect(readings, nil)
			if err != nil {
				log.Printf("migrate-detections [%s]: %v", deviceID, err)
				return
			}
			if len(results) != len(readings) {
				log.Printf("migrate-detections [%s]: result count mismatch", deviceID)
				return
			}

			for i, det := range results {
				fishCount := det.FishCount
				if fishCount == 0 && det.FishDetected {
					fishCount = 1
				}
				if err := postgres.BulkUpdateReadingDetection(
					postgres.DB, readings[i].ID,
					det.FishDetected, float32(det.Confidence), fishCount, det.FishType,
				); err != nil {
					log.Printf("migrate-detections [%s]: update %s: %v", deviceID, readings[i].ID, err)
				}
			}

			if err := postgres.MarkAllReadingsAsReadForDevice(postgres.DB, deviceID); err != nil {
				log.Printf("migrate-detections [%s]: mark read failed: %v", deviceID, err)
				return
			}

			atomic.AddInt64(&correctedCount, int64(len(readings)))
			atomic.AddInt64(&markedReadCount, int64(len(readings)))
			log.Printf("migrate-detections [%s]: corrected+marked %d readings", deviceID, len(readings))
		}

		for i := 0; i < workers; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for deviceID := range jobs {
					processDevice(deviceID)
				}
			}()
		}

		for _, dev := range devices {
			jobs <- dev.DeviceID
		}
		close(jobs)
		wg.Wait()

		log.Printf("migrate-detections: done devices=%d corrected=%d marked_read=%d workers=%d", len(devices), correctedCount, markedReadCount, workers)
	}()

	return c.JSON(fiber.Map{
		"ok":      true,
		"message": fmt.Sprintf("migration started for %d devices with %d workers (runs in background)", len(devices), workers),
	})
}

func SessionsPageHandler(c *fiber.Ctx) error {
	page := c.QueryInt("page", 1)
	sessions, total, _ := postgres.GetAllSessions(postgres.DB, page, 20)
	activeSessions, _ := postgres.GetAllActiveSessions(postgres.DB)
	devices, _ := postgres.GetAllDevices(postgres.DB)

	data := commonData(c)
	data["Title"] = "Fishing Sessions"
	data["active"] = "sessions"
	data["sessions"] = sessions
	data["total"] = total
	data["page"] = page
	data["active_sessions"] = activeSessions
	data["devices"] = devices
	return c.Render("dashboard/sessions", data)
}

type photoView struct {
	ID                uuid.UUID
	RecordedAt        time.Time
	HasCNN            bool
	IsFish            bool
	FishLabel         string
	Confidence        string
	Species           string
	SpeciesConfidence string
	Scene             string
}

func clampConf(v float64) float64 {
	if v > 1 {
		v = v / 100
	}
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

func SessionDetailPageHandler(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Redirect("/admin/sessions")
	}

	s, err := postgres.GetSessionByID(postgres.DB, id)
	if err != nil || s == nil {
		return c.Redirect("/admin/sessions")
	}

	catches, _ := postgres.GetSessionCatchLogs(postgres.DB, id)
	sessionPhotos, _ := postgres.GetPhotosBySession(postgres.DB, id)

	var durationStr string
	if s.EndedAt != nil {
		dur := s.EndedAt.Sub(s.StartedAt)
		h := int(dur.Hours())
		m := int(dur.Minutes()) % 60
		durationStr = fmt.Sprintf("%dh %dm", h, m)
	} else {
		dur := time.Since(s.StartedAt)
		h := int(dur.Hours())
		m := int(dur.Minutes()) % 60
		durationStr = fmt.Sprintf("%dh %dm (ongoing)", h, m)
	}

	var photosReadings []photoView
	for _, p := range sessionPhotos {
		pv := photoView{ID: p.ID, RecordedAt: p.CapturedAt}

		if p.CNNFishDetected != nil {
			pv.HasCNN = true
			pv.IsFish = *p.CNNFishDetected
			if pv.IsFish {
				pv.FishLabel = "Fish detected"
			} else {
				pv.FishLabel = "No fish"
			}
		} else {
			pv.FishLabel = "Pending"
		}
		if p.CNNConfidence != nil {
			pv.HasCNN = true
			pv.Confidence = fmt.Sprintf("%.0f%%", clampConf(*p.CNNConfidence)*100)
		}
		if p.CNNSpecies != "" {
			pv.HasCNN = true
			pv.Species = p.CNNSpecies
		}
		if p.CNNSpeciesConf != nil && p.CNNSpecies != "" {
			pv.SpeciesConfidence = fmt.Sprintf("%.0f%%", clampConf(*p.CNNSpeciesConf)*100)
		}
		if p.CNNSceneClass != "" {
			pv.HasCNN = true
			pv.Scene = p.CNNSceneClass
		}
		photosReadings = append(photosReadings, pv)
	}

	data := commonData(c)
	data["Title"] = s.Name
	data["active"] = "sessions"
	data["session"] = s
	data["catches"] = catches
	data["duration"] = durationStr
	data["photosReadings"] = photosReadings
	return c.Render("dashboard/session_detail", data)
}
