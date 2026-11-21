package postgres

import (
	"github.com/astianmuchui/mobilesonar/internal/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

func CreateEvent(db *gorm.DB, e *models.Event) error {
	e.Acknowledged = true
	return db.Create(e).Error
}

func GetAllEvents(db *gorm.DB, page, pageSize int) ([]models.Event, int64, error) {
	var events []models.Event
	var total int64
	offset := (page - 1) * pageSize

	db.Model(&models.Event{}).Count(&total)
	err := db.Order("occurred_at DESC").Offset(offset).Limit(pageSize).Find(&events).Error
	return events, total, err
}

func GetRecentEvents(db *gorm.DB, limit int) ([]models.Event, error) {
	var events []models.Event
	err := db.Order("occurred_at DESC").Limit(limit).Find(&events).Error
	return events, err
}

func GetUnacknowledgedCount(db *gorm.DB) (int64, error) {
	var count int64
	err := db.Model(&models.Event{}).Where("acknowledged = FALSE").Count(&count).Error
	return count, err
}

func AcknowledgeEvent(db *gorm.DB, eventID uuid.UUID) error {
	return db.Model(&models.Event{}).
		Where("id = ?", eventID).
		Update("acknowledged", true).Error
}

func AcknowledgeAllEvents(db *gorm.DB) (int64, error) {
	res := db.Model(&models.Event{}).
		Where("acknowledged = FALSE").
		Update("acknowledged", true)
	if res.Error != nil {
		return 0, res.Error
	}
	return res.RowsAffected, nil
}

func GetEventsByDevice(db *gorm.DB, deviceID string, limit int) ([]models.Event, error) {
	var events []models.Event
	err := db.Where("device_id = ?", deviceID).
		Order("occurred_at DESC").Limit(limit).Find(&events).Error
	return events, err
}

func GetCriticalEvents(db *gorm.DB, limit int) ([]models.Event, error) {
	var events []models.Event
	err := db.Where("severity IN ('warning','critical') AND acknowledged = FALSE").
		Order("occurred_at DESC").Limit(limit).Find(&events).Error
	return events, err
}
