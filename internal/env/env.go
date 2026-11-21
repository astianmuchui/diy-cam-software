package env

import (
	"log"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	AppPort string

	DBHost     string
	DBUser     string
	DBPassword string
	DBName     string
	DBPort     string

	RedisHost     string
	RedisPort     string
	RedisPassword string

	MQTTBroker   string
	MQTTPort     string
	MQTTClientID string
	MQTTUser     string
	MQTTPassword string

	SessionSecret string

	OpenCageKey string

	MLAPIUrl string

	MaxPhotoSizeKB int
}

var C Config

func Load() {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, reading from environment")
	}

	C = Config{
		AppPort: getEnv("APP_PORT", "8090"),

		DBHost:     getEnv("DB_HOST", "localhost"),
		DBUser:     getEnv("DB_USER", "postgres"),
		DBPassword: getEnv("DB_PASSWORD", ""),
		DBName:     getEnv("DB_NAME", "sonarcore"),
		DBPort:     getEnv("DB_PORT", "5432"),

		RedisHost:     getEnv("REDIS_HOST", "localhost"),
		RedisPort:     getEnv("REDIS_PORT", "6379"),
		RedisPassword: getEnv("REDIS_PASSWORD", ""),

		MQTTBroker:   getEnv("MQTT_BROKER", "localhost"),
		MQTTPort:     getEnv("MQTT_PORT", "1883"),
		MQTTClientID: getEnv("MQTT_CLIENT_ID", "sonarcore"),
		MQTTUser:     getEnv("MQTT_USER", ""),
		MQTTPassword: getEnv("MQTT_PASSWORD", ""),

		SessionSecret: getEnv("SESSION_SECRET", "change-me-in-production"),

		OpenCageKey: getEnv("OPENCAGE_API_KEY", ""),

		MLAPIUrl: getEnv("ML_API_URL", "http://localhost:5000"),

		MaxPhotoSizeKB: getEnvInt("MAX_PHOTO_SIZE_KB", 5120),
	}
}

func getEnvInt(key string, fallback int) int {
	if val := os.Getenv(key); val != "" {
		if n, err := strconv.Atoi(val); err == nil && n > 0 {
			return n
		}
		log.Printf("env: invalid %s=%q, using default %d", key, os.Getenv(key), fallback)
	}
	return fallback
}

func getEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}
