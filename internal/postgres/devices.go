package postgres

import (
	"time"

	"github.com/astianmuchui/mobilesonar/internal/models"
	"gorm.io/gorm"
)

const (
	DeviceStatusOnline  = "online"
	DeviceStatusOffline = "offline"
	DeviceStatusError   = "error"
)

func GetAllDevices(db *gorm.DB) ([]models.Device, error) {
	var devices []models.Device
	err := db.Order("registered_at DESC").Find(&devices).Error
	return devices, err
}

func GetDeviceByID(db *gorm.DB, deviceID string) (*models.Device, error) {
	var device models.Device
	err := db.Where("device_id = ?", deviceID).First(&device).Error
	return &device, err
}

func CreateDevice(db *gorm.DB, device *models.Device) error {
	return db.Create(device).Error
}

func UpdateDeviceHeartbeat(db *gorm.DB, deviceID string) error {
	now := time.Now()
	return db.Model(&models.Device{}).
		Where("device_id = ?", deviceID).
		Updates(map[string]any{
			"last_seen": now,
			"status":    DeviceStatusOnline,
		}).Error
}

func SetDeviceOffline(db *gorm.DB, deviceID string) error {
	return db.Model(&models.Device{}).
		Where("device_id = ?", deviceID).
		Update("status", DeviceStatusOffline).Error
}

type DeviceStats struct {
	TotalDevices   int64
	OnlineDevices  int64
	OfflineDevices int64
}

func GetDeviceStats(db *gorm.DB) (DeviceStats, error) {
	var stats DeviceStats
	sql := `
		SELECT
			COUNT(*)                                            AS total_devices,
			COUNT(CASE WHEN status = 'online'  THEN 1 END)     AS online_devices,
			COUNT(CASE WHEN status = 'offline' THEN 1 END)     AS offline_devices
		FROM devices
	`
	err := db.Raw(sql).Scan(&stats).Error
	return stats, err
}
