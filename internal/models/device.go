package models

import (
	"time"

	"github.com/google/uuid"
)

type Device struct {
	ID              uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	DeviceID        string    `gorm:"uniqueIndex;not null"`
	Name            string    `gorm:"not null"`
	Description     string
	DeviceKey       string `gorm:"not null"`
	Status          string `gorm:"default:offline"`
	LastSeen        *time.Time
	FirmwareVersion string
	OwnerID         *uuid.UUID
	RegisteredAt    time.Time `gorm:"autoCreateTime"`
	UpdatedAt       time.Time `gorm:"autoUpdateTime"`
}
