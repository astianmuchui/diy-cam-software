package http

import (
	"log"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/proxy"

	"github.com/astianmuchui/mobilesonar/internal/env"
)

func CNNProxyHandler(c *fiber.Ctx) error {
	rest := c.Params("*")
	target := strings.TrimRight(env.C.MLAPIUrl, "/") + "/" + strings.TrimLeft(rest, "/")
	if q := string(c.Request().URI().QueryString()); q != "" {
		target += "?" + q
	}

	log.Printf("cnn proxy: %s %s -> %s (clen=%d)",
		c.Method(), c.OriginalURL(), target, c.Request().Header.ContentLength())

	if err := proxy.Do(c, target); err != nil {
		log.Printf("cnn proxy: forward to %s failed: %v", target, err)
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{"error": "cnn service unreachable"})
	}

	c.Response().Header.Del(fiber.HeaderServer)
	log.Printf("cnn proxy: %s -> %d", target, c.Response().StatusCode())
	return nil
}
