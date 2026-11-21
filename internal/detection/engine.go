package detection

import (
	"sync"
	"time"

	"github.com/astianmuchui/mobilesonar/external/nasa"
	"github.com/astianmuchui/mobilesonar/internal/models"
)

const defaultWindowSize = 50

var DefaultEngine *Engine

type recentDetection struct {
	DeviceID  string    `json:"device_id"`
	Timestamp time.Time `json:"timestamp"`
	Detection Detection `json:"detection"`
}

type Engine struct {
	mu         sync.RWMutex
	correctors map[string]*TiltCorrector

	diagMu         sync.Mutex
	recentResults  []recentDetection
	gateRejections []gateRejection
}

type gateRejection struct {
	DeviceID string    `json:"device_id"`
	At       time.Time `json:"at"`
	Reasons  []string  `json:"reasons"`
}

func NewEngine() *Engine {
	return &Engine{
		correctors: make(map[string]*TiltCorrector),
	}
}

func (e *Engine) Process(reading *models.SensorReading, weather []nasa.WeatherInsight) Detection {

	gate := Gate(reading)
	if !gate.Valid {
		d := Detection{
			FishDetected:   false,
			Confidence:     0,
			Classification: "no_detection",
			GateRejected:   true,
			RejectReasons:  gate.RejectReasons,
			Factors:        map[string]float64{},
		}
		e.recordRejection(reading.DeviceID, gate.RejectReasons)
		return d
	}

	corrector := e.getOrCreateCorrector(reading.DeviceID)
	corrDepth, baseline := corrector.Correct(
		float64(reading.WaterDepthM),
		float64(reading.TiltAngle),
	)

	envScore := ScoreEnvironment(weather, reading.RecordedAt)

	det := Score(corrDepth, baseline, reading, envScore, nil, nil)
	det.GateRejected = false

	e.recordResult(reading.DeviceID, reading.RecordedAt, det)
	return det
}

func (e *Engine) CorrectorParams(deviceID string) (slope, intercept, rSquared, baseline float64, windowLen int) {
	e.mu.RLock()
	c, ok := e.correctors[deviceID]
	e.mu.RUnlock()
	if !ok {
		return
	}
	return c.Params()
}

func (e *Engine) AllCorrectorDevices() []string {
	e.mu.RLock()
	defer e.mu.RUnlock()
	ids := make([]string, 0, len(e.correctors))
	for id := range e.correctors {
		ids = append(ids, id)
	}
	return ids
}

func (e *Engine) RecentDetections(n int) []recentDetection {
	e.diagMu.Lock()
	defer e.diagMu.Unlock()
	if n > len(e.recentResults) {
		n = len(e.recentResults)
	}
	out := make([]recentDetection, n)
	copy(out, e.recentResults[len(e.recentResults)-n:])
	return out
}

func (e *Engine) GateRejectionsLastHour() []gateRejection {
	e.diagMu.Lock()
	defer e.diagMu.Unlock()
	cutoff := time.Now().Add(-time.Hour)
	var out []gateRejection
	for _, r := range e.gateRejections {
		if r.At.After(cutoff) {
			out = append(out, r)
		}
	}
	return out
}

func (e *Engine) getOrCreateCorrector(deviceID string) *TiltCorrector {
	e.mu.RLock()
	c, ok := e.correctors[deviceID]
	e.mu.RUnlock()
	if ok {
		return c
	}
	e.mu.Lock()
	defer e.mu.Unlock()

	if c, ok = e.correctors[deviceID]; ok {
		return c
	}
	c = NewTiltCorrector(defaultWindowSize)
	e.correctors[deviceID] = c
	return c
}

func (e *Engine) recordResult(deviceID string, ts time.Time, d Detection) {
	e.diagMu.Lock()
	defer e.diagMu.Unlock()
	e.recentResults = append(e.recentResults, recentDetection{
		DeviceID:  deviceID,
		Timestamp: ts,
		Detection: d,
	})
	if len(e.recentResults) > 100 {
		e.recentResults = e.recentResults[1:]
	}
}

func (e *Engine) recordRejection(deviceID string, reasons []string) {
	e.diagMu.Lock()
	defer e.diagMu.Unlock()
	e.gateRejections = append(e.gateRejections, gateRejection{
		DeviceID: deviceID,
		At:       time.Now(),
		Reasons:  reasons,
	})

	if len(e.gateRejections) > 1000 {
		e.gateRejections = e.gateRejections[1:]
	}
}
