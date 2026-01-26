package handlers

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/luketaylor45/atlas/core/internal/database"
	"github.com/luketaylor45/atlas/core/internal/models"
	"github.com/luketaylor45/atlas/core/internal/utils"
	"golang.org/x/crypto/bcrypt"
)

type SFTPAuthRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// ValidateSFTPCredentials validates SFTP login credentials
func ValidateSFTPCredentials(c *gin.Context) {
	var req SFTPAuthRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"valid": false})
		return
	}

	// Pattern: service_short_id.username
	parts := strings.Split(req.Username, ".")
	if len(parts) < 2 {
		c.JSON(http.StatusOK, gin.H{"valid": false})
		return
	}

	serviceIDPrefix := parts[0]
	actualUsername := strings.Join(parts[1:], ".")

	// Find user by username
	var user models.User
	if err := database.DB.Where("username = ?", actualUsername).First(&user).Error; err != nil {
		c.JSON(http.StatusOK, gin.H{"valid": false})
		return
	}

	// Validate user's account password
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		c.JSON(http.StatusOK, gin.H{"valid": false})
		return
	}

	// Find the targeted service
	var service models.Service
	if err := database.DB.Where("uuid LIKE ?", serviceIDPrefix+"%").First(&service).Error; err != nil {
		c.JSON(http.StatusOK, gin.H{"valid": false})
		return
	}

	// Check access
	hasAccess := false
	accessLevel := ""

	if user.IsAdmin {
		hasAccess = true
		accessLevel = "Admin"
	} else if service.UserID == user.ID {
		hasAccess = true
		accessLevel = "Owner"
	} else {
		// Check sub-user access
		var su models.ServiceUser
		if err := database.DB.Where("service_id = ? AND user_id = ? AND can_access_sftp = ?", service.ID, user.ID, true).First(&su).Error; err == nil {
			hasAccess = true
			accessLevel = "Sub-user"
		}
	}

	if hasAccess {
		utils.LogActivityDirect(service.ID, user.ID, "sftp_login", "SFTP", fmt.Sprintf("SFTP connection established (%s)", accessLevel), c.ClientIP())
		c.JSON(http.StatusOK, gin.H{
			"valid":        true,
			"service_uuid": service.UUID,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{"valid": false})
}
