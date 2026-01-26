package main

import (
	"log"
	"os"

	"github.com/luketaylor45/atlas/core/internal/config"
	"github.com/luketaylor45/atlas/core/internal/database"
	"github.com/luketaylor45/atlas/core/internal/models"
)

func main() {
	log.Println("!!! SYSTEM RESET TRIGGERED !!!")
	config.LoadConfig()
	database.Connect()

	// 1. Drop Tables in Order
	log.Println("Deleting Database Tables...")
	tables := []interface{}{
		&models.Service{},
		&models.EggVariable{},
		&models.Egg{},
		&models.Nest{},
		&models.Node{},
		&models.User{},
	}

	for _, table := range tables {
		if err := database.DB.Migrator().DropTable(table); err != nil {
			log.Printf("Warning: Failed to drop table: %v", err)
		}
	}

	// 2. Clear Local Data Folder
	dataDir := "C:\\AtlasData"
	log.Printf("Clearing Local Server Files: %s", dataDir)
	if err := os.RemoveAll(dataDir); err != nil {
		log.Printf("Error clearing data folder: %v", err)
	}

	log.Println("--------------------------------------------------")
	log.Println("RESET COMPLETE")
	log.Println("Run: go run cmd/server/main.go to start fresh.")
	log.Println("--------------------------------------------------")
}
