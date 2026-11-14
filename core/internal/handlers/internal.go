package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/luketaylor45/atlas/core/internal/database"
	"github.com/luketaylor45/atlas/core/internal/middleware"
	"github.com/luketaylor45/atlas/core/internal/models"
)

type ServiceStatusRequest struct {
	Status   string `json:"status" binding:"required"`
	Stage    string `json:"stage"`
	Progress *int   `json:"progress"`
}

// Service statuses a node may report.
const (
	StatusInstalling         = "installing"
	StatusInstallationFailed = "installation_failed"
	StatusStarting           = "starting"
	StatusRunning            = "running"
	StatusStopping           = "stopping"
	StatusOffline            = "offline"
)

var reportableStatuses = map[string]bool{
	StatusInstalling:         true,
	StatusInstallationFailed: true,
	StatusStarting:           true,
	StatusRunning:            true,
	StatusStopping:           true,
	StatusOffline:            true,
}

// HandleServerStatusUpdate records a status reported by the node that hosts the service.
func HandleServerStatusUpdate(c *gin.Context) {
	uuid := c.Param("uuid")
	node := middleware.NodeFromContext(c)

	var req ServiceStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if !reportableStatuses[req.Status] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Unknown service status"})
		return
	}

	updates := map[string]interface{}{
		"status": req.Status,
	}
	if req.Stage != "" {
		stage := req.Stage
		if len(stage) > 255 {
			stage = stage[:252] + "..."
		}
		updates["installation_stage"] = stage
	}
	if req.Progress != nil {
		if *req.Progress < 0 || *req.Progress > 100 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Progress must be between 0 and 100"})
			return
		}
		updates["installation_progress"] = *req.Progress
	}

	// A node can only report on its own services.
	result := database.DB.Model(&models.Service{}).Where("uuid = ? AND node_id = ?", uuid, node.ID).Updates(updates)
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update status"})
		return
	}
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Service not found on this node"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "updated"})
}
