package middleware

import (
	"github.com/gofiber/fiber/v2"
	"golang.org/x/crypto/bcrypt"

	"github.com/astianmuchui/mobilesonar/internal/models"
	"github.com/astianmuchui/mobilesonar/internal/postgres"
)

func DeviceAuth(c *fiber.Ctx) error {
	deviceID := c.Get("X-Device-ID")
	deviceKey := c.Get("X-Device-Key")

	if deviceID == "" || deviceKey == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "missing device credentials",
		})
	}

	var device models.Device
	if err := postgres.DB.Where("device_id = ?", deviceID).First(&device).Error; err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "device not found",
		})
	}

	if err := bcrypt.CompareHashAndPassword([]byte(device.DeviceKey), []byte(deviceKey)); err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "invalid device key",
		})
	}

	c.Locals("device_id", deviceID)
	return c.Next()
}
