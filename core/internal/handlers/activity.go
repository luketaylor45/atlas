package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/luketaylor45/atlas/core/internal/database"
	"github.com/luketaylor45/atlas/core/internal/models"
)

// GetServiceActivityLogs returns activity logs for a service
func GetServiceActivityLogs(c *gin.Context) {
	// Owner or admin only: the log exposes other users and their IP addresses.
	service, ok := getServiceByUUID(c)
	if !ok {
		return
	}

	// Get pagination params
	limit := 50
	offset := 0
	if limitParam := c.Query("limit"); limitParam != "" {
		if l, err := strconv.Atoi(limitParam); err == nil && l > 0 && l <= 200 {
			limit = l
		}
	}
	if offsetParam := c.Query("offset"); offsetParam != "" {
		if o, err := strconv.Atoi(offsetParam); err == nil && o >= 0 {
			offset = o
		}
	}

	// Get activity logs
	var logs []models.ActivityLog
	query := database.DB.Where("service_id = ?", service.ID).
		Preload("User").
		Order("created_at DESC").
		Limit(limit).
		Offset(offset)

	// Apply action filter if provided
	if action := c.Query("action"); action != "" {
		query = query.Where("action = ?", action)
	}

	if err := query.Find(&logs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch activity logs"})
		return
	}

	// Get total count
	var total int64
	countQuery := database.DB.Model(&models.ActivityLog{}).Where("service_id = ?", service.ID)
	if action := c.Query("action"); action != "" {
		countQuery = countQuery.Where("action = ?", action)
	}
	countQuery.Count(&total)

	c.JSON(http.StatusOK, gin.H{
		"logs":   logs,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}
