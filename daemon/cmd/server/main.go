package main

import (
	"log"

	"github.com/gin-gonic/gin"
	"github.com/luketaylor45/atlas/daemon/internal/api"
	"github.com/luketaylor45/atlas/daemon/internal/config"
	"github.com/luketaylor45/atlas/daemon/internal/docker"
)

func main() {
	config.LoadConfig()
	docker.Init()

	log.Println("Starting Atlas Daemon...")
	log.Printf("Connecting to Core at %s", config.NodeConfig.CoreURL)

	// Start Heartbeat
	go api.StartHeartbeat()

	r := gin.Default()

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
