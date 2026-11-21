package models

import (
	"time"

	"github.com/google/uuid"
)

type SensorReading struct {
	ID       uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	DeviceID string    `gorm:"index;not null"`

	Latitude    float64
	Longitude   float64
	GPSAccuracy float32

	DistanceCM  float32
	WaterDepthM float32

	AccelX    float32
	AccelY    float32
	AccelZ    float32
	GyroX     float32
	GyroY     float32
	GyroZ     float32
	TiltAngle float32

	FishDetected        bool `gorm:"default:false"`
	FishCount           int  `gorm:"default:0"`
	FishType            string
	DetectionConfidence float32
	EchoAmplitude       float32

	BatteryVoltage   float32
	SignalStrength   int
	TransmissionType string

	SessionID *uuid.UUID `gorm:"type:uuid;index" json:"session_id"`

	PhotoPath       string   `json:"photo_path"`
	CNNFishDetected *bool    `gorm:"column:cnn_fish_detected" json:"cnn_fish_detected"`
	CNNConfidence   *float64 `gorm:"column:cnn_confidence" json:"cnn_confidence"`
	CNNSceneClass   string   `gorm:"column:cnn_scene_class" json:"cnn_scene_class"`
	CNNSpecies      string   `gorm:"column:cnn_species" json:"cnn_species"`
	CNNSpeciesConf  *float64 `gorm:"column:cnn_species_confidence" json:"cnn_species_confidence"`
	CNNFishCount    *int     `gorm:"column:cnn_fish_count" json:"cnn_fish_count"`

	RecordedAt time.Time `gorm:"index;not null"`
	CreatedAt  time.Time `gorm:"autoCreateTime"`
	IsRead     bool      `gorm:"default:false;index"`
	ReadAt     *time.Time

	Device  Device          `gorm:"foreignKey:DeviceID;references:DeviceID"`
	Session *FishingSession `gorm:"foreignKey:SessionID"`
}
