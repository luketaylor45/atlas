package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/luketaylor45/atlas/core/internal/database"
	"github.com/luketaylor45/atlas/core/internal/models"
	"github.com/luketaylor45/atlas/core/internal/utils"
)

// GetServiceActivityLogs returns activity logs for a service
func GetServiceActivityLogs(c *gin.Context) {
	userID := c.MustGet("user_id").(uint)
	uuid := c.Param("uuid")

	// Get service and verify ownership/access
	service, _, ok := utils.FindServiceForUser(uuid, userID)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "Service not found"})
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
