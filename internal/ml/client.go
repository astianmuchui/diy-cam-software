package ml

import (
	"bytes"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"time"

	"github.com/astianmuchui/mobilesonar/internal/env"
	"github.com/astianmuchui/mobilesonar/internal/models"
)

var httpClient = &http.Client{Timeout: 30 * time.Second}

type MLReading struct {
	ID          string  `json:"id"`
	DeviceID    string  `json:"device_id"`
	WaterDepthM float32 `json:"water_depth_m"`
	DistanceCM  float32 `json:"distance_cm"`
	TiltAngle   float32 `json:"tilt_angle"`
	AccelX      float32 `json:"accel_x"`
	AccelY      float32 `json:"accel_y"`
	AccelZ      float32 `json:"accel_z"`
	GyroX       float32 `json:"gyro_x"`
	GyroY       float32 `json:"gyro_y"`
	GyroZ       float32 `json:"gyro_z"`
	RecordedAt  string  `json:"recorded_at"`
}

type MLWeather struct {
	Temperature    float64 `json:"temperature"`
	SolarRadiation float64 `json:"solar_radiation"`
	WindSpeed      float64 `json:"wind_speed"`
	Precipitation  float64 `json:"precipitation"`
}

type MLCatchLog struct {
	ID        string  `json:"id"`
	FishType  string  `json:"fish_type"`
	Count     int     `json:"count"`
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	CaughtAt  string  `json:"caught_at"`
}

type DetectionResult struct {
	FishDetected      bool               `json:"fish_detected"`
	Confidence        float64            `json:"confidence"`
	Classification    string             `json:"classification"`
	CorrectedDepth    float64            `json:"corrected_depth"`
	FishCount         int                `json:"fish_count"`
	FishType          string             `json:"fish_type"`
	SpeciesConfidence float64            `json:"species_confidence"`
	SpeciesVotes      map[string]float64 `json:"species_votes"`
	GateValid         bool               `json:"gate_valid"`
}

type batchRequest struct {
	Readings []MLReading `json:"readings"`
	Weather  *MLWeather  `json:"weather,omitempty"`
}

type batchResponse struct {
	Count int               `json:"count"`
	Data  []DetectionResult `json:"data"`
}

func BatchDetect(readings []models.SensorReading, weather *MLWeather) ([]DetectionResult, error) {
	payload := batchRequest{Weather: weather}
	for _, r := range readings {
		payload.Readings = append(payload.Readings, toMLReading(r))
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("%s/api/v1/detect/batch", env.C.MLAPIUrl)
	resp, err := httpClient.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("ml batch request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ml api returned %d", resp.StatusCode)
	}

	var result batchResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("ml batch decode failed: %w", err)
	}
	return result.Data, nil
}

type sessionAnalyzeRequest struct {
	Readings  []MLReading  `json:"readings"`
	CatchLogs []MLCatchLog `json:"catch_logs,omitempty"`
	Weather   *MLWeather   `json:"weather,omitempty"`
}

type sessionAnalyzeResponse struct {
	SessionID  string            `json:"session_id"`
	Count      int               `json:"count"`
	Detections []DetectionResult `json:"detections"`
}

func SessionAnalyze(sessionID string, readings []models.SensorReading, catchLogs []models.CatchLog, weather *MLWeather) ([]DetectionResult, error) {
	payload := sessionAnalyzeRequest{Weather: weather}
	for _, r := range readings {
		payload.Readings = append(payload.Readings, toMLReading(r))
	}
	for _, cl := range catchLogs {
		if cl.FishType == "" || cl.FishType == "unknown" {
			continue
		}
		payload.CatchLogs = append(payload.CatchLogs, MLCatchLog{
			ID:        cl.ID.String(),
			FishType:  cl.FishType,
			Count:     cl.Count,
			Latitude:  cl.Latitude,
			Longitude: cl.Longitude,
			CaughtAt:  cl.CaughtAt.UTC().Format(time.RFC3339),
		})
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("%s/api/v1/sessions/%s/analyze", env.C.MLAPIUrl, sessionID)
	resp, err := httpClient.Post(url, "application/json", bytes.NewReader(body))
	if err != nil || resp.StatusCode != http.StatusOK {
		if resp != nil {
			resp.Body.Close()
		}
		return BatchDetect(readings, weather)
	}
	defer resp.Body.Close()

	var result sessionAnalyzeResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return BatchDetect(readings, weather)
	}
	return result.Detections, nil
}

