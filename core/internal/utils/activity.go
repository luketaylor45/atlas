package utils

import (
	"encoding/json"
	"log"

	"github.com/gin-gonic/gin"
	"github.com/luketaylor45/atlas/core/internal/database"
	"github.com/luketaylor45/atlas/core/internal/models"
)

// optionalID turns "no service" or "no user" (zero) into NULL. A zero ID
// would violate the foreign key and the entry would be silently lost.
func optionalID(id uint) *uint {
	if id == 0 {
		return nil
	}
	return &id
}

func saveActivity(entry *models.ActivityLog) {
	if err := database.DB.Create(entry).Error; err != nil {
		log.Printf("[Core] Failed to record activity %q: %v", entry.Action, err)
	}
}

// LogActivity records an action taken through the API. A serviceID of zero
// records an instance-wide action (managing nodes, users, eggs).
func LogActivity(c *gin.Context, serviceID uint, action string, resource string, description string, metadata map[string]interface{}) {
	var userID uint
	if value, exists := c.Get("user_id"); exists {
		userID, _ = value.(uint)
	}

	metadataJSON := ""
	if metadata != nil {
		if encoded, err := json.Marshal(metadata); err == nil {
			metadataJSON = string(encoded)
		}
	}

	saveActivity(&models.ActivityLog{
		ServiceID:   optionalID(serviceID),
		UserID:      optionalID(userID),
		Action:      action,
		Resource:    resource,
		Description: description,
		IPAddress:   c.ClientIP(),
		UserAgent:   c.GetHeader("User-Agent"),
		Metadata:    metadataJSON,
	})
}

// LogActivityDirect records an action that did not come through an
// authenticated API request (for example an SFTP login).
func LogActivityDirect(serviceID uint, userID uint, action string, resource string, description string, ipAddress string) {
	saveActivity(&models.ActivityLog{
		ServiceID:   optionalID(serviceID),
		UserID:      optionalID(userID),
		Action:      action,
		Resource:    resource,
		Description: description,
		IPAddress:   ipAddress,
		UserAgent:   "System",
	})
}
