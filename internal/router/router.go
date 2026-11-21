package router

import (
	"github.com/gofiber/fiber/v2"

	apphttp "github.com/astianmuchui/mobilesonar/internal/http"
	"github.com/astianmuchui/mobilesonar/internal/middleware"
)

func Begin(app *fiber.App) {
	app.Get("/", apphttp.HomeHandler)

	app.Get("/login", apphttp.LoginPageHandler)
	app.Get("/register", apphttp.RegisterPageHandler)
	app.Post("/auth/login", apphttp.LoginHandler)
	app.Post("/auth/register", apphttp.RegisterHandler)
	app.Get("/auth/logout", apphttp.LogoutHandler)

	app.Route("/admin", func(r fiber.Router) {
		r.Use(middleware.SessionAuth)

		r.Get("/dashboard", apphttp.DashboardHandler)
		r.Get("/devices", apphttp.DevicesHandler)
		r.Post("/devices/add", apphttp.AddDeviceHandler)
		r.Get("/readings", apphttp.ReadingsHandler)
		r.Get("/events", apphttp.EventsHandler)
		r.Get("/map", apphttp.MapHandler)
		r.Get("/weather", apphttp.WeatherHandler)
		r.Get("/insights", apphttp.InsightsPageHandler)
		r.Get("/sessions", apphttp.SessionsPageHandler)
		r.Get("/sessions/:id", apphttp.SessionDetailPageHandler)
		r.Get("/photos/:reading_id", apphttp.AdminPhotoHandler)
	})

	app.Route("/api/v1", func(api fiber.Router) {
		api.Get("/readings", apphttp.GetReadingsHandler)
		api.Get("/readings/recent", apphttp.GetRecentReadingsHandler)
		api.Get("/devices", apphttp.GetDevicesHandler)
		api.Get("/events", apphttp.GetEventsHandler)
		api.Post("/events/:id/acknowledge", apphttp.AcknowledgeEventHandler)
		api.Get("/analytics/overview", apphttp.GetOverviewHandler)
		api.Get("/analytics/fish-probability", apphttp.GetFishProbabilityHandler)
		api.Get("/analytics/density-map", apphttp.GetDensityMapHandler)
		api.Get("/analytics/trends", apphttp.GetTrendsHandler)
		api.Get("/analytics/daily-stats", apphttp.GetDailyStatsHandler)
		api.Get("/weather/insights", apphttp.GetWeatherInsightsHandler)
		api.Get("/insights", apphttp.GetInsightsHandler)

		api.Post("/photos/upload", apphttp.PhotoUploadHandler)
		api.Get("/photos/:reading_id", apphttp.PhotoRetrieveHandler)

		api.All("/cnn/*", apphttp.CNNProxyHandler)

		api.Post("/catch-logs", apphttp.CreateCatchLogHandler)
		api.Get("/catch-logs", apphttp.GetCatchLogsHandler)
		api.Get("/catch-logs/nearby", apphttp.GetNearbyCatchLogsHandler)
		api.Delete("/catch-logs/:id", apphttp.DeleteCatchLogHandler)

		api.Get("/analytics/detection-diagnostics", apphttp.GetDetectionDiagnosticsHandler)

		api.Post("/sessions", apphttp.StartSessionHandler)
		api.Get("/sessions", apphttp.ListSessionsHandler)
		api.Get("/sessions/active", apphttp.ListActiveSessionsHandler)
		api.Get("/sessions/map-coverage", apphttp.SessionMapCoverageHandler)
		api.Get("/sessions/:id", apphttp.GetSessionHandler)
		api.Get("/sessions/:id/readings", apphttp.GetSessionReadingsHandler)
		api.Get("/sessions/:id/analysis", apphttp.GetSessionAnalysisHandler)
		api.Post("/sessions/:id/end", apphttp.EndSessionHandler)

		api.Post("/admin/migrate-detections", apphttp.MigrateDetectionsHandler)
		api.Post("/admin/migrate-events-ack", apphttp.MigrateAcknowledgeEventsHandler)
	})
}
