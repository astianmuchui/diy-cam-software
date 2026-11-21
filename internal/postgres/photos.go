package postgres

import (
	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/astianmuchui/mobilesonar/internal/models"
)

func CreatePhoto(db *gorm.DB, p *models.Photo) error {
	return db.Create(p).Error
}

func GetPhotoByID(db *gorm.DB, id uuid.UUID) (*models.Photo, error) {
	var p models.Photo
	if err := db.Where("id = ?", id).First(&p).Error; err != nil {
		return nil, err
	}
	return &p, nil
}

func GetPhotoByCorrelationID(db *gorm.DB, correlationID string) (*models.Photo, error) {
	var p models.Photo
	if err := db.Where("correlation_id = ?", correlationID).
		Order("created_at DESC").First(&p).Error; err != nil {
		return nil, err
	}
	return &p, nil
}

func GetPhotosBySession(db *gorm.DB, sessionID uuid.UUID) ([]models.Photo, error) {
	var photos []models.Photo
	err := db.Where("session_id = ?", sessionID).
		Order("captured_at ASC").Find(&photos).Error
	return photos, err
}

func UpdatePhotoCNNResult(db *gorm.DB, id uuid.UUID, fishDetected bool, confidence float64, sceneClass, species string, speciesConf float64, fishCount int) error {
	return db.Model(&models.Photo{}).Where("id = ?", id).Updates(map[string]any{
		"cnn_fish_detected":      fishDetected,
		"cnn_confidence":         confidence,
		"cnn_scene_class":        sceneClass,
		"cnn_species":            species,
		"cnn_species_confidence": speciesConf,
		"cnn_fish_count":         fishCount,
	}).Error
}

func LinkPhotoToReading(db *gorm.DB, photoID, readingID uuid.UUID, sessionID *uuid.UUID) error {
	updates := map[string]any{"reading_id": readingID}
	if sessionID != nil {
		updates["session_id"] = *sessionID
	}
	return db.Model(&models.Photo{}).Where("id = ?", photoID).Updates(updates).Error
}

func GetActiveSession(db *gorm.DB) (*models.FishingSession, error) {
	var s models.FishingSession
	if err := db.Where("status = ?", "active").
		Order("started_at DESC").First(&s).Error; err != nil {
		return nil, err
	}
	return &s, nil
}
