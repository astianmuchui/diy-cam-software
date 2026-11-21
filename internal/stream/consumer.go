package stream

import (
	"context"
	"log"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"

	rdb "github.com/astianmuchui/mobilesonar/internal/redis"
)

const (
	MaxRetries = 3

	KeyDeviceID        = "device_id"
	KeyEventType       = "event_type"
	KeySeverity        = "severity"
	KeyReadingID       = "reading_id"
	KeyLat             = "lat"
	KeyLon             = "lon"
	KeyOccurredAt      = "occurred_at"
	KeyCameraReadingID = "camera_reading_id"

	ReadingGroup    = "reading-processors"
	ReadingConsumer = "worker-1"
	EventGroup      = "event-processors"
	EventConsumer   = "worker-1"
)

func EnsureGroups(ctx context.Context) {
	groups := []struct {
		stream string
		group  string
	}{
		{StreamReadings, "reading-processors"},
		{StreamEvents, "event-processors"},
	}

	for _, g := range groups {
		err := rdb.Client.XGroupCreateMkStream(ctx, g.stream, g.group, "$").Err()
		if err != nil && err.Error() != "BUSYGROUP Consumer Group name already exists" {
			log.Printf("stream: group create warning [%s/%s]: %v", g.stream, g.group, err)
		}
	}
}

func ReadGroup(ctx context.Context, stream, group, consumer string, count int64, blockDur time.Duration) ([]redis.XMessage, error) {
	res, err := rdb.Client.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group:    group,
		Consumer: consumer,
		Streams:  []string{stream, ">"},
		Count:    count,
		Block:    blockDur,
	}).Result()
	if err != nil {
		return nil, err
	}
	if len(res) == 0 {
		return nil, nil
	}
	return res[0].Messages, nil
}

func Ack(ctx context.Context, stream, group, msgID string) {
	if err := rdb.Client.XAck(ctx, stream, group, msgID).Err(); err != nil {
		log.Printf("stream: ack failed [%s]: %v", msgID, err)
	}
}

func RetryCount(ctx context.Context, stream, group, msgID string) int64 {
	pending, err := rdb.Client.XPendingExt(ctx, &redis.XPendingExtArgs{
		Stream: stream,
		Group:  group,
		Start:  msgID,
		End:    msgID,
		Count:  1,
	}).Result()
	if err != nil || len(pending) == 0 {
		return 0
	}
	return pending[0].RetryCount
}

func ParseFloat64(v map[string]any, key string) float64 {
	if s, ok := v[key].(string); ok {
		f, _ := strconv.ParseFloat(s, 64)
		return f
	}
	return 0
}

func ParseFloat32(v map[string]any, key string) float32 {
	return float32(ParseFloat64(v, key))
}

func ParseInt(v map[string]any, key string) int {
	if s, ok := v[key].(string); ok {
		n, _ := strconv.Atoi(s)
		return n
	}
	return 0
}

func ParseInt64(v map[string]any, key string) int64 {
	if s, ok := v[key].(string); ok {
		n, _ := strconv.ParseInt(s, 10, 64)
		return n
	}
	return 0
}

func ParseBool(v map[string]any, key string) bool {
	if s, ok := v[key].(string); ok {
		return s == "true"
	}
	return false
}

func ParseString(v map[string]any, key string) string {
	if s, ok := v[key].(string); ok {
		return s
	}
	return ""
}
