package main

import (
	"log"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/luketaylor45/atlas/daemon/internal/api"
	"github.com/luketaylor45/atlas/daemon/internal/config"
	"github.com/luketaylor45/atlas/daemon/internal/docker"
	"github.com/luketaylor45/atlas/daemon/internal/sftp"
)

func main() {
	log.Println("[DEBUG] Loading configuration...")
	config.LoadConfig()
	if err := config.ValidateNodeToken(); err != nil {
		log.Printf("[WARN] %v. All authenticated routes will reject requests until a real token is configured.", err)
	}

	log.Println("[DEBUG] Initializing Docker client...")
	docker.Init()

	log.Println("Starting Atlas Daemon...")
	log.Printf("Connecting to Core at %s", config.NodeConfig.CoreURL)

	// Start SFTP Server
	log.Println("[DEBUG] Starting SFTP subsystem...")
	sftp.SetAuthValidator(api.ValidateSFTPCredentials)
	sftpServer := sftp.NewServer(config.NodeConfig.SFTPPort, "C:\\AtlasData")
	if err := sftpServer.Start(); err != nil {
		log.Printf("[WARN] SFTP Server failed to start: %v", err)
	} else {
		log.Printf("[SFTP] ✓ SFTP Server enabled on port %s", config.NodeConfig.SFTPPort)
	}

	// Start Heartbeat
	go api.StartHeartbeat()

	r := gin.Default()

	// CORS Setup
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization", "X-Node-Token"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	r.GET("/status", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status": "online",
			"system": "Atlas Daemon",
		})
	})

	// Browser-facing routes: node token OR short-lived per-server ticket issued by the Core.
	browser := r.Group("/api/servers/:uuid", api.NodeOrTicketAuth())
	browser.GET("/console", api.HandleConsole)
	browser.GET("/stats", api.HandleStats)

	// Core-only routes: node token required.
	secure := r.Group("/api/servers", api.NodeAuth())
	secure.POST("", api.CreateServer)
	secure.POST("/:uuid/power", api.HandlePowerAction)
	secure.POST("/:uuid/command", api.HandleSendCommand)
	secure.POST("/:uuid/reinstall", api.HandleReinstall)
	secure.PUT("/:uuid", api.UpdateServer)
	secure.DELETE("/:uuid", api.DeleteServer)

	// File Management
	secure.GET("/:uuid/files/list", api.ListFiles)
	secure.GET("/:uuid/files/content", api.GetFileContent)
	secure.POST("/:uuid/files/write", api.WriteFile)
	secure.POST("/:uuid/files/create-folder", api.CreateFolder)
	secure.POST("/:uuid/files/upload", api.UploadFile)
	secure.DELETE("/:uuid/files", api.DeleteFile)

	log.Printf("Daemon listening on port %s", config.NodeConfig.Port)
	if err := r.Run(":" + config.NodeConfig.Port); err != nil {
		log.Fatalf("Failed to start daemon: %v", err)
	}
}
