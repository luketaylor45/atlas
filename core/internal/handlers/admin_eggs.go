package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/luketaylor45/atlas/core/internal/database"
	"github.com/luketaylor45/atlas/core/internal/models"
)

type ImportEggRequest struct {
	NestID  uint   `json:"nest_id"`
	Content string `json:"content"` // JSON string of the Pterodactyl egg
}

type PterodactylEgg struct {
	Name         string            `json:"name"`
	Description  string            `json:"description"`
	DockerImages map[string]string `json:"docker_images"`
	Startup      string            `json:"startup"`
	Config       struct {
		Stop string `json:"stop"`
	} `json:"config"`
	Scripts struct {
		Installation struct {
			Script     string `json:"script"`
			Container  string `json:"container"`
			Entrypoint string `json:"entrypoint"`
		} `json:"installation"`
	} `json:"scripts"`
	Variables []struct {
		Name         string `json:"name"`
		Description  string `json:"description"`
		EnvVariable  string `json:"env_variable"`
		DefaultValue string `json:"default_value"`
		UserViewable bool   `json:"user_viewable"`
		UserEditable bool   `json:"user_editable"`
		Rules        string `json:"rules"`
		FieldType    string `json:"field_type"`
	} `json:"variables"`
}

func ImportEgg(c *gin.Context) {
	var req ImportEggRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var ptEgg PterodactylEgg
	if err := json.Unmarshal([]byte(req.Content), &ptEgg); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid Pterodactyl egg format: " + err.Error()})
		return
	}

	// 1. Verify Nest exists
	var nest models.Nest
	if err := database.DB.First(&nest, req.NestID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Nest not found"})
		return
	}

	// 2. Convert to Atlas Egg
	atlasEgg := models.Egg{
		UUID:              uuid.New().String(),
		NestID:            nest.ID,
		Name:              ptEgg.Name,
		Description:       ptEgg.Description,
		StartupCommand:    ptEgg.Startup,
		StopCommand:       ptEgg.Config.Stop,
		InstallScript:     ptEgg.Scripts.Installation.Script,
		InstallContainer:  ptEgg.Scripts.Installation.Container,
		InstallEntrypoint: ptEgg.Scripts.Installation.Entrypoint,
	}

	// Convert Docker Images map to JSON array of image strings
	var images []string
	for _, img := range ptEgg.DockerImages {
		images = append(images, img)
	}
	imagesJSON, _ := json.Marshal(images)
	atlasEgg.DockerImages = string(imagesJSON)

	if err := database.DB.Create(&atlasEgg).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create egg: " + err.Error()})
		return
	}

	// 3. Import Variables
	for _, v := range ptEgg.Variables {
		inputType := v.FieldType
		if inputType == "" {
			inputType = "text"
		}
		atlasVar := models.EggVariable{
			EggID:               atlasEgg.ID,
			Name:                v.Name,
			Description:         v.Description,
			EnvironmentVariable: v.EnvVariable,
			DefaultValue:        v.DefaultValue,
			UserViewable:        v.UserViewable,
			UserEditable:        v.UserEditable,
			Rules:               v.Rules,
			InputType:           inputType,
		}
		database.DB.Create(&atlasVar)
	}

	c.JSON(http.StatusCreated, atlasEgg)
}

// UpdateEgg modifies an existing egg
func UpdateEgg(c *gin.Context) {
	eggID := c.Param("id")
	var egg models.Egg
	if err := database.DB.Preload("Variables").First(&egg, eggID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Egg not found"})
		return
	}

	if err := c.ShouldBindJSON(&egg); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := database.DB.Save(&egg).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update egg"})
		return
	}

	c.JSON(http.StatusOK, egg)
}

// DeleteEgg removes an egg
func DeleteEgg(c *gin.Context) {
	eggID := c.Param("id")
	if err := database.DB.Delete(&models.Egg{}, eggID).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete egg"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "deleted"})
}
