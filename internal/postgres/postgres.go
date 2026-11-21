package postgres

import (
	"fmt"
	"log"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/astianmuchui/mobilesonar/internal/env"
)

var DB *gorm.DB

func Connect() {
	dsn := fmt.Sprintf(
		"host=%s user=%s password=%s dbname=%s port=%s sslmode=disable TimeZone=Africa/Nairobi",
		env.C.DBHost,
		env.C.DBUser,
		env.C.DBPassword,
		env.C.DBName,
		env.C.DBPort,
	)

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger:                                   logger.Default.LogMode(logger.Info),
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	if err != nil {
		log.Fatalf("postgres: failed to connect: %v", err)
	}

	if err := db.Exec("CREATE EXTENSION IF NOT EXISTS postgis").Error; err != nil {
		log.Printf("postgres: postgis extension warning: %v", err)
	}

	if err := db.Exec("CREATE EXTENSION IF NOT EXISTS pgcrypto").Error; err != nil {
		log.Printf("postgres: pgcrypto extension warning: %v", err)
	}

	DB = db
	log.Println("postgres: connected")
}
