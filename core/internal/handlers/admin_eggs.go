package handlers

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/luketaylor45/atlas/core/internal/database"
	"github.com/luketaylor45/atlas/core/internal/models"
)

// GetEggs returns all eggs
func GetEggs(c *gin.Context) {
	var eggs []models.Egg
	database.DB.Where("available = ?", true).Preload("Nest.Parent").Preload("Variables").Find(&eggs)
	c.JSON(http.StatusOK, eggs)
}

// GetNests returns all nests
func GetNests(c *gin.Context) {
	var nests []models.Nest
	database.DB.
		Preload("SubNests").
		Preload("SubNests.Eggs", "available = ?", true).
		Preload("SubNests.Eggs.Variables").
		Preload("Eggs", "available = ?", true).
		Preload("Eggs.Variables").
		Find(&nests)

	// Every nest is listed, with its sub-nests also nested inside it, which
	// is the shape the panel already reads. Empty nests are included:
	// otherwise a newly created nest could never be chosen to import an egg into.
	c.JSON(http.StatusOK, nests)
}

// NestRequest carries the editable fields of a nest.
type NestRequest struct {
	Name        *string `json:"name"`
	Description *string `json:"description"`
	ParentID    *uint   `json:"parent_id"`
}

// CreateNest creates a new nest
func CreateNest(c *gin.Context) {
	var req NestRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}
	if req.Name == nil || strings.TrimSpace(*req.Name) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "A nest name is required"})
		return
	}

	nest := models.Nest{UUID: uuid.New().String(), Name: strings.TrimSpace(*req.Name)}
	if req.Description != nil {
		nest.Description = *req.Description
	}
	if req.ParentID != nil && *req.ParentID != 0 {
		var parent models.Nest
		if err := database.DB.First(&parent, *req.ParentID).Error; err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "The parent nest does not exist"})
			return
		}
		if parent.ParentID != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Nests can only be nested one level deep"})
			return
		}
		nest.ParentID = &parent.ID
	}

	if err := database.DB.Create(&nest).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create nest"})
		return
	}
	c.JSON(http.StatusCreated, nest)
}

// UpdateNest renames or redescribes a nest
func UpdateNest(c *gin.Context) {
	var nest models.Nest
	if err := database.DB.First(&nest, c.Param("id")).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Nest not found"})
		return
	}

	var req NestRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	updates := map[string]interface{}{}
	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if name == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "A nest name is required"})
			return
		}
		updates["name"] = name
	}
	if req.Description != nil {
		updates["description"] = *req.Description
	}
	if len(updates) > 0 {
		if err := database.DB.Model(&models.Nest{}).Where("id = ?", nest.ID).Updates(updates).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update nest"})
			return
		}
		database.DB.First(&nest, nest.ID)
	}
	c.JSON(http.StatusOK, nest)
}

// DeleteNest removes a nest and its eggs, unless a server still uses one of them.
func DeleteNest(c *gin.Context) {
	var nest models.Nest
	if err := database.DB.First(&nest, c.Param("id")).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Nest not found"})
		return
	}

	var eggIDs []uint
	database.DB.Model(&models.Egg{}).
		Where("nest_id = ? OR nest_id IN (?)", nest.ID, database.DB.Model(&models.Nest{}).Select("id").Where("parent_id = ?", nest.ID)).
		Pluck("id", &eggIDs)
	if count := eggsInUse(eggIDs); count > 0 {
		c.JSON(http.StatusConflict, gin.H{"error": fmt.Sprintf("Eggs in this nest are used by %d server(s). Delete or move them first.", count)})
		return
	}

	if err := database.DB.Delete(&models.Nest{}, nest.ID).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete nest"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "deleted"})
}

// UpdateEgg modifies an existing egg
func UpdateEgg(c *gin.Context) {
	eggID := c.Param("id")
	var egg models.Egg
	if err := database.DB.First(&egg, eggID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Egg not found"})
		return
	}

	identity := egg
	if err := c.ShouldBindJSON(&egg); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// The request body cannot move the egg to another record or rewrite the
	// fields Atlas tracks itself.
	egg.ID = identity.ID
	egg.UUID = identity.UUID
	egg.Source = identity.Source
	egg.SeedHash = identity.SeedHash
	egg.CreatedAt = identity.CreatedAt
	// Remember the edit so the catalog seeder does not undo it on restart.
	egg.Customized = true
	egg.Variables = nil
	egg.Servers = nil

	if err := database.DB.Omit("Variables", "Servers", "Nest").Save(&egg).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update egg"})
		return
	}

	database.DB.Preload("Variables").First(&egg, egg.ID)
	c.JSON(http.StatusOK, egg)
}

// eggsInUse counts the services that depend on the given eggs.
func eggsInUse(eggIDs []uint) int64 {
	if len(eggIDs) == 0 {
		return 0
	}
	var count int64
	database.DB.Unscoped().Model(&models.Service{}).Where("egg_id IN ?", eggIDs).Count(&count)
	return count
}

// DeleteEgg removes an egg that no service uses.
func DeleteEgg(c *gin.Context) {
	var egg models.Egg
	if err := database.DB.First(&egg, c.Param("id")).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Egg not found"})
		return
	}
	if count := eggsInUse([]uint{egg.ID}); count > 0 {
		c.JSON(http.StatusConflict, gin.H{"error": fmt.Sprintf("This egg is used by %d server(s). Delete or move them first.", count)})
		return
	}

	if err := database.DB.Delete(&models.Egg{}, egg.ID).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete egg"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "deleted"})
}
