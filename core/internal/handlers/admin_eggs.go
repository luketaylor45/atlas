package handlers

import (
	"encoding/json"
	"fmt"
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

	visible := make([]models.Nest, 0, len(nests))
	for _, nest := range nests {
		subNests := make([]models.Nest, 0, len(nest.SubNests))
		for _, subNest := range nest.SubNests {
			if len(subNest.Eggs) > 0 {
				subNests = append(subNests, subNest)
			}
		}
		nest.SubNests = subNests

		if len(nest.Eggs) > 0 || len(nest.SubNests) > 0 {
			visible = append(visible, nest)
		}
	}

	c.JSON(http.StatusOK, visible)
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
			Available:       true,
			Source:          "import",
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
		Available:       true,
		Source:          "import",
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
