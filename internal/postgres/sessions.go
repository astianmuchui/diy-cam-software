package postgres

import (
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/astianmuchui/mobilesonar/internal/models"
)

var ErrActiveSessionExists = errors.New("device already has an active session")
var ErrSessionNotActive = errors.New("session is not active")

type SessionCoverageRow struct {
	SessionID    string  `gorm:"column:session_id"`
	Name         string  `gorm:"column:name"`
	DeviceID     string  `gorm:"column:device_id"`
	Status       string  `gorm:"column:status"`
	ReadingCount int64   `gorm:"column:reading_count"`
	Latitude     float64 `gorm:"column:latitude"`
	Longitude    float64 `gorm:"column:longitude"`
}

func GetSessionCoveragePoints(db *gorm.DB) ([]SessionCoverageRow, error) {
	var rows []SessionCoverageRow
	sql := `
		WITH ranked AS (
			SELECT
				s.id::text                                AS session_id,
				s.name,
				s.device_id,
				s.status,
				COUNT(r.id) OVER (PARTITION BY s.id)     AS reading_count,
				ROUND(r.latitude::numeric,  4)            AS latitude,
				ROUND(r.longitude::numeric, 4)            AS longitude,
				ROW_NUMBER() OVER (
					PARTITION BY s.id
					ORDER BY r.recorded_at ASC
				)                                         AS rn,
				COUNT(r.id) OVER (PARTITION BY s.id)     AS total,
				DENSE_RANK() OVER (ORDER BY s.started_at DESC) AS sess_rank
			FROM fishing_sessions s
			JOIN sensor_readings r ON r.session_id = s.id
			WHERE ABS(r.latitude)  > 0.001
			  AND ABS(r.longitude) > 0.001
		)
		SELECT session_id, name, device_id, status, reading_count, latitude, longitude
		FROM ranked
		WHERE sess_rank    <= 30
		  AND (rn = 1 OR rn % GREATEST(1, (total / 200)::int) = 0)
		ORDER BY sess_rank ASC, rn ASC
	`
	err := db.Raw(sql).Scan(&rows).Error
	return rows, err
}

func CreateSession(db *gorm.DB, s *models.FishingSession) error {
	var count int64
	db.Model(&models.FishingSession{}).
		Where("device_id = ? AND status = 'active'", s.DeviceID).
		Count(&count)
	if count > 0 {
		return ErrActiveSessionExists
	}
	s.ID = uuid.New()
	s.Status = "active"
	if s.StartedAt.IsZero() {
		s.StartedAt = time.Now()
	}
	return db.Create(s).Error
}

func GetActiveSessionForDevice(db *gorm.DB, deviceID string) (*models.FishingSession, error) {
	var s models.FishingSession
	err := db.Where("device_id = ? AND status = 'active'", deviceID).First(&s).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &s, err
}

func GetSessionByID(db *gorm.DB, id uuid.UUID) (*models.FishingSession, error) {
	var s models.FishingSession
	err := db.First(&s, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &s, err
}

func EndSession(db *gorm.DB, id uuid.UUID) error {
	now := time.Now()
	res := db.Model(&models.FishingSession{}).
		Where("id = ? AND status = 'active'", id).
		Updates(map[string]any{
			"status":   "ended",
			"ended_at": now,
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrSessionNotActive
	}
	return nil
}

func UpdateSessionTotalFish(db *gorm.DB, id uuid.UUID, total int) error {
	return db.Model(&models.FishingSession{}).Where("id = ?", id).
		Update("total_fish", total).Error
}

func RecomputeSessionTotalFish(db *gorm.DB, id uuid.UUID) (int, error) {
	var catchTotal *int
	if err := db.Model(&models.CatchLog{}).
		Where("session_id = ?", id).
		Select("COALESCE(SUM(count), 0)").Scan(&catchTotal).Error; err != nil {
		return 0, err
	}

	total := 0
	if catchTotal != nil && *catchTotal > 0 {
		total = *catchTotal
	} else {
		var photoTotal *int
		if err := db.Model(&models.Photo{}).
			Where("session_id = ? AND cnn_fish_count IS NOT NULL", id).
			Select("COALESCE(SUM(cnn_fish_count), 0)").Scan(&photoTotal).Error; err != nil {
			return 0, err
		}
		if photoTotal != nil {
			total = *photoTotal
		}
	}

	if err := UpdateSessionTotalFish(db, id, total); err != nil {
		return 0, err
	}
	return total, nil
}

func GetAllSessions(db *gorm.DB, page, pageSize int) ([]models.FishingSession, int64, error) {
	var sessions []models.FishingSession
	var total int64
	offset := (page - 1) * pageSize
	db.Model(&models.FishingSession{}).Count(&total)
	err := db.Order("started_at DESC").Offset(offset).Limit(pageSize).Find(&sessions).Error
	return sessions, total, err
}

func GetSessionsByDevice(db *gorm.DB, deviceID string, limit int) ([]models.FishingSession, error) {
	var sessions []models.FishingSession
	err := db.Where("device_id = ?", deviceID).
		Order("started_at DESC").Limit(limit).Find(&sessions).Error
	return sessions, err
}

func GetAllActiveSessions(db *gorm.DB) ([]models.FishingSession, error) {
	var sessions []models.FishingSession
	err := db.Where("status = 'active'").Order("started_at DESC").Find(&sessions).Error
	return sessions, err
}

func GetSessionReadings(db *gorm.DB, sessionID uuid.UUID) ([]models.SensorReading, error) {
	var readings []models.SensorReading
	err := db.Where("session_id = ?", sessionID).
		Order("recorded_at ASC").Find(&readings).Error
	return readings, err
}

func AttachReadingToActiveSession(db *gorm.DB, r *models.SensorReading) {
	sess, err := GetActiveSessionForDevice(db, r.DeviceID)
	if err != nil || sess == nil {
		return
	}
	r.SessionID = &sess.ID
}

func GetSessionCatchLogs(db *gorm.DB, sessionID uuid.UUID) ([]models.CatchLog, error) {
	var logs []models.CatchLog
	err := db.Where("session_id = ?", sessionID).Order("caught_at ASC").Find(&logs).Error
	return logs, err
}

func GetAllReadingsForDevice(db *gorm.DB, deviceID string) ([]models.SensorReading, error) {
	var readings []models.SensorReading
	err := db.Where("device_id = ?", deviceID).Order("recorded_at ASC").Find(&readings).Error
	return readings, err
}

func BulkUpdateReadingDetection(db *gorm.DB, id uuid.UUID, fishDetected bool, confidence float32, fishCount int, fishType string) error {
	return db.Model(&models.SensorReading{}).Where("id = ?", id).Updates(map[string]any{
		"fish_detected":        fishDetected,
		"detection_confidence": confidence,
		"fish_count":           fishCount,
		"fish_type":            fishType,
	}).Error
}

func MarkAllReadingsAsReadForDevice(db *gorm.DB, deviceID string) error {
	now := time.Now()
	return db.Model(&models.SensorReading{}).
		Where("device_id = ?", deviceID).
		Updates(map[string]any{
			"is_read": true,
			"read_at": now,
		}).Error
}
