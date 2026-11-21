package postgres

import (
	"errors"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/astianmuchui/mobilesonar/internal/models"
)

func GetReadingByID(db *gorm.DB, id uuid.UUID) (*models.SensorReading, error) {
	var r models.SensorReading
	err := db.First(&r, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &r, err
}

func UpdateReadingPhoto(db *gorm.DB, id uuid.UUID, path string) error {
	return db.Model(&models.SensorReading{}).Where("id = ?", id).Update("photo_path", path).Error
}

func UpdateReadingCNNResult(db *gorm.DB, id uuid.UUID, fishDetected bool, confidence float64, sceneClass, species string, speciesConf float64, fishCount int) error {
	return db.Model(&models.SensorReading{}).Where("id = ?", id).Updates(map[string]any{
		"cnn_fish_detected":      fishDetected,
		"cnn_confidence":         confidence,
		"cnn_scene_class":        sceneClass,
		"cnn_species":            species,
		"cnn_species_confidence": speciesConf,
		"cnn_fish_count":         fishCount,
	}).Error
}

func CreateReading(db *gorm.DB, r *models.SensorReading) error {
	return db.Create(r).Error
}

func GetRecentReadings(db *gorm.DB, limit int) ([]models.SensorReading, error) {
	var readings []models.SensorReading
	err := db.Order("recorded_at DESC").Limit(limit).Find(&readings).Error
	return readings, err
}

func GetAllReadings(db *gorm.DB, page, pageSize int) ([]models.SensorReading, int64, error) {
	var readings []models.SensorReading
	var total int64
	offset := (page - 1) * pageSize

	db.Model(&models.SensorReading{}).Count(&total)
	err := db.Order("recorded_at DESC").Offset(offset).Limit(pageSize).Find(&readings).Error
	return readings, total, err
}

func GetReadingsByDevice(db *gorm.DB, deviceID string, limit int) ([]models.SensorReading, error) {
	var readings []models.SensorReading
	err := db.Where("device_id = ?", deviceID).
		Order("recorded_at DESC").Limit(limit).Find(&readings).Error
	return readings, err
}

type OverviewStats struct {
	TotalReadings int64
	ReadingsToday int64
	FishDetected  int64
	FishToday     int64
}

func GetOverviewStats(db *gorm.DB) (OverviewStats, error) {
	var s OverviewStats
	sql := `
		SELECT
			COUNT(*) AS total_readings,
			COUNT(*) FILTER (WHERE recorded_at >= NOW() - INTERVAL '24 hours')                              AS readings_today,
			COUNT(*) FILTER (WHERE fish_detected = TRUE)                                                    AS fish_detected,
			COUNT(*) FILTER (WHERE fish_detected = TRUE AND recorded_at >= NOW() - INTERVAL '24 hours')     AS fish_today
		FROM sensor_readings
	`
	err := db.Raw(sql).Scan(&s).Error
	return s, err
}

type FishProbabilityResult struct {
	Lat             float64 `json:"lat"`
	Lon             float64 `json:"lon"`
	FishProbability float64 `json:"fish_probability"`
	AvgFishCount    float64 `json:"avg_fish_count"`
	TotalReadings   int64   `json:"total_readings"`
	DominantType    string  `json:"dominant_fish_type"`
}

func GetFishProbabilityNear(db *gorm.DB, lat, lon, radiusMeters float64, days int) ([]FishProbabilityResult, error) {
	var results []FishProbabilityResult

	sql := `
		SELECT
			AVG(latitude)  AS lat,
			AVG(longitude) AS lon,
			COUNT(*) FILTER (WHERE fish_detected = TRUE)::FLOAT / NULLIF(COUNT(*), 0) AS fish_probability,
			COALESCE(AVG(fish_count) FILTER (WHERE fish_detected = TRUE), 0)           AS avg_fish_count,
			COUNT(*)                                                                    AS total_readings,
			COALESCE(MODE() WITHIN GROUP (ORDER BY fish_type), 'unknown')               AS dominant_fish_type
		FROM sensor_readings
		WHERE
			latitude  BETWEEN ? - (? / 111000.0) AND ? + (? / 111000.0)
			AND longitude BETWEEN ? - (? / (111000.0 * COS(RADIANS(?)))) AND ? + (? / (111000.0 * COS(RADIANS(?))))
			AND recorded_at >= NOW() - (INTERVAL '1 day' * ?)
		GROUP BY
			ROUND(latitude::numeric,  3),
			ROUND(longitude::numeric, 3)
		ORDER BY fish_probability DESC
		LIMIT 200
	`
	err := db.Raw(sql,
		lat, radiusMeters, lat, radiusMeters,
		lon, radiusMeters, lat, lon, radiusMeters, lon,
		days,
	).Scan(&results).Error
	return results, err
}

type DensityPoint struct {
	Lat            float64 `json:"lat"`
	Lon            float64 `json:"lon"`
	DetectionCount int64   `json:"count"`
	TotalFish      int64   `json:"total_fish"`
	AvgConfidence  float64 `json:"avg_confidence"`
}

func GetDensityMap(db *gorm.DB, days int) ([]DensityPoint, error) {
	var results []DensityPoint
	sql := `
		SELECT
			ROUND(latitude::numeric,  3)                                      AS lat,
			ROUND(longitude::numeric, 3)                                      AS lon,
			COUNT(*)                                                           AS detection_count,
			COALESCE(SUM(fish_count), 0)                                      AS total_fish,
			COALESCE(AVG(detection_confidence), 0)                            AS avg_confidence
		FROM sensor_readings
		WHERE ABS(latitude)  > 0.001
		  AND ABS(longitude) > 0.001
		  AND recorded_at >= NOW() - (INTERVAL '1 day' * ?)
		GROUP BY ROUND(latitude::numeric, 3), ROUND(longitude::numeric, 3)
		ORDER BY detection_count DESC
		LIMIT 500
	`
	err := db.Raw(sql, days).Scan(&results).Error
	return results, err
}

type DailyDetectionStat struct {
	Date           string  `json:"date"`
	TotalReadings  int64   `json:"total_readings"`
	FishDetections int64   `json:"fish_detections"`
	DetectionRate  float64 `json:"detection_rate"`
	AvgFishCount   float64 `json:"avg_fish_count"`
	AvgConfidence  float64 `json:"avg_confidence"`
	AvgDepthM      float64 `json:"avg_depth_m"`
}

func GetDailyDetectionStats(db *gorm.DB, days int) ([]DailyDetectionStat, error) {
	var results []DailyDetectionStat
	sql := `
		SELECT
			TO_CHAR(DATE(recorded_at AT TIME ZONE 'Africa/Nairobi'), 'YYYYMMDD')          AS date,
			COUNT(*)                                                                        AS total_readings,
			COUNT(*) FILTER (WHERE fish_detected = TRUE)                                   AS fish_detections,
			COUNT(*) FILTER (WHERE fish_detected = TRUE)::FLOAT / NULLIF(COUNT(*), 0)      AS detection_rate,
			COALESCE(AVG(fish_count)             FILTER (WHERE fish_detected = TRUE), 0)   AS avg_fish_count,
			COALESCE(AVG(detection_confidence)   FILTER (WHERE fish_detected = TRUE), 0)   AS avg_confidence,
			COALESCE(AVG(water_depth_m), 0)                                                AS avg_depth_m
		FROM sensor_readings
		WHERE recorded_at >= NOW() - (INTERVAL '1 day' * ?)
		GROUP BY DATE(recorded_at AT TIME ZONE 'Africa/Nairobi')
		ORDER BY DATE(recorded_at AT TIME ZONE 'Africa/Nairobi') DESC
	`
	err := db.Raw(sql, days).Scan(&results).Error
	return results, err
}

type HourlyTrend struct {
	Hour          int     `json:"hour"`
	DetectionRate float64 `json:"detection_rate"`
	AvgFishCount  float64 `json:"avg_fish_count"`
}

func GetHourlyTrends(db *gorm.DB, days int) ([]HourlyTrend, error) {
	var results []HourlyTrend
	sql := `
		SELECT
			EXTRACT(HOUR FROM recorded_at AT TIME ZONE 'Africa/Nairobi')::INT AS hour,
			COUNT(*) FILTER (WHERE fish_detected = TRUE)::FLOAT / NULLIF(COUNT(*), 0) AS detection_rate,
			COALESCE(AVG(fish_count) FILTER (WHERE fish_detected = TRUE), 0)           AS avg_fish_count
		FROM sensor_readings
		WHERE recorded_at >= NOW() - (INTERVAL '1 day' * ?)
		GROUP BY hour
		ORDER BY hour
	`
	err := db.Raw(sql, days).Scan(&results).Error
	return results, err
}
