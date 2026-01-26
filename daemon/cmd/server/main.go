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

	// Secure Routes
	r.POST("/api/servers", api.CreateServer)
	r.POST("/api/servers/:uuid/power", api.HandlePowerAction)
	r.GET("/api/servers/:uuid/console", api.HandleConsole)
	r.GET("/api/servers/:uuid/stats", api.HandleStats)
	r.POST("/api/servers/:uuid/command", api.HandleSendCommand)
	r.POST("/api/servers/:uuid/reinstall", api.HandleReinstall)
	r.PUT("/api/servers/:uuid", api.UpdateServer)
	r.DELETE("/api/servers/:uuid", api.DeleteServer)

	// File Management
	r.GET("/api/servers/:uuid/files/list", api.ListFiles)
	r.GET("/api/servers/:uuid/files/content", api.GetFileContent)
	r.POST("/api/servers/:uuid/files/write", api.WriteFile)
	r.POST("/api/servers/:uuid/files/create-folder", api.CreateFolder)
	r.POST("/api/servers/:uuid/files/upload", api.UploadFile)
	r.DELETE("/api/servers/:uuid/files", api.DeleteFile)

	log.Printf("Daemon listening on port %s", config.NodeConfig.Port)
	if err := r.Run(":" + config.NodeConfig.Port); err != nil {
		log.Fatalf("Failed to start daemon: %v", err)
	}
}
