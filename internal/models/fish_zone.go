package models

import (
	"time"

	"github.com/google/uuid"
)

type FishZone struct {
	ID               uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	Name             string
	CenterLat        float64
	CenterLon        float64
	RadiusMeters     float64
	FishProbability  float64 `gorm:"default:0"`
	DominantFishType string
	TotalDetections  int `gorm:"default:0"`
	LastDetection    *time.Time
	UpdatedAt        time.Time `gorm:"autoUpdateTime"`
}
