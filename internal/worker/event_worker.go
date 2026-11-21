package worker

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/astianmuchui/mobilesonar/internal/models"
	"github.com/astianmuchui/mobilesonar/internal/postgres"
	"github.com/astianmuchui/mobilesonar/internal/stream"
)

func StartEventWorker(ctx context.Context, db *gorm.DB) {
	log.Println("worker: event worker started")
	for {
		select {
		case <-ctx.Done():
			log.Println("worker: event worker stopped")
			return
		default:
		}

		messages, err := stream.ReadGroup(ctx, stream.StreamEvents, stream.EventGroup, stream.EventConsumer, 20, 5*time.Second)
		if err != nil {
			continue
		}

		for _, msg := range messages {
			if err := processEvent(ctx, db, msg.Values); err != nil {
				log.Printf("worker: event failed [%s]: %v", msg.ID, err)
				continue
			}
			stream.Ack(ctx, stream.StreamEvents, stream.EventGroup, msg.ID)
		}
	}
}

func processEvent(ctx context.Context, db *gorm.DB, v map[string]any) error {
	readingIDStr := stream.ParseString(v, stream.KeyReadingID)
	var readingID *uuid.UUID
	if readingIDStr != "" {
		if parsed, err := uuid.Parse(readingIDStr); err == nil {
			readingID = &parsed
		}
	}

	skip := map[string]bool{
		stream.KeyDeviceID: true, stream.KeyEventType: true, stream.KeySeverity: true,
		stream.KeyReadingID: true, stream.KeyLat: true, stream.KeyLon: true, stream.KeyOccurredAt: true,
	}
	payloadMap := make(map[string]any, len(v))
	for k, val := range v {
		if !skip[k] {
			payloadMap[k] = val
		}
	}
	payloadBytes, err := json.Marshal(payloadMap)
	if err != nil {
		log.Printf("worker: payload marshal failed: %v", err)
		payloadBytes = []byte("{}")
	}

	occurredAt := time.Now()
	if ts := stream.ParseInt64(v, stream.KeyOccurredAt); ts > 0 {
		occurredAt = time.Unix(ts, 0)
	}

	severity := stream.ParseString(v, stream.KeySeverity)
	if severity == "" {
		severity = "info"
	}

	event := models.Event{
		ID:         uuid.New(),
		DeviceID:   stream.ParseString(v, stream.KeyDeviceID),
		EventType:  stream.ParseString(v, stream.KeyEventType),
		Severity:   severity,
		Payload:    payloadBytes,
		ReadingID:  readingID,
		Latitude:   stream.ParseFloat64(v, stream.KeyLat),
		Longitude:  stream.ParseFloat64(v, stream.KeyLon),
		OccurredAt: occurredAt,
	}

	return postgres.CreateEvent(db, &event)
}
