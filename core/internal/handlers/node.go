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
		CPU      float64 `json:"cpu"`
		RAM      float64 `json:"ram"`
		Cores    int     `json:"cores"`
		TotalRAM float64 `json:"total_ram"`
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
	updates := map[string]interface{}{
		"is_online":      true,
		"last_heartbeat": time.Now(),
		"used_cpu":       req.Stats.CPU,
		"used_ram":       uint64(req.Stats.RAM),
	}
	// Older daemons do not report the host's size; keep what is known.
	if req.Stats.Cores > 0 {
		updates["cpu_cores"] = req.Stats.Cores
	}
	if req.Stats.TotalRAM > 0 {
		updates["host_ram"] = uint64(req.Stats.TotalRAM)
	}
	err := database.DB.Model(&models.Node{}).Where("id = ?", node.ID).Updates(updates).Error
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to record heartbeat"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "acknowledged"})
}
