package handlers

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/luketaylor45/atlas/core/internal/database"
	"github.com/luketaylor45/atlas/core/internal/models"
	"github.com/luketaylor45/atlas/core/internal/utils"
)

// getServiceByUUID loads a service for its owner or an admin. Sub-users are
// refused: managing who has access, and reading the activity log of everyone
// who does, belongs to the owner alone.
func getServiceByUUID(c *gin.Context) (*models.Service, bool) {
	userID := c.MustGet("user_id").(uint)
	uuid := c.Param("uuid")

	service, subUser, ok := utils.FindServiceForUser(uuid, userID)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "Service not found or access denied"})
		return nil, false
	}
	if subUser != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "Only the server owner can do this"})
		return nil, false
	}
	return service, true
}

// GetServiceUsers returns all sub-users for a service
func GetServiceUsers(c *gin.Context) {
	service, ok := getServiceByUUID(c)
	if !ok {
		return
	}

	var users []models.ServiceUser
	if err := database.DB.Where("service_id = ?", service.ID).Preload("User").Find(&users).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch service users"})
		return
	}

	c.JSON(http.StatusOK, users)
}

type AddServiceUserRequest struct {
	// The user is identified by ID (admins, who can list users) or by exact
	// username (owners, who cannot).
	UserID          uint   `json:"user_id"`
	Username        string `json:"username"`
	CanViewConsole  bool   `json:"can_view_console"`
	CanSendCommands bool   `json:"can_send_commands"`
	CanManageFiles  bool   `json:"can_manage_files"`
	CanEditStartup  bool   `json:"can_edit_startup"`
	CanControlPower bool   `json:"can_control_power"`
	CanAccessSFTP   bool   `json:"can_access_sftp"`
}

// AddServiceUser adds a sub-user to a service
func AddServiceUser(c *gin.Context) {
	service, ok := getServiceByUUID(c)
	if !ok {
		return
	}

	var req AddServiceUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	var target models.User
	lookup := database.DB
	switch {
	case req.UserID != 0:
		lookup = lookup.Where("id = ?", req.UserID)
	case strings.TrimSpace(req.Username) != "":
		lookup = lookup.Where("LOWER(username) = LOWER(?)", strings.TrimSpace(req.Username))
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "Enter the username of the person to add"})
		return
	}
	if err := lookup.First(&target).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "No user with that username exists"})
		return
	}
	req.UserID = target.ID

	// Prevent adding the owner as a sub-user
	if req.UserID == service.UserID {
		c.JSON(http.StatusForbidden, gin.H{"error": "The server owner cannot be added as a sub-user"})
		return
	}

	// Check if user already has access
	var count int64
	database.DB.Model(&models.ServiceUser{}).Where("service_id = ? AND user_id = ?", service.ID, req.UserID).Count(&count)
	if count > 0 {
		c.JSON(http.StatusConflict, gin.H{"error": "User already has access to this service"})
		return
	}

	serviceUser := models.ServiceUser{
		ServiceID:       service.ID,
		UserID:          req.UserID,
		CanViewConsole:  req.CanViewConsole,
		CanSendCommands: req.CanSendCommands,
		CanManageFiles:  req.CanManageFiles,
		CanEditStartup:  req.CanEditStartup,
		CanControlPower: req.CanControlPower,
		CanAccessSFTP:   req.CanAccessSFTP,
	}

	if err := database.DB.Create(&serviceUser).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to add user"})
		return
	}
	// The console permission defaults to true in the database, so "false"
	// has to be written explicitly or a new sub-user would silently get it.
	if !req.CanViewConsole {
		database.DB.Model(&models.ServiceUser{}).Where("id = ?", serviceUser.ID).Update("can_view_console", false)
	}
	serviceUser.User = target

	// Log activity
	utils.LogActivity(c, service.ID, "user_added", "", "Sub-user added to service", map[string]interface{}{
		"added_user_id": req.UserID,
		"permissions": map[string]bool{
			"console":  req.CanViewConsole,
			"commands": req.CanSendCommands,
			"files":    req.CanManageFiles,
			"startup":  req.CanEditStartup,
			"power":    req.CanControlPower,
			"sftp":     req.CanAccessSFTP,
		},
	})

	c.JSON(http.StatusCreated, serviceUser)
}

// UpdateServiceUser modifies sub-user permissions
func UpdateServiceUser(c *gin.Context) {
	service, ok := getServiceByUUID(c)
	if !ok {
		return
	}
	userID := c.Param("userId")

	var req AddServiceUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var serviceUser models.ServiceUser
	if err := database.DB.Where("service_id = ? AND user_id = ?", service.ID, userID).First(&serviceUser).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Sub-user not found on this service"})
		return
	}

	// Update permissions
	serviceUser.CanViewConsole = req.CanViewConsole
	serviceUser.CanSendCommands = req.CanSendCommands
	serviceUser.CanManageFiles = req.CanManageFiles
	serviceUser.CanEditStartup = req.CanEditStartup
	serviceUser.CanControlPower = req.CanControlPower
	serviceUser.CanAccessSFTP = req.CanAccessSFTP

	// Written as a map: a struct update would skip permissions being set to false.
	if err := database.DB.Model(&models.ServiceUser{}).Where("id = ?", serviceUser.ID).Updates(map[string]interface{}{
		"can_view_console":  req.CanViewConsole,
		"can_send_commands": req.CanSendCommands,
		"can_manage_files":  req.CanManageFiles,
		"can_edit_startup":  req.CanEditStartup,
		"can_control_power": req.CanControlPower,
		"can_access_sftp":   req.CanAccessSFTP,
	}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update user"})
		return
	}

	// Log activity
	utils.LogActivity(c, service.ID, "user_updated", "", "Sub-user permissions updated", map[string]interface{}{
		"updated_user_id": userID,
	})

	c.JSON(http.StatusOK, serviceUser)
}

// RemoveServiceUser removes a sub-user from a service
func RemoveServiceUser(c *gin.Context) {
	service, ok := getServiceByUUID(c)
	if !ok {
		return
	}
	userID := c.Param("userId")

	if err := database.DB.Where("service_id = ? AND user_id = ?", service.ID, userID).Delete(&models.ServiceUser{}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to remove user"})
		return
	}

	// Log activity
	utils.LogActivity(c, service.ID, "user_removed", "", "Sub-user removed from service", map[string]interface{}{
		"removed_user_id": userID,
	})

	c.JSON(http.StatusOK, gin.H{"status": "removed"})
}
