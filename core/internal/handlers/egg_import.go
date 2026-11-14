package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/luketaylor45/atlas/core/internal/database"
	"github.com/luketaylor45/atlas/core/internal/eggfile"
	"github.com/luketaylor45/atlas/core/internal/models"
	"github.com/luketaylor45/atlas/core/internal/utils"
	"gorm.io/gorm"
)

type ImportEggRequest struct {
	NestID  uint   `json:"nest_id"`
	Content string `json:"content"` // JSON string of the egg (Atlas or Pterodactyl format)
}

// parseEggImport turns an Atlas or Pterodactyl egg file into an egg and its
// variables, without touching the database.
func parseEggImport(content string) (*models.Egg, []models.EggVariable, []string, error) {
	parsed, err := eggfile.Parse([]byte(content))
	if err != nil {
		return nil, nil, nil, err
	}
	notes := parsed.Notes
	if len(parsed.ScriptFiles) > 0 {
		// Script files only exist next to the catalog on the server's disk.
		notes = append(notes, "This egg refers to install script files, which only catalog eggs can use. Its install script may be incomplete.")
	}
	return &parsed.Egg, parsed.Variables, notes, nil
}

// ImportEgg adds an egg from an Atlas or Pterodactyl egg file.
func ImportEgg(c *gin.Context) {
	var req ImportEggRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	var nest models.Nest
	if err := database.DB.First(&nest, req.NestID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Choose a nest to import the egg into"})
		return
	}

	egg, variables, notes, err := parseEggImport(req.Content)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	egg.UUID = uuid.New().String()
	egg.NestID = nest.ID
	egg.Available = true
	egg.Source = "import"

	// The egg and its variables are created together or not at all.
	err = database.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(egg).Error; err != nil {
			return err
		}
		for i := range variables {
			variables[i].EggID = egg.ID
			viewable, editable := variables[i].UserViewable, variables[i].UserEditable
			if err := tx.Create(&variables[i]).Error; err != nil {
				return err
			}
			// Written separately: a false value would otherwise be replaced
			// by the column default on insert.
			if err := tx.Model(&models.EggVariable{}).Where("id = ?", variables[i].ID).Updates(map[string]interface{}{
				"user_viewable": viewable,
				"user_editable": editable,
			}).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to import the egg"})
		return
	}

	utils.LogActivity(c, 0, "create", "egg", "Imported egg: "+egg.Name, nil)

	database.DB.Preload("Variables").First(egg, egg.ID)
	// The egg's own fields stay at the top level, as before; the notes ride along.
	c.JSON(http.StatusCreated, importedEgg{Egg: egg, Notes: notes})
}

// importedEgg is an egg plus what the importer adapted or left out.
type importedEgg struct {
	*models.Egg
	Notes []string `json:"import_notes"`
}
