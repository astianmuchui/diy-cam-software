package http

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/astianmuchui/mobilesonar/external/nasa"
	"github.com/astianmuchui/mobilesonar/internal/detection"
	rdb "github.com/astianmuchui/mobilesonar/internal/redis"
)

func GetDetectionDiagnosticsHandler(c *fiber.Ctx) error {
	ctx := c.UserContext()
	eng := detection.DefaultEngine
	if eng == nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "engine not initialised"})
	}

	data, err := rdb.GetOrSet(ctx, "diag:detection", 15*time.Second, func() (any, error) {

		type correctorState struct {
			DeviceID  string  `json:"device_id"`
			Slope     float64 `json:"slope"`
			Intercept float64 `json:"intercept"`
			RSquared  float64 `json:"r_squared"`
			Baseline  float64 `json:"baseline_m"`
			Window    int     `json:"window_size"`
		}
		devices := eng.AllCorrectorDevices()
		correctors := make([]correctorState, 0, len(devices))
		for _, id := range devices {
			slope, intercept, rSq, baseline, wlen := eng.CorrectorParams(id)
			correctors = append(correctors, correctorState{
				DeviceID:  id,
				Slope:     slope,
				Intercept: intercept,
				RSquared:  rSq,
				Baseline:  baseline,
				Window:    wlen,
			})
		}

		recent := eng.RecentDetections(20)

		rejections := eng.GateRejectionsLastHour()
		rejectRate := 0.0
		total := len(recent) + len(rejections)
		if total > 0 {
			rejectRate = float64(len(rejections)) / float64(total)
		}

		var envScore detection.EnvironmentScore
		cacheKey := fmt.Sprintf("weather:%.2f:%.2f", -1.0, 33.0)
		if wb, err := rdb.Client.Get(ctx, cacheKey).Bytes(); err == nil {
			var insights []nasa.WeatherInsight
			if json.Unmarshal(wb, &insights) == nil {
				envScore = detection.ScoreEnvironment(insights, time.Now())
			}
		}

		return fiber.Map{
			"correctors":          correctors,
			"recent_detections":   recent,
			"gate_rejections_1h":  rejections,
			"gate_rejection_rate": rejectRate,
			"environment_score":   envScore,
			"generated_at":        time.Now().UTC(),
		}, nil
	})
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	c.Set("Content-Type", "application/json")
	return c.Send(data)
}
