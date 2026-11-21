package http

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"github.com/astianmuchui/mobilesonar/external/nasa"
	"github.com/astianmuchui/mobilesonar/internal/detection"
	"github.com/astianmuchui/mobilesonar/internal/postgres"
	rdb "github.com/astianmuchui/mobilesonar/internal/redis"
)

func GetReadingsHandler(c *fiber.Ctx) error {
	page := c.QueryInt("page", 1)
	pageSize := c.QueryInt("page_size", 20)
	if pageSize > 100 {
		pageSize = 100
	}
	readings, total, err := postgres.GetAllReadings(postgres.DB, page, pageSize)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{
		"data":      readings,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

func GetRecentReadingsHandler(c *fiber.Ctx) error {
	limit := c.QueryInt("limit", 10)
	readings, err := postgres.GetRecentReadings(postgres.DB, limit)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(readings)
}

func GetDevicesHandler(c *fiber.Ctx) error {
	devices, err := postgres.GetAllDevices(postgres.DB)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(devices)
}

func GetEventsHandler(c *fiber.Ctx) error {
	page := c.QueryInt("page", 1)
	pageSize := c.QueryInt("page_size", 20)
	if pageSize > 100 {
		pageSize = 100
	}
	events, total, err := postgres.GetAllEvents(postgres.DB, page, pageSize)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{
		"data":      events,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

func AcknowledgeEventHandler(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid event id"})
	}
	if err := postgres.AcknowledgeEvent(postgres.DB, id); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"ok": true})
}

func MigrateAcknowledgeEventsHandler(c *fiber.Ctx) error {
	updated, err := postgres.AcknowledgeAllEvents(postgres.DB)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{
		"ok":                  true,
		"acknowledged_events": updated,
	})
}

func GetOverviewHandler(c *fiber.Ctx) error {
	ctx := c.UserContext()
	data, err := rdb.GetOrSet(ctx, "analytics:overview", 30*time.Second, func() (any, error) {
		stats, err := postgres.GetOverviewStats(postgres.DB)
		if err != nil {
			return nil, err
		}
		devStats, err := postgres.GetDeviceStats(postgres.DB)
		if err != nil {
			return nil, err
		}
		unacked, err := postgres.GetUnacknowledgedCount(postgres.DB)
		if err != nil {
			return nil, err
		}
		return fiber.Map{
			"readings":       stats,
			"devices":        devStats,
			"unacked_events": unacked,
		}, nil
	})
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	c.Set("Content-Type", "application/json")
	return c.Send(data)
}

func GetFishProbabilityHandler(c *fiber.Ctx) error {
	var req FishProbabilityRequest
	if err := c.QueryParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid query params"})
	}
	if req.Radius == 0 {
		req.Radius = 1000
	}
	if req.Days == 0 {
		req.Days = 30
	}

	ctx := c.UserContext()
	cacheKey := fmt.Sprintf("fish_prob:%.4f:%.4f:%.0f:%d", req.Lat, req.Lon, req.Radius, req.Days)
	data, err := rdb.GetOrSet(ctx, cacheKey, 5*time.Minute, func() (any, error) {
		return postgres.GetFishProbabilityNear(postgres.DB, req.Lat, req.Lon, req.Radius, req.Days)
	})
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	c.Set("Content-Type", "application/json")
	return c.Send(data)
}

func GetDensityMapHandler(c *fiber.Ctx) error {
	var req DensityMapRequest
	if err := c.QueryParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid query params"})
	}
	if req.Days == 0 {
		req.Days = 7
	}

	ctx := c.UserContext()
	cacheKey := fmt.Sprintf("density:%d", req.Days)
	data, err := rdb.GetOrSet(ctx, cacheKey, 2*time.Minute, func() (any, error) {
		return postgres.GetDensityMap(postgres.DB, req.Days)
	})
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	c.Set("Content-Type", "application/json")
	return c.Send(data)
}

func GetTrendsHandler(c *fiber.Ctx) error {
	var req TrendsRequest
	if err := c.QueryParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid query params"})
	}
	if req.Days == 0 {
		req.Days = 30
	}

	ctx := c.UserContext()
	cacheKey := fmt.Sprintf("trends:%d", req.Days)
	data, err := rdb.GetOrSet(ctx, cacheKey, 5*time.Minute, func() (any, error) {
		return postgres.GetHourlyTrends(postgres.DB, req.Days)
	})
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	c.Set("Content-Type", "application/json")
	return c.Send(data)
}

func GetDailyStatsHandler(c *fiber.Ctx) error {
	days := c.QueryInt("days", 14)
	ctx := c.UserContext()
	cacheKey := fmt.Sprintf("daily_stats:%d", days)
	data, err := rdb.GetOrSet(ctx, cacheKey, 2*time.Minute, func() (any, error) {
		return postgres.GetDailyDetectionStats(postgres.DB, days)
	})
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	c.Set("Content-Type", "application/json")
	return c.Send(data)
}

func GetWeatherInsightsHandler(c *fiber.Ctx) error {
	var req WeatherRequest
	if err := c.QueryParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid query params"})
	}

	if req.Lat == 0 && req.Lon == 0 {
		req.Lat = -1.0
		req.Lon = 33.0
	}

	ctx := c.UserContext()
	cacheKey := fmt.Sprintf("weather:%.2f:%.2f", req.Lat, req.Lon)
	data, err := rdb.GetOrSet(ctx, cacheKey, time.Hour, func() (any, error) {
		return nasa.Fetch(req.Lat, req.Lon)
	})
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	var days []nasa.WeatherInsight
	if jsonErr := json.Unmarshal(data, &days); jsonErr == nil {
		loc, locErr := time.LoadLocation("Africa/Nairobi")
		if locErr != nil {
			loc = time.UTC
		}
		for i := range days {
			ts := time.Date(2000, 1, 1, 6, 0, 0, 0, loc)
			if d, pErr := time.ParseInLocation("20060102", days[i].Date, loc); pErr == nil {
				ts = time.Date(d.Year(), d.Month(), d.Day(), 6, 0, 0, 0, loc)
			}
			env := detection.ScoreEnvironment([]nasa.WeatherInsight{days[i]}, ts)
			days[i].FishingScore = int(env.Overall*10 + 0.5)
			days[i].Recommendation = weatherAdvice(env.Recommendation)
		}
		if rescored, mErr := json.Marshal(days); mErr == nil {
			data = rescored
		}
	}

	c.Set("Content-Type", "application/json")
	return c.Send(data)
}

func weatherAdvice(rec string) string {
	switch rec {
	case "prime":
		return "Excellent conditions — fish all day"
	case "good":
		return "Good conditions — early morning recommended"
	case "moderate":
		return "Fair conditions — try dawn or dusk"
	default:
		return "Poor conditions — consider waiting"
	}
}
