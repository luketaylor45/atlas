package database

import (
	"log"

	"github.com/luketaylor45/atlas/core/internal/config"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var DB *gorm.DB

func Connect() {
	var err error
	dsn := config.AppConfig.DatabaseURL
	DB, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	} // Note: In dev, maybe don't fatal immediately if user hasn't setup DB yet? For now, fatal is fine.

	// AutoMigrate removed from here to allow manual control in reset tool/main.go
	log.Println("Connected to Database")
}
