package handlers

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/luketaylor45/atlas/core/internal/config"
	"github.com/luketaylor45/atlas/core/internal/database"
	"github.com/luketaylor45/atlas/core/internal/models"
	"github.com/luketaylor45/atlas/core/internal/utils"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type RegisterRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// A fixed hash compared against when the username does not exist, so a
// failed login takes the same time whether or not the account is real.
var dummyPasswordHash, _ = bcrypt.GenerateFromPassword([]byte("atlas-timing-equaliser"), bcrypt.DefaultCost)

func Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Username and password are required"})
		return
	}

	var user models.User
	if err := database.DB.Where("LOWER(username) = LOWER(?)", req.Username).First(&user).Error; err != nil {
		_ = bcrypt.CompareHashAndPassword(dummyPasswordHash, []byte(req.Password))
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
		return
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
		return
	}

	token, err := utils.GenerateToken(user.ID, user.IsAdmin)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"token": token,
		"user":  user,
	})
}

// Register creates a new regular user account
func Register(c *gin.Context) {
	if !config.AppConfig.RegistrationEnabled {
		c.JSON(http.StatusForbidden, gin.H{"error": "Registration is disabled. Ask an administrator to create your account."})
		return
	}

	var req RegisterRequest
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

	user := models.User{Username: username, Password: hashed, IsAdmin: false}
	if err := database.DB.Create(&user).Error; err != nil {
		if isUniqueViolation(err) {
			c.JSON(http.StatusConflict, gin.H{"error": "Username is already taken"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user"})
		return
	}

	token, err := utils.GenerateToken(user.ID, user.IsAdmin)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Account created, but signing in failed. Please log in."})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "User registered successfully",
		"token":   token,
		"user":    user,
	})
}

const setupCompletedSetting = "setup_completed"

var errSetupCompleted = errors.New("setup already completed")

// InitialSetup creates the first admin user if no users exist
func InitialSetup(c *gin.Context) {
	var req RegisterRequest
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

	user := models.User{Username: username, Password: hashed, IsAdmin: true}
	err = database.DB.Transaction(func(tx *gorm.DB) error {
		var count int64
		if err := tx.Unscoped().Model(&models.User{}).Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			return errSetupCompleted
		}
		// Two simultaneous setup requests both see zero users. Only one can
		// insert this row; the other is turned away instead of creating a
		// second administrator.
		claim := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&models.Setting{Key: setupCompletedSetting, Value: "true"})
		if claim.Error != nil {
			return claim.Error
		}
		if claim.RowsAffected == 0 {
			return errSetupCompleted
		}
		return tx.Create(&user).Error
	})
	if errors.Is(err, errSetupCompleted) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Setup already completed"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create the administrator account"})
		return
	}

	token, err := utils.GenerateToken(user.ID, user.IsAdmin)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Account created, but signing in failed. Please log in."})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "Administrator account created",
		"token":   token,
		"user":    user,
	})
}

// GetSetupStatus checks if the system needs setup
func GetSetupStatus(c *gin.Context) {
	var count int64
	database.DB.Unscoped().Model(&models.User{}).Count(&count)
	c.JSON(http.StatusOK, gin.H{
		"setup_required":       count == 0,
		"registration_enabled": config.AppConfig.RegistrationEnabled,
	})
}

// GetCurrentUser returns the account behind the session. The panel uses it
// to confirm a stored session is still valid and what it is allowed to do,
// instead of trusting what it saved at login.
func GetCurrentUser(c *gin.Context) {
	var user models.User
	if err := database.DB.First(&user, c.MustGet("user_id").(uint)).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Account no longer exists"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"user": user})
}

// ChangeOwnPassword lets a signed-in user change their password.
func ChangeOwnPassword(c *gin.Context) {
	var req struct {
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	var user models.User
	if err := database.DB.First(&user, c.MustGet("user_id").(uint)).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Account no longer exists"})
		return
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.CurrentPassword)); err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "Your current password is incorrect"})
		return
	}
	if req.NewPassword == req.CurrentPassword {
		c.JSON(http.StatusBadRequest, gin.H{"error": "The new password must be different from the current one"})
		return
	}

	hashed, err := hashPassword(req.NewPassword)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := database.DB.Model(&models.User{}).Where("id = ?", user.ID).Update("password", hashed).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to change password"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "password_changed"})
}
