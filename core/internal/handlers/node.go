package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/luketaylor45/atlas/core/internal/database"
	"github.com/luketaylor45/atlas/core/internal/models"
)

type HeartbeatRequest struct {
	Token string `json:"token" binding:"required"`
	Stats struct {
		CPU float64 `json:"cpu"`
		RAM float64 `json:"ram"`
	} `json:"stats"`
}

func HandleHeartbeat(c *gin.Context) {
	var req HeartbeatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Find node by token
	var node models.Node
	if err := database.DB.Where("token = ?", req.Token).First(&node).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid node token"})
		return
	}

	// Update heartbeat
	node.IsOnline = true
	node.LastHeartbeat = time.Now()
	// In a real app we'd save stats too (Redis/InfluxDB)

	database.DB.Save(&node)

	c.JSON(http.StatusOK, gin.H{"status": "acknowledged"})
}
