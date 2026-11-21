package sessions

import (
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2/middleware/session"
	redisstorage "github.com/gofiber/storage/redis/v3"

	"github.com/astianmuchui/mobilesonar/internal/env"
)

var Store *session.Store

func Init() {
	port, err := strconv.Atoi(env.C.RedisPort)
	if err != nil {
		port = 6379
	}

	storage := redisstorage.New(redisstorage.Config{
		Host:     env.C.RedisHost,
		Port:     port,
		Password: env.C.RedisPassword,
		Database: 1,
	})

	Store = session.New(session.Config{
		Storage:        storage,
		Expiration:     8 * time.Hour,
		CookieName:     "sonar_session",
		CookieSecure:   false,
		CookieHTTPOnly: true,
		CookieSameSite: "Lax",
	})
}
