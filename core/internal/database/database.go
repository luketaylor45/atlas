package database

import (
	"log"
	"time"

	"github.com/luketaylor45/atlas/core/internal/config"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var DB *gorm.DB

func Connect() {
	var err error
	dsn := config.AppConfig.DatabaseURL

	// Try to connect up to 10 times
	for i := 1; i <= 10; i++ {
		DB, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
		if err == nil {
			log.Println("Connected to Database")
			return
		}
		log.Printf("Attempt %d: Failed to connect to database. Retrying in 3s...", i)
		time.Sleep(3 * time.Second)
	}

	log.Fatalf("Permanent failure connecting to database: %v", err)
}
