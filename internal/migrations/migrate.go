package migrations

import (
	"log"

	"gorm.io/gorm"

	"github.com/astianmuchui/mobilesonar/internal/models"
)

func Run(db *gorm.DB) {
	log.Println("migrations: running auto-migrate...")

	err := db.AutoMigrate(
		&models.User{},
		&models.Device{},
		&models.FishingSession{},
		&models.SensorReading{},
		&models.Event{},
		&models.FishZone{},
		&models.CatchLog{},
		&models.Photo{},
	)
	if err != nil {
		log.Fatalf("migrations: failed: %v", err)
	}

	backfill := `
		INSERT INTO photos (id, session_id, reading_id, device_id, correlation_id,
		                    file_path, cnn_fish_detected, cnn_confidence,
		                    cnn_scene_class, cnn_species, cnn_species_confidence,
		                    captured_at, created_at)
		SELECT gen_random_uuid(), sr.session_id, sr.id, sr.device_id, '',
		       sr.photo_path, sr.cnn_fish_detected, sr.cnn_confidence,
		       sr.cnn_scene_class, sr.cnn_species, sr.cnn_species_confidence,
		       sr.recorded_at, now()
		FROM sensor_readings sr
		WHERE sr.photo_path <> ''
		  AND NOT EXISTS (SELECT 1 FROM photos p WHERE p.reading_id = sr.id)`
	if res := db.Exec(backfill); res.Error != nil {
		log.Printf("migrations: photo backfill warning: %v", res.Error)
	} else if res.RowsAffected > 0 {
		log.Printf("migrations: photo backfill inserted %d rows", res.RowsAffected)
	}

	indexes := []string{
		`CREATE INDEX IF NOT EXISTS idx_sr_fish ON sensor_readings(fish_detected) WHERE fish_detected = TRUE`,
		`CREATE INDEX IF NOT EXISTS idx_sr_device_time ON sensor_readings(device_id, recorded_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_events_severity ON events(severity) WHERE severity IN ('warning','critical')`,
		`CREATE INDEX IF NOT EXISTS idx_catch_logs_caught_at ON catch_logs(caught_at)`,
		`CREATE INDEX IF NOT EXISTS idx_catch_logs_location ON catch_logs(latitude, longitude)`,

		`CREATE UNIQUE INDEX IF NOT EXISTS idx_sessions_one_active_per_device
		    ON fishing_sessions(device_id) WHERE status = 'active'`,
	}

	for _, sql := range indexes {
		if err := db.Exec(sql).Error; err != nil {
			log.Printf("migrations: index warning: %v", err)
		}
	}

	log.Println("migrations: done")
}
