package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/luketaylor45/atlas/core/internal/database"
	"github.com/luketaylor45/atlas/core/internal/models"
)

// GetAdminOverview returns system-wide statistics for the admin dashboard
func GetAdminOverview(c *gin.Context) {
	var userCount int64
	var serviceCount int64
	var nodeCount int64

	database.DB.Model(&models.User{}).Count(&userCount)
	database.DB.Model(&models.Service{}).Count(&serviceCount)
	database.DB.Model(&models.Node{}).Count(&nodeCount)

	// Calculate total resources
	var stats struct {
		TotalMemory uint64 `json:"total_memory"`
		TotalDisk   uint64 `json:"total_disk"`
	}

	database.DB.Model(&models.Service{}).Select("SUM(memory) as total_memory, SUM(disk) as total_disk").Scan(&stats)

	c.JSON(http.StatusOK, gin.H{
		"users":        userCount,
		"services":     serviceCount,
		"nodes":        nodeCount,
		"total_memory": stats.TotalMemory,
		"total_disk":   stats.TotalDisk,
	})
}
