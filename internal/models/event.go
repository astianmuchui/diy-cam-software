package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
)

type Event struct {
	ID           uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	DeviceID     string    `gorm:"index;not null"`
	EventType    string    `gorm:"index;not null"`
	Severity     string    `gorm:"default:info"`
	Payload      datatypes.JSON
	ReadingID    *uuid.UUID
	Latitude     float64
	Longitude    float64
	Acknowledged bool      `gorm:"default:true"`
	OccurredAt   time.Time `gorm:"index;not null"`
	CreatedAt    time.Time `gorm:"autoCreateTime"`

	Device Device `gorm:"foreignKey:DeviceID;references:DeviceID"`
}
