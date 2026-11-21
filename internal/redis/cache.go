package redisclient

import (
	"context"
	"encoding/json"
	"time"
)

func GetOrSet(ctx context.Context, key string, ttl time.Duration, fn func() (any, error)) ([]byte, error) {
	cached, err := Client.Get(ctx, key).Bytes()
	if err == nil {
		return cached, nil
	}

	data, err := fn()
	if err != nil {
		return nil, err
	}

	b, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	Client.Set(ctx, key, b, ttl)
	return b, nil
}

func Invalidate(ctx context.Context, keys ...string) {
	if len(keys) > 0 {
		Client.Del(ctx, keys...)
	}
}
