package middleware

import (
	"github.com/gofiber/fiber/v2"

	"github.com/astianmuchui/mobilesonar/internal/sessions"
)

func SessionAuth(c *fiber.Ctx) error {
	sess, err := sessions.Store.Get(c)
	if err != nil {
		return c.Redirect("/")
	}

	userID := sess.Get("user_id")
	if userID == nil {
		return c.Redirect("/")
	}

	c.Locals("user_id", userID)
	c.Locals("user_email", sess.Get("user_email"))
	c.Locals("user_role", sess.Get("user_role"))
	c.Locals("user_name", sess.Get("user_name"))

	return c.Next()
}
