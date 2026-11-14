package main

import (
	"context"
	"log"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/luketaylor45/atlas/core/internal/config"
	"github.com/luketaylor45/atlas/core/internal/database"
	"github.com/luketaylor45/atlas/core/internal/handlers"
	"github.com/luketaylor45/atlas/core/internal/router"
	"github.com/luketaylor45/atlas/core/internal/utils"
)

func main() {
	config.LoadConfig()

	// Connect DB
	database.Connect()

	// Auto Migrate
	if err := database.Migrate(); err != nil {
		log.Fatalf("Database migration failed: %v", err)
	}

	if err := utils.EnsureJWTSecret(); err != nil {
		log.Fatalf("Signing secret unavailable: %v", err)
	}

	if config.AppConfig.Environment != "development" {
		gin.SetMode(gin.ReleaseMode)
	}
	r := gin.Default()

	// Only the configured reverse proxies may set the client address, so the
	// login rate limit and activity log cannot be dodged with a forged header.
	var proxies []string
	for _, proxy := range strings.Split(config.AppConfig.TrustedProxies, ",") {
		if proxy = strings.TrimSpace(proxy); proxy != "" {
			proxies = append(proxies, proxy)
		}
	}
	if err := r.SetTrustedProxies(proxies); err != nil {
		log.Fatalf("Invalid TRUSTED_PROXIES: %v", err)
	}

	// Setup routes
	router.Setup(r)

	go handlers.StartReconciler(context.Background())

	log.Printf("Atlas Core starting on port %s", config.AppConfig.Port)
	if err := r.Run(":" + config.AppConfig.Port); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
