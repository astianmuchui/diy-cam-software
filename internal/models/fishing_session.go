package models

import (
	"time"

	"github.com/google/uuid"
)

type FishingSession struct {
	ID        uuid.UUID  `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	DeviceID  string     `gorm:"index;not null" json:"device_id"`
	Name      string     `gorm:"not null" json:"name"`
	Status    string     `gorm:"default:'active'" json:"status"`
	Notes     string     `json:"notes"`
	StartedAt time.Time  `gorm:"index;not null" json:"started_at"`
	EndedAt   *time.Time `json:"ended_at"`
	TotalFish int        `gorm:"default:0" json:"total_fish"`
	CreatedAt time.Time  `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time  `gorm:"autoUpdateTime" json:"updated_at"`

	Device   Device          `gorm:"foreignKey:DeviceID;references:DeviceID" json:"-"`
	Readings []SensorReading `gorm:"foreignKey:SessionID" json:"-"`
}
