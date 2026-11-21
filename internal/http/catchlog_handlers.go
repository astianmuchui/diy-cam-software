package http

import (
	"log"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"github.com/astianmuchui/mobilesonar/internal/ml"
	"github.com/astianmuchui/mobilesonar/internal/models"
	"github.com/astianmuchui/mobilesonar/internal/postgres"
)

type createCatchLogRequest struct {
	Latitude  float64   `json:"latitude"`
	Longitude float64   `json:"longitude"`
	FishType  string    `json:"fish_type"`
	Count     int       `json:"count"`
	Notes     string    `json:"notes"`
	CaughtAt  time.Time `json:"caught_at"`
}

func CreateCatchLogHandler(c *fiber.Ctx) error {
	var req createCatchLogRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	if req.Latitude < -5 || req.Latitude > 5 || req.Longitude < 33 || req.Longitude > 42 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "coordinates out of Kenya range"})
	}
	if req.CaughtAt.IsZero() {
		req.CaughtAt = time.Now()
	}

	cl := &models.CatchLog{
		ID:        uuid.New(),
		Latitude:  req.Latitude,
		Longitude: req.Longitude,
		FishType:  req.FishType,
		Count:     req.Count,
		Notes:     req.Notes,
		CaughtAt:  req.CaughtAt,
	}
	if err := postgres.CreateCatchLog(postgres.DB, cl); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	go func() {
		if err := ml.IngestCatchLogs([]models.CatchLog{*cl}); err != nil {
			log.Printf("catch-log ingest: %v", err)
		}
	}()
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"data": cl})
}

func GetCatchLogsHandler(c *fiber.Ctx) error {
	page := c.QueryInt("page", 1)
	pageSize := c.QueryInt("page_size", 20)
	days := c.QueryInt("days", 0)
	if pageSize > 100 {
		pageSize = 100
	}
	logs, total, err := postgres.GetAllCatchLogs(postgres.DB, page, pageSize, days)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{
		"data":      logs,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

func GetNearbyCatchLogsHandler(c *fiber.Ctx) error {
	lat := c.QueryFloat("lat", 0)
	lon := c.QueryFloat("lon", 0)
	radius := c.QueryFloat("radius", 1000)
	days := c.QueryInt("days", 30)

	if lat == 0 && lon == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "lat and lon are required"})
	}
	logs, err := postgres.GetNearbyCatchLogs(postgres.DB, lat, lon, radius, days)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"data": logs})
}

func DeleteCatchLogHandler(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid id"})
	}
	if err := postgres.DeleteCatchLog(postgres.DB, id); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"ok": true})
}
