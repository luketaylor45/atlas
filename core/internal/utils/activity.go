package utils

import (
	"encoding/json"

	"github.com/gin-gonic/gin"
	"github.com/luketaylor45/atlas/core/internal/database"
	"github.com/luketaylor45/atlas/core/internal/models"
)

// LogActivity creates an activity log entry
func LogActivity(c *gin.Context, serviceID uint, action string, resource string, description string, metadata map[string]interface{}) {
	userID, exists := c.Get("user_id")
	if !exists {
		userID = uint(0) // System action
	}

	metadataJSON := ""
	if metadata != nil {
		if bytes, err := json.Marshal(metadata); err == nil {
			metadataJSON = string(bytes)
		}
	}

	log := models.ActivityLog{
		ServiceID:   serviceID,
		UserID:      userID.(uint),
		Action:      action,
		Resource:    resource,
		Description: description,
		IPAddress:   c.ClientIP(),
		UserAgent:   c.GetHeader("User-Agent"),
		Metadata:    metadataJSON,
	}

	// Fire and forget - don't block the request
	go database.DB.Create(&log)
}

// LogActivityDirect creates a log entry without gin context (for background tasks)
func LogActivityDirect(serviceID uint, userID uint, action string, resource string, description string, ipAddress string) {
	log := models.ActivityLog{
		ServiceID:   serviceID,
		UserID:      userID,
		Action:      action,
		Resource:    resource,
		Description: description,
		IPAddress:   ipAddress,
		UserAgent:   "System",
	}

	database.DB.Create(&log)
}
