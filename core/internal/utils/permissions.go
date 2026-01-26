package utils

import (
	"github.com/luketaylor45/atlas/core/internal/database"
	"github.com/luketaylor45/atlas/core/internal/models"
)

// PermissionCheck checks if a user has a specific permission for a service
func PermissionCheck(userID uint, serviceID uint, permission string) (bool, bool) {
	var service models.Service
	// Check if owner
	if err := database.DB.Where("id = ? AND user_id = ?", serviceID, userID).First(&service).Error; err == nil {
		return true, true // Is owner
	}

	// Check if sub-user
	var serviceUser models.ServiceUser
	if err := database.DB.Where("service_id = ? AND user_id = ?", serviceID, userID).First(&serviceUser).Error; err != nil {
		return false, false // No access
	}

	// Dynamic permission check
	switch permission {
	case "can_view_console":
		return serviceUser.CanViewConsole, false
	case "can_send_commands":
		return serviceUser.CanSendCommands, false
	case "can_manage_files":
		return serviceUser.CanManageFiles, false
	case "can_edit_startup":
		return serviceUser.CanEditStartup, false
	case "can_control_power":
		return serviceUser.CanControlPower, false
	case "can_access_sftp":
		return serviceUser.CanAccessSFTP, false
	}

	return false, false
}

// FindServiceForUser finds a service by UUID and checks access
func FindServiceForUser(uuid string, userID uint) (*models.Service, *models.ServiceUser, bool) {
	var user models.User
	if err := database.DB.First(&user, userID).Error; err != nil {
		return nil, nil, false
	}

	var service models.Service
	db := database.DB.Preload("Node").Preload("Egg.Nest").Preload("Egg.Variables")

	if user.IsAdmin {
		// Admins can see any service
		if err := db.Where("uuid = ?", uuid).First(&service).Error; err == nil {
			return &service, nil, true
		}
		return nil, nil, false
	}

	// For standard users: Find service where user is owner OR sub-user
	err := db.Where("uuid = ? AND (user_id = ? OR id IN (SELECT service_id FROM service_users WHERE user_id = ?))", uuid, userID, userID).
		First(&service).Error

	if err != nil || service.ID == 0 {
		return nil, nil, false
	}

	// Check if this user is the owner
	if service.UserID == userID {
		return &service, nil, true
	}

	// It's a sub-user, fetch their permissions
	var serviceUser models.ServiceUser
	database.DB.Where("service_id = ? AND user_id = ?", service.ID, userID).First(&serviceUser)

	return &service, &serviceUser, true
}
