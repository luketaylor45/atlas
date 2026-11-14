package main

import (
	"context"
	"log"

	"github.com/gin-gonic/gin"
	"github.com/luketaylor45/atlas/daemon/internal/api"
	"github.com/luketaylor45/atlas/daemon/internal/config"
	"github.com/luketaylor45/atlas/daemon/internal/docker"
	"github.com/luketaylor45/atlas/daemon/internal/serverfs"
	"github.com/luketaylor45/atlas/daemon/internal/sftp"
)

func main() {
	log.Println("[DEBUG] Loading configuration...")
	config.LoadConfig()
	serverfs.ConfigureProcessPermissions()

	log.Printf("[DEBUG] Repairing server storage permissions for %s...", config.NodeConfig.DataPath)
	if err := serverfs.RepairExistingServices(config.NodeConfig.DataPath); err != nil {
		log.Printf("[WARN] Some server storage permissions could not be repaired: %v", err)
	}

	log.Println("[DEBUG] Initializing Docker client...")
	docker.Init()

	log.Println("Starting Atlas Daemon...")
	log.Printf("Connecting to Core at %s", config.NodeConfig.CoreURL)

	api.CleanupInterruptedInstalls(context.Background())

	// Start SFTP Server
	log.Println("[DEBUG] Starting SFTP subsystem...")
	sftp.SetAuthValidator(api.ValidateSFTPCredentials)
	sftpServer := sftp.NewServer(config.NodeConfig.SFTPPort, config.NodeConfig.DataPath)
	if err := sftpServer.Start(); err != nil {
		log.Printf("[WARN] SFTP Server failed to start: %v", err)
	} else {
		log.Printf("[SFTP] ✓ SFTP Server enabled on port %s", config.NodeConfig.SFTPPort)
	}

	// Start Heartbeat
	go api.StartHeartbeat()

	if !api.NodeTokenConfigured() {
		log.Println("[WARN] NODE_TOKEN is not set. Every API request will be rejected until the token from Admin > Nodes is configured.")
	}

	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())
	// Only Core talks to the daemon, so no proxy headers are trusted and no
	// browser origins are allowed.
	_ = r.SetTrustedProxies(nil)

	r.GET("/status", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status": "online",
			"system": "Atlas Daemon",
		})
	})

	// Every API route requires the node token.
	servers := r.Group("/api/servers", api.RequireNodeToken())
	servers.POST("", api.CreateServer)
	servers.GET("", api.HandleListServers)

	server := servers.Group("/:uuid", api.RequireServiceUUID())
	server.POST("/power", api.HandlePowerAction)
	server.GET("/console", api.HandleConsole)
	server.GET("/stats", api.HandleStats)
	server.POST("/command", api.HandleSendCommand)
	server.POST("/reinstall", api.HandleReinstall)
	server.POST("/rebuild", api.HandleRebuild)
	server.POST("/install/cancel", api.HandleCancelInstall)
	server.GET("/install/log", api.HandleInstallLog)
	server.PUT("", api.UpdateServer)
	server.DELETE("", api.DeleteServer)

	// File Management
	server.GET("/files/list", api.ListFiles)
	server.GET("/files/content", api.GetFileContent)
	server.POST("/files/write", api.WriteFile)
	server.POST("/files/create-folder", api.CreateFolder)
	server.POST("/files/extract", api.ExtractArchive)
	server.POST("/files/upload", api.UploadFile)
	server.DELETE("/files", api.DeleteFile)

	log.Printf("Daemon listening on port %s", config.NodeConfig.Port)
	if err := r.Run(":" + config.NodeConfig.Port); err != nil {
		log.Fatalf("Failed to start daemon: %v", err)
	}
}
