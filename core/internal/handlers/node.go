package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/luketaylor45/atlas/core/internal/database"
	"github.com/luketaylor45/atlas/core/internal/middleware"
	"github.com/luketaylor45/atlas/core/internal/models"
)

type HeartbeatRequest struct {
	Stats struct {
		CPU float64 `json:"cpu"`
		RAM float64 `json:"ram"`
	} `json:"stats"`
}

func HandleHeartbeat(c *gin.Context) {
	node := middleware.NodeFromContext(c)

	var req HeartbeatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Only the heartbeat columns are written, so a heartbeat can never
	// overwrite an admin's concurrent edit of the node.
	err := database.DB.Model(&models.Node{}).Where("id = ?", node.ID).Updates(map[string]interface{}{
		"is_online":      true,
		"last_heartbeat": time.Now(),
		"used_cpu":       req.Stats.CPU,
		"used_ram":       uint64(req.Stats.RAM),
	}).Error
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to record heartbeat"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "acknowledged"})
}