func IngestCatchLogs(catchLogs []models.CatchLog) error {
	if len(catchLogs) == 0 {
		return nil
	}
	var payload []MLCatchLog
	for _, cl := range catchLogs {
		payload = append(payload, MLCatchLog{
			ID:        cl.ID.String(),
			FishType:  cl.FishType,
			Count:     cl.Count,
			Latitude:  cl.Latitude,
			Longitude: cl.Longitude,
			CaughtAt:  cl.CaughtAt.UTC().Format(time.RFC3339),
		})
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	url := fmt.Sprintf("%s/api/v1/catch-logs/ingest", env.C.MLAPIUrl)
	resp, err := httpClient.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("ml ingest request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("ml ingest returned %d", resp.StatusCode)
	}
	return nil
}

type CNNClassifyResult struct {
	IsFish            bool           `json:"is_fish"`
	FishConfidence    float64        `json:"fish_confidence"`
	SceneClass        string         `json:"scene_class"`
	Species           string         `json:"species"`
	SpeciesConfidence float64        `json:"species_confidence"`
	FishCount         int            `json:"fish_count"`
	SpeciesBreakdown  map[string]int `json:"species_breakdown"`
}

type classifyEnvelope struct {
	Data CNNClassifyResult `json:"data"`
}

func ClassifyPhoto(readingID string, jpeg []byte) (*CNNClassifyResult, error) {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)

	_ = w.WriteField("reading_id", readingID)

	fw, err := w.CreateFormFile("photo", "photo.jpg")
	if err != nil {
		return nil, err
	}
	if _, err := fw.Write(jpeg); err != nil {
		return nil, err
	}
	w.Close()

	url := fmt.Sprintf("%s/api/v1/classify", env.C.MLAPIUrl)
	resp, err := httpClient.Post(url, w.FormDataContentType(), &buf)
	if err != nil {
		return nil, fmt.Errorf("ml classify: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ml classify returned %d", resp.StatusCode)
	}

	var env classifyEnvelope
	if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
		return nil, fmt.Errorf("ml classify decode: %w", err)
	}
	return &env.Data, nil
}

type InsightsResult struct {
	GeneratedAt        string          `json:"generated_at"`
	Location           json.RawMessage `json:"location"`
	CurrentEnvironment json.RawMessage `json:"current_environment"`
	BestTimeWindows    json.RawMessage `json:"best_time_windows"`
	BestLocations      json.RawMessage `json:"best_locations"`
	ExpectedSpecies    json.RawMessage `json:"expected_species"`
	CatchTrend         json.RawMessage `json:"catch_trend"`
	Advisory           string          `json:"advisory"`
	SampleSize         int             `json:"sample_size"`
}

type insightsEnvelope struct {
	Data  InsightsResult `json:"data"`
	Error string         `json:"error"`
}

type insightsRequest struct {
	Latitude     float64    `json:"latitude"`
	Longitude    float64    `json:"longitude"`
	Timestamp    string     `json:"timestamp,omitempty"`
	RadiusM      float64    `json:"radius_m,omitempty"`
	HorizonHours int        `json:"horizon_hours,omitempty"`
	Weather      *MLWeather `json:"weather,omitempty"`
}

func GetInsights(lat, lon, radiusM float64, weather *MLWeather) (*InsightsResult, error) {
	body, err := json.Marshal(insightsRequest{
		Latitude:  lat,
		Longitude: lon,
		RadiusM:   radiusM,
		Weather:   weather,
	})
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("%s/api/v1/insights", env.C.MLAPIUrl)
	resp, err := httpClient.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("ml insights request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ml insights returned %d", resp.StatusCode)
	}

	var envl insightsEnvelope
	if err := json.NewDecoder(resp.Body).Decode(&envl); err != nil {
		return nil, fmt.Errorf("ml insights decode: %w", err)
	}
	if envl.Error != "" {
		return nil, fmt.Errorf("ml insights error: %s", envl.Error)
	}
	return &envl.Data, nil
}

func CheckHealth() error {
	url := fmt.Sprintf("%s/health", env.C.MLAPIUrl)
	resp, err := httpClient.Get(url)
	if err != nil {
		return fmt.Errorf("ml health: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("ml health returned %d", resp.StatusCode)
	}
	return nil
}

func toMLReading(r models.SensorReading) MLReading {
	return MLReading{
		ID:          r.ID.String(),
		DeviceID:    r.DeviceID,
		WaterDepthM: r.WaterDepthM,
		DistanceCM:  r.DistanceCM,
		TiltAngle:   r.TiltAngle,
		AccelX:      r.AccelX,
		AccelY:      r.AccelY,
		AccelZ:      r.AccelZ,
		GyroX:       r.GyroX,
		GyroY:       r.GyroY,
		GyroZ:       r.GyroZ,
		RecordedAt:  r.RecordedAt.UTC().Format(time.RFC3339),
	}
}
