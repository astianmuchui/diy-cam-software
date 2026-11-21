package models

import (
	"time"

	"github.com/google/uuid"
)

type Photo struct {
	ID uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`

	SessionID *uuid.UUID `gorm:"type:uuid;index" json:"session_id"`

	ReadingID *uuid.UUID `gorm:"type:uuid;index" json:"reading_id"`

	DeviceID string `gorm:"index" json:"device_id"`

	CorrelationID string `gorm:"index" json:"correlation_id"`

	FilePath  string `gorm:"not null" json:"file_path"`
	SizeBytes int    `json:"size_bytes"`

	CNNFishDetected *bool    `gorm:"column:cnn_fish_detected" json:"cnn_fish_detected"`
	CNNConfidence   *float64 `gorm:"column:cnn_confidence" json:"cnn_confidence"`
	CNNSceneClass   string   `gorm:"column:cnn_scene_class" json:"cnn_scene_class"`
	CNNSpecies      string   `gorm:"column:cnn_species" json:"cnn_species"`
	CNNSpeciesConf  *float64 `gorm:"column:cnn_species_confidence" json:"cnn_species_confidence"`
	CNNFishCount    *int     `gorm:"column:cnn_fish_count" json:"cnn_fish_count"`

	CapturedAt time.Time `gorm:"index" json:"captured_at"`
	CreatedAt  time.Time `gorm:"autoCreateTime" json:"created_at"`
}
