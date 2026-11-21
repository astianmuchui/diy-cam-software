package postgres

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/astianmuchui/mobilesonar/internal/models"
)

func CreateCatchLog(db *gorm.DB, cl *models.CatchLog) error {
	return db.Create(cl).Error
}

func GetAllCatchLogs(db *gorm.DB, page, pageSize, days int) ([]models.CatchLog, int64, error) {
	var logs []models.CatchLog
	var total int64
	offset := (page - 1) * pageSize

	q := db.Model(&models.CatchLog{})
	if days > 0 {
		cutoff := time.Now().AddDate(0, 0, -days)
		q = q.Where("caught_at >= ?", cutoff)
	}
	q.Count(&total)
	err := q.Order("caught_at DESC").Offset(offset).Limit(pageSize).Find(&logs).Error
	return logs, total, err
}

func GetNearbyCatchLogs(db *gorm.DB, lat, lon, radiusMeters float64, days int) ([]models.CatchLog, error) {
	var logs []models.CatchLog
	sql := `
		SELECT *
		FROM catch_logs
		WHERE
			latitude  BETWEEN ? - (? / 111000.0) AND ? + (? / 111000.0)
			AND longitude BETWEEN ? - (? / (111000.0 * COS(RADIANS(?)))) AND ? + (? / (111000.0 * COS(RADIANS(?))))
			AND caught_at >= NOW() - (? || ' days')::INTERVAL
		ORDER BY caught_at DESC
		LIMIT 200
	`
	err := db.Raw(sql,
		lat, radiusMeters, lat, radiusMeters,
		lon, radiusMeters, lat, lon, radiusMeters, lon,
		days,
	).Scan(&logs).Error
	return logs, err
}

func DeleteCatchLog(db *gorm.DB, id uuid.UUID) error {
	return db.Delete(&models.CatchLog{}, "id = ?", id).Error
}
