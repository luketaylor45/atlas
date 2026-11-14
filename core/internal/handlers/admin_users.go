package handlers

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/luketaylor45/atlas/core/internal/database"
	"github.com/luketaylor45/atlas/core/internal/models"
	"github.com/luketaylor45/atlas/core/internal/utils"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

const (
	minPasswordLength = 8
	// bcrypt only reads the first 72 bytes; longer input is refused rather
	// than silently truncated.
	maxPasswordLength = 72
	minUsernameLength = 3
	maxUsernameLength = 32
)

// hashPassword validates and hashes a password.
func hashPassword(password string) (string, error) {
	if len(password) < minPasswordLength {
		return "", fmt.Errorf("password must be at least %d characters", minPasswordLength)
	}
	if len(password) > maxPasswordLength {
		return "", fmt.Errorf("password must be at most %d characters", maxPasswordLength)
	}
	hashed, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", errors.New("failed to secure the password")
	}
	return string(hashed), nil
}

// normalizeUsername trims and validates a username. SFTP logins are written
// "<service>.<username>", and deleted accounts are parked under a "#" name,
// so only a conservative character set is allowed.
func normalizeUsername(username string) (string, error) {
	username = strings.TrimSpace(username)
	if len(username) < minUsernameLength || len(username) > maxUsernameLength {
		return "", fmt.Errorf("username must be between %d and %d characters", minUsernameLength, maxUsernameLength)
	}
	for _, char := range username {
		switch {
		case char >= 'a' && char <= 'z', char >= 'A' && char <= 'Z', char >= '0' && char <= '9', char == '_', char == '-', char == '.':
		default:
			return "", errors.New("username may only contain letters, numbers, dots, dashes and underscores")
		}
	}
	return username, nil
}

func usernameTaken(tx *gorm.DB, username string, exceptID uint) bool {
	var count int64
	tx.Model(&models.User{}).Where("LOWER(username) = LOWER(?) AND id != ?", username, exceptID).Count(&count)
	return count > 0
}

// otherAdminExists reports whether an admin besides the given user remains.
func otherAdminExists(tx *gorm.DB, exceptID uint) bool {
	var count int64
	tx.Model(&models.User{}).Where("is_admin = ? AND id != ?", true, exceptID).Count(&count)
	return count > 0
}

// GetUsers returns all users in the system
func GetUsers(c *gin.Context) {
	var users []models.User
	if err := database.DB.Order("id").Find(&users).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch users"})
		return
	}
	c.JSON(http.StatusOK, users)
}

// CreateUser handles administrative user creation
func CreateUser(c *gin.Context) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
		IsAdmin  bool   `json:"is_admin"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	username, err := normalizeUsername(req.Username)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	hashed, err := hashPassword(req.Password)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if usernameTaken(database.DB, username, 0) {
		c.JSON(http.StatusConflict, gin.H{"error": "Username is already taken"})
		return
	}

	user := models.User{Username: username, Password: hashed, IsAdmin: req.IsAdmin}
	if err := database.DB.Create(&user).Error; err != nil {
		if isUniqueViolation(err) {
			c.JSON(http.StatusConflict, gin.H{"error": "Username is already taken"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user"})
		return
	}

	utils.LogActivity(c, 0, "create", "user", fmt.Sprintf("Created user: %s", user.Username), nil)
	c.JSON(http.StatusCreated, user)
}

// UpdateUser modifies an existing user
func UpdateUser(c *gin.Context) {
	var user models.User
	if err := database.DB.First(&user, c.Param("id")).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
		IsAdmin  *bool  `json:"is_admin"` // Pointer to distinguish between false and unset
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	updates := map[string]interface{}{}

	if req.Username != "" && req.Username != user.Username {
		username, err := normalizeUsername(req.Username)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if usernameTaken(database.DB, username, user.ID) {
			c.JSON(http.StatusConflict, gin.H{"error": "Username is already taken"})
			return
		}
		updates["username"] = username
	}
	if req.Password != "" {
		hashed, err := hashPassword(req.Password)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		updates["password"] = hashed
	}
	if req.IsAdmin != nil && *req.IsAdmin != user.IsAdmin {
		if !*req.IsAdmin && !otherAdminExists(database.DB, user.ID) {
			c.JSON(http.StatusConflict, gin.H{"error": "This is the only administrator. Make another user an administrator first."})
			return
		}
		updates["is_admin"] = *req.IsAdmin
	}

	if len(updates) > 0 {
		if err := database.DB.Model(&models.User{}).Where("id = ?", user.ID).Updates(updates).Error; err != nil {
			if isUniqueViolation(err) {
				c.JSON(http.StatusConflict, gin.H{"error": "Username is already taken"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update user"})
			return
		}
		database.DB.First(&user, user.ID)
		utils.LogActivity(c, 0, "update", "user", fmt.Sprintf("Updated user: %s", user.Username), nil)
	}

	c.JSON(http.StatusOK, user)
}

// DeleteUser removes a user from the system
func DeleteUser(c *gin.Context) {
	var user models.User
	if err := database.DB.First(&user, c.Param("id")).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	if user.ID == c.MustGet("user_id").(uint) {
		c.JSON(http.StatusConflict, gin.H{"error": "You cannot delete your own account"})
		return
	}
	if user.IsAdmin && !otherAdminExists(database.DB, user.ID) {
		c.JSON(http.StatusConflict, gin.H{"error": "This is the only administrator and cannot be deleted"})
		return
	}

	var serviceCount int64
	database.DB.Model(&models.Service{}).Where("user_id = ?", user.ID).Count(&serviceCount)
	if serviceCount > 0 {
		c.JSON(http.StatusConflict, gin.H{"error": "This user owns servers. Delete or reassign them first."})
		return
	}

	err := database.DB.Transaction(func(tx *gorm.DB) error {
		// Access the user was given to other people's servers goes with them.
		if err := tx.Where("user_id = ?", user.ID).Delete(&models.ServiceUser{}).Error; err != nil {
			return err
		}
		// The username is released so it can be registered again; the row is
		// kept (soft-deleted) because activity logs refer to it.
		if err := tx.Model(&models.User{}).Where("id = ?", user.ID).
			Update("username", database.DeletedUsername(user.Username, user.ID)).Error; err != nil {
			return err
		}
		return tx.Delete(&models.User{}, user.ID).Error
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete user"})
		return
	}

	utils.LogActivity(c, 0, "delete", "user", fmt.Sprintf("Deleted user: %s", user.Username), nil)
	c.JSON(http.StatusOK, gin.H{"status": "deleted"})
}
