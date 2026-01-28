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
	Content string `json:"content"` // JSON string of the egg (Atlas or Pterodactyl format)
}

// GetEggs returns all eggs
func GetEggs(c *gin.Context) {
	var eggs []models.Egg
	database.DB.Preload("Nest.Parent").Preload("Variables").Find(&eggs)
	c.JSON(http.StatusOK, eggs)
}

// GetNests returns all nests
func GetNests(c *gin.Context) {
	var nests []models.Nest
	database.DB.Preload("SubNests").Preload("SubNests.Eggs.Variables").Preload("Eggs.Variables").Find(&nests)
	c.JSON(http.StatusOK, nests)
}

// CreateNest creates a new nest
func CreateNest(c *gin.Context) {
	var nest models.Nest
	if err := c.ShouldBindJSON(&nest); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	nest.UUID = uuid.New().String()
	if err := database.DB.Create(&nest).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create nest"})
		return
	}
	c.JSON(http.StatusCreated, nest)
}

// UpdateNest modifies a nest
func UpdateNest(c *gin.Context) {
	id := c.Param("id")
	var nest models.Nest
	if err := database.DB.First(&nest, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Nest not found"})
		return
	}
	if err := c.ShouldBindJSON(&nest); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	database.DB.Save(&nest)
	c.JSON(http.StatusOK, nest)
}

// DeleteNest removes a nest
func DeleteNest(c *gin.Context) {
	id := c.Param("id")
	database.DB.Delete(&models.Nest{}, id)
	c.JSON(http.StatusOK, gin.H{"status": "deleted"})
}

func ImportEgg(c *gin.Context) {
	var req ImportEggRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Try to parse as Atlas format first
	var atlasJSON struct {
		UUID            string   `json:"uuid"`
		Name            string   `json:"name"`
		Description     string   `json:"description"`
		DockerImages    []string `json:"docker_images"`
		StartupCommand  string   `json:"startup_command"`
		StopCommand     string   `json:"stop_command"`
		ScriptInstall   string   `json:"script_install"`
		ScriptContainer string   `json:"script_container"`
		ScriptEntry     string   `json:"script_entry"`
		Variables       []struct {
			Name         string `json:"name"`
			EnvVariable  string `json:"env_variable"`
			DefaultValue string `json:"default_value"`
			UserEditable bool   `json:"user_editable"`
			UserViewable bool   `json:"user_viewable"`
			Description  string `json:"description"`
		} `json:"variables"`
	}

	if err := json.Unmarshal([]byte(req.Content), &atlasJSON); err == nil && atlasJSON.Name != "" && atlasJSON.StartupCommand != "" {
		// Verify Nest
		var nest models.Nest
		if err := database.DB.First(&nest, req.NestID).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Nest not found"})
			return
		}

		dockerImagesJSON, _ := json.Marshal(atlasJSON.DockerImages)
		egg := models.Egg{
			UUID:            uuid.New().String(),
			NestID:          nest.ID,
			Name:            atlasJSON.Name,
			Description:     atlasJSON.Description,
			DockerImages:    string(dockerImagesJSON),
			StartupCommand:  atlasJSON.StartupCommand,
			StopCommand:     atlasJSON.StopCommand,
			ScriptInstall:   atlasJSON.ScriptInstall,
			ScriptContainer: atlasJSON.ScriptContainer,
			ScriptEntry:     atlasJSON.ScriptEntry,
		}

		if err := database.DB.Create(&egg).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create egg"})
			return
		}

		for _, v := range atlasJSON.Variables {
			eggVar := models.EggVariable{
				EggID:               egg.ID,
				Name:                v.Name,
				EnvironmentVariable: v.EnvVariable,
				DefaultValue:        v.DefaultValue,
				UserEditable:        v.UserEditable,
				UserViewable:        v.UserViewable,
				Description:         v.Description,
			}
			database.DB.Create(&eggVar)
		}

		c.JSON(http.StatusCreated, egg)
		return
	}

	// Fallback to Pterodactyl format
	var ptEgg struct {
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
		} `json:"variables"`
	}

	if err := json.Unmarshal([]byte(req.Content), &ptEgg); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid egg format"})
		return
	}

	// Convert Pterodactyl to Atlas... (Legacy logic)
	var nest models.Nest
	database.DB.First(&nest, req.NestID)

	var images []string
	for _, img := range ptEgg.DockerImages {
		images = append(images, img)
	}
	imagesJSON, _ := json.Marshal(images)

	egg := models.Egg{
		UUID:            uuid.New().String(),
		NestID:          nest.ID,
		Name:            ptEgg.Name,
		Description:     ptEgg.Description,
		DockerImages:    string(imagesJSON),
		StartupCommand:  ptEgg.Startup,
		StopCommand:     ptEgg.Config.Stop,
		ScriptInstall:   ptEgg.Scripts.Installation.Script,
		ScriptContainer: ptEgg.Scripts.Installation.Container,
		ScriptEntry:     ptEgg.Scripts.Installation.Entrypoint,
	}

	database.DB.Create(&egg)

	for _, v := range ptEgg.Variables {
		database.DB.Create(&models.EggVariable{
			EggID:               egg.ID,
			Name:                v.Name,
			Description:         v.Description,
			EnvironmentVariable: v.EnvVariable,
			DefaultValue:        v.DefaultValue,
			UserViewable:        v.UserViewable,
			UserEditable:        v.UserEditable,
		})
	}

	c.JSON(http.StatusCreated, egg)
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
