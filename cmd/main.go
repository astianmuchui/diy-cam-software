package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/gofiber/template/django/v3"

	"github.com/astianmuchui/mobilesonar/internal/detection"
	"github.com/astianmuchui/mobilesonar/internal/env"
	"github.com/astianmuchui/mobilesonar/internal/migrations"
	"github.com/astianmuchui/mobilesonar/internal/ml"
	"github.com/astianmuchui/mobilesonar/internal/mqtt"
	"github.com/astianmuchui/mobilesonar/internal/postgres"
	rdb "github.com/astianmuchui/mobilesonar/internal/redis"
	"github.com/astianmuchui/mobilesonar/internal/router"
	"github.com/astianmuchui/mobilesonar/internal/sessions"
	"github.com/astianmuchui/mobilesonar/internal/stream"
	"github.com/astianmuchui/mobilesonar/internal/worker"
)

func main() {
	env.Load()

	postgres.Connect()
	rdb.Connect()
	migrations.Run(postgres.DB)
	if updated, err := postgres.AcknowledgeAllEvents(postgres.DB); err != nil {
		log.Printf("migrations: auto-ack events failed: %v", err)
	} else {
		log.Printf("migrations: auto-ack events updated=%d", updated)
	}
	sessions.Init()
	stream.EnsureGroups(context.Background())
	mqtt.Connect()

	go func() {
		if err := ml.CheckHealth(); err != nil {
			log.Printf("sonarcore: ml service unreachable: %v", err)
		} else {
			log.Println("sonarcore: ml service ok")
		}
	}()

	eng := detection.NewEngine()
	detection.DefaultEngine = eng

	ctx, cancel := context.WithCancel(context.Background())
	go worker.StartReadingWorker(ctx, postgres.DB, eng)
	go worker.StartEventWorker(ctx, postgres.DB)

	registerTemplateFilters()
	engine := django.New("./internal/templates", ".html")

	bodyLimit := env.C.MaxPhotoSizeKB*1024 + 1<<20

	app := fiber.New(fiber.Config{
		AppName:   "SonarCore v1.0",
		Views:     engine,
		BodyLimit: bodyLimit,
	})
	log.Printf("sonarcore: photo cap=%dKB, http body limit=%dB", env.C.MaxPhotoSizeKB, bodyLimit)

	app.Use(logger.New())
	app.Use(recover.New())
	app.Use(cors.New())

	app.Use(func(c *fiber.Ctx) error {
		c.Set("X-Content-Type-Options", "nosniff")
		c.Set("X-Frame-Options", "DENY")
		c.Set("X-XSS-Protection", "1; mode=block")
		c.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		return c.Next()
	})

	app.Static("/assets", "./assets")

	router.Begin(app)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-quit
		log.Println("sonarcore: shutting down...")
		cancel()
		_ = app.Shutdown()
	}()

	log.Printf("sonarcore: listening on :%s", env.C.AppPort)
	if err := app.Listen(":" + env.C.AppPort); err != nil {
		log.Fatalf("sonarcore: server error: %v", err)
	}
}
