package handlers

import (
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/luketaylor45/atlas/core/internal/database"
	"github.com/luketaylor45/atlas/core/internal/middleware"
	"github.com/luketaylor45/atlas/core/internal/models"
	"github.com/luketaylor45/atlas/core/internal/utils"
	"golang.org/x/crypto/bcrypt"
)

type SFTPAuthRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// The panel shows SFTP usernames as "<first 8 characters of the service
// UUID>.<username>". Only that exact shape, or a full UUID, is accepted, so
// the value can be used in a prefix match without wildcard injection.
var sftpServicePrefixPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}(-[0-9a-fA-F]{4}){0,3}(-[0-9a-fA-F]{12})?$`)

// ValidateSFTPCredentials validates an SFTP login on behalf of a node.
func ValidateSFTPCredentials(c *gin.Context) {
	node := middleware.NodeFromContext(c)

	var req SFTPAuthRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"valid": false})
		return
	}

	servicePrefix, username, found := strings.Cut(req.Username, ".")
	if !found || username == "" || !sftpServicePrefixPattern.MatchString(servicePrefix) {
		c.JSON(http.StatusOK, gin.H{"valid": false})
		return
	}

	var user models.User
	if err := database.DB.Where("LOWER(username) = LOWER(?)", username).First(&user).Error; err != nil {
		c.JSON(http.StatusOK, gin.H{"valid": false})
		return
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		c.JSON(http.StatusOK, gin.H{"valid": false})
		return
	}

	// The service must live on the node asking, and the prefix must identify
	// exactly one service there.
	var services []models.Service
	if err := database.DB.Where("node_id = ? AND uuid LIKE ?", node.ID, strings.ToLower(servicePrefix)+"%").Limit(2).Find(&services).Error; err != nil || len(services) != 1 {
		c.JSON(http.StatusOK, gin.H{"valid": false})
		return
	}
	service := services[0]

	accessLevel := ""
	switch {
	case user.IsAdmin:
		accessLevel = "Admin"
	case service.UserID == user.ID:
		accessLevel = "Owner"
	default:
		var su models.ServiceUser
		if err := database.DB.Where("service_id = ? AND user_id = ? AND can_access_sftp = ?", service.ID, user.ID, true).First(&su).Error; err == nil {
			accessLevel = "Sub-user"
		}
	}

	if accessLevel == "" {
		c.JSON(http.StatusOK, gin.H{"valid": false})
		return
	}

	utils.LogActivityDirect(service.ID, user.ID, "sftp_login", "SFTP", fmt.Sprintf("SFTP connection established (%s)", accessLevel), c.ClientIP())
	c.JSON(http.StatusOK, gin.H{
		"valid":        true,
		"service_uuid": service.UUID,
	})
}
