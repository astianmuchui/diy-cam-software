package models

import (
	"time"

	"github.com/google/uuid"
)

type CatchLog struct {
	ID        uuid.UUID  `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	SessionID *uuid.UUID `gorm:"type:uuid;index" json:"session_id"`
	DeviceID  string     `gorm:"index" json:"device_id"`
	Latitude  float64    `json:"latitude"`
	Longitude float64    `json:"longitude"`
	FishType  string     `json:"fish_type"`
	Count     int        `json:"count"`
	Notes     string     `json:"notes"`
	CaughtAt  time.Time  `gorm:"index;not null" json:"caught_at"`
	CreatedAt time.Time  `gorm:"autoCreateTime" json:"created_at"`
}
