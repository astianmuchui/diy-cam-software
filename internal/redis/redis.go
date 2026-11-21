package redisclient

import (
	"context"
	"fmt"
	"log"

	"github.com/astianmuchui/mobilesonar/internal/env"
	"github.com/redis/go-redis/v9"
)

var Client *redis.Client
var Ctx = context.Background()

func Connect() {
	Client = redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%s", env.C.RedisHost, env.C.RedisPort),
		Password: env.C.RedisPassword,
		DB:       0,
	})

	if _, err := Client.Ping(Ctx).Result(); err != nil {
		log.Fatalf("redis: failed to connect: %v", err)
	}

	log.Println("redis: connected")
}
