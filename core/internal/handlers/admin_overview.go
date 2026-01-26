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
	var totalNodes int64
	var healthyNodes int64

	database.DB.Model(&models.User{}).Count(&userCount)
	database.DB.Model(&models.Service{}).Count(&serviceCount)
	database.DB.Model(&models.Node{}).Count(&totalNodes)
	database.DB.Model(&models.Node{}).Where("is_online = ?", true).Count(&healthyNodes)

	// Calculate total service allocations
	var stats struct {
		TotalMemory uint64 `json:"total_memory"`
		TotalDisk   uint64 `json:"total_disk"`
		TotalCPU    uint64 `json:"total_cpu"`
	}
	database.DB.Model(&models.Service{}).Select("SUM(memory) as total_memory, SUM(disk) as total_disk, SUM(cpu) as total_cpu").Scan(&stats)

	// Fetch Node Status Details for Load Visualization
	var nodeDetails []models.Node
	database.DB.Find(&nodeDetails)

	// Fetch Recent Events
	var logs []models.ActivityLog
	database.DB.Order("created_at DESC").Limit(5).Find(&logs)

	health := "OPTIMAL"
	if totalNodes > 0 && healthyNodes < totalNodes {
		health = "DEGRADED"
	}

	c.JSON(http.StatusOK, gin.H{
		"users":         userCount,
		"services":      serviceCount,
		"nodes_total":   totalNodes,
		"nodes_healthy": healthyNodes,
		"total_memory":  stats.TotalMemory,
		"total_disk":    stats.TotalDisk,
		"total_cpu":     stats.TotalCPU,
		"nodes":         nodeDetails,
		"logs":          logs,
		"system_health": health,
	})
}
