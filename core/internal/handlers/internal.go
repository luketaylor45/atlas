package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/luketaylor45/atlas/core/internal/database"
	"github.com/luketaylor45/atlas/core/internal/models"
)

type ServiceStatusRequest struct {
	Token    string `json:"token" binding:"required"`
	Status   string `json:"status" binding:"required"`
	Stage    string `json:"stage"`
	Progress int    `json:"progress"`
}

func HandleServerStatusUpdate(c *gin.Context) {
	uuid := c.Param("uuid")
	var req ServiceStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Verify node token
	var node models.Node
	if err := database.DB.Where("token = ?", req.Token).First(&node).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid node token"})
		return
	}

	// Update service status
	updates := map[string]interface{}{
		"status": req.Status,
	}
	if req.Stage != "" {
		updates["installation_stage"] = req.Stage
	}
	if req.Progress > 0 {
		updates["installation_progress"] = req.Progress
	}

	if err := database.DB.Model(&models.Service{}).Where("uuid = ?", uuid).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update status"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "updated"})
}
