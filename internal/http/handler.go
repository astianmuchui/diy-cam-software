package http

import (
	"github.com/gofiber/fiber/v2"
	"golang.org/x/crypto/bcrypt"

	"github.com/astianmuchui/mobilesonar/internal/env"
	"github.com/astianmuchui/mobilesonar/internal/models"
	"github.com/astianmuchui/mobilesonar/internal/postgres"
	"github.com/astianmuchui/mobilesonar/internal/sessions"
)

func commonData(c *fiber.Ctx) fiber.Map {
	unacked, _ := postgres.GetUnacknowledgedCount(postgres.DB)
	return fiber.Map{
		"user_name":     c.Locals("user_name"),
		"user_role":     c.Locals("user_role"),
		"unacked_count": unacked,
	}
}

func HomeHandler(c *fiber.Ctx) error {
	return c.Redirect("/login")
}

func LoginPageHandler(c *fiber.Ctx) error {
	data := fiber.Map{"Title": "Login — SonarCore"}
	if c.Query("registered") == "1" {
		data["Success"] = "Account created — you can now sign in."
	}
	return c.Render("login", data)
}

func RegisterPageHandler(c *fiber.Ctx) error {
	return c.Render("register", fiber.Map{"Title": "Register — SonarCore"})
}

func RegisterHandler(c *fiber.Ctx) error {
	var req RegisterRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Render("register", fiber.Map{
			"Title": "Register — SonarCore",
			"Error": "Invalid request",
		})
	}

	renderErr := func(msg string) error {
		return c.Render("register", fiber.Map{
			"Title":    "Register — SonarCore",
			"Error":    msg,
			"FullName": req.FullName,
			"Email":    req.Email,
		})
	}

	if req.FullName == "" || req.Email == "" || req.Password == "" {
		return renderErr("All fields are required")
	}
	if len(req.Password) < 8 {
		return renderErr("Password must be at least 8 characters")
	}
	if req.Password != req.ConfirmPassword {
		return renderErr("Passwords do not match")
	}

	if existing, err := postgres.GetUserByEmail(postgres.DB, req.Email); err == nil && existing != nil {
		return renderErr("An account with that email already exists")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return renderErr("Could not process password")
	}

	user := &models.User{
		FullName:     req.FullName,
		Email:        req.Email,
		PasswordHash: string(hash),
		Role:         "admin",
	}
	if err := postgres.CreateUser(postgres.DB, user); err != nil {
		return renderErr("Registration failed — please try again")
	}

	return c.Redirect("/login?registered=1")
}

func LoginHandler(c *fiber.Ctx) error {
	var req LoginRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Render("login", fiber.Map{
			"Title": "Login — SonarCore",
			"Error": "Invalid request",
		})
	}

	user, err := postgres.GetUserByEmail(postgres.DB, req.Email)
	if err != nil {
		return c.Render("login", fiber.Map{
			"Title": "Login — SonarCore",
			"Error": "Invalid credentials",
		})
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		return c.Render("login", fiber.Map{
			"Title": "Login — SonarCore",
			"Error": "Invalid credentials",
		})
	}

	sess, err := sessions.Store.Get(c)
	if err != nil {
		return c.Render("login", fiber.Map{
			"Title": "Login — SonarCore",
			"Error": "Session error, please try again",
		})
	}
	sess.Set("user_id", user.ID.String())
	sess.Set("user_email", user.Email)
	sess.Set("user_role", user.Role)
	sess.Set("user_name", user.FullName)
	if err := sess.Save(); err != nil {
		return c.Render("login", fiber.Map{
			"Title": "Login — SonarCore",
			"Error": "Could not save session",
		})
	}

	return c.Redirect("/admin/dashboard")
}

func LogoutHandler(c *fiber.Ctx) error {
	if sess, err := sessions.Store.Get(c); err == nil {
		_ = sess.Destroy()
	}
	return c.Redirect("/login")
}

func DashboardHandler(c *fiber.Ctx) error {
	stats, _ := postgres.GetOverviewStats(postgres.DB)
	devStats, _ := postgres.GetDeviceStats(postgres.DB)
	recentEvents, _ := postgres.GetRecentEvents(postgres.DB, 5)
	recentReadings, _ := postgres.GetRecentReadings(postgres.DB, 5)
	activeSessions, _ := postgres.GetAllActiveSessions(postgres.DB)

	data := commonData(c)
	data["Title"] = "Dashboard"
	data["active"] = "dashboard"
	data["stats"] = stats
	data["dev_stats"] = devStats
	data["recent_events"] = recentEvents
	data["recent_readings"] = recentReadings
	data["active_session_count"] = len(activeSessions)
	return c.Render("dashboard/index", data)
}

func DevicesHandler(c *fiber.Ctx) error {
	devices, _ := postgres.GetAllDevices(postgres.DB)
	devStats, _ := postgres.GetDeviceStats(postgres.DB)

	data := commonData(c)
	data["Title"] = "Devices"
	data["active"] = "devices"
	data["devices"] = devices
	data["dev_stats"] = devStats
	return c.Render("dashboard/devices", data)
}

func AddDeviceHandler(c *fiber.Ctx) error {
	var req AddDeviceRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request payload"})
	}
	if req.DeviceID == "" || req.Name == "" || req.PlainKey == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "device_id, name, and device_key are required",
		})
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.PlainKey), bcrypt.DefaultCost)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to hash device key"})
	}

	device := &models.Device{
		DeviceID:        req.DeviceID,
		Name:            req.Name,
		Description:     req.Description,
		DeviceKey:       string(hash),
		FirmwareVersion: req.FirmwareVersion,
		Status:          postgres.DeviceStatusOffline,
	}
	if err := postgres.CreateDevice(postgres.DB, device); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"ok": true, "device_id": device.DeviceID})
}

func ReadingsHandler(c *fiber.Ctx) error {
	page := c.QueryInt("page", 1)
	readings, total, _ := postgres.GetAllReadings(postgres.DB, page, 20)

	data := commonData(c)
	data["Title"] = "Readings"
	data["active"] = "readings"
	data["readings"] = readings
	data["total"] = total
	data["page"] = page
	return c.Render("dashboard/readings", data)
}

func EventsHandler(c *fiber.Ctx) error {
	page := c.QueryInt("page", 1)
	events, total, _ := postgres.GetAllEvents(postgres.DB, page, 20)

	data := commonData(c)
	data["Title"] = "Events"
	data["active"] = "events"
	data["events"] = events
	data["total"] = total
	data["page"] = page
	return c.Render("dashboard/events", data)
}

func MapHandler(c *fiber.Ctx) error {
	data := commonData(c)
	data["Title"] = "Map"
	data["active"] = "map"
	data["opencage_key"] = env.C.OpenCageKey
	return c.Render("dashboard/map", data)
}

func WeatherHandler(c *fiber.Ctx) error {
	data := commonData(c)
	data["Title"] = "Weather Insights"
	data["active"] = "weather"
	return c.Render("dashboard/weather", data)
}
