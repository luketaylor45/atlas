package main

import (
	"log"

	"github.com/gin-gonic/gin"
	"github.com/luketaylor45/atlas/core/internal/config"
	"github.com/luketaylor45/atlas/core/internal/database"
	"github.com/luketaylor45/atlas/core/internal/models"
	"github.com/luketaylor45/atlas/core/internal/router"
)

func main() {
	config.LoadConfig()

	// Connect DB
	database.Connect()

	// Auto Migrate
	database.DB.AutoMigrate(&models.User{}, &models.Node{}, &models.Nest{}, &models.Egg{}, &models.EggVariable{}, &models.Service{}, &models.ServiceUser{}, &models.ActivityLog{}, &models.News{})

	// Seed basic data (Nests/Categories)
	//database.SeedDefaults()

	r := gin.Default()

	// Setup routes
	router.Setup(r)

	log.Printf("Atlas Core starting on port %s", config.AppConfig.Port)
	if err := r.Run(":" + config.AppConfig.Port); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
