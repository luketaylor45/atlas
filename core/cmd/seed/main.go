package main

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"github.com/luketaylor45/atlas/core/internal/config"
	"github.com/luketaylor45/atlas/core/internal/database"
	"github.com/luketaylor45/atlas/core/internal/models"
)

// Simplified Nest Structure for JSON Import
type JSONNest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// Simplified Egg Structure for JSON Import
type JSONEgg struct {
	UUID            string       `json:"uuid"`
	Name            string       `json:"name"`
	Description     string       `json:"description"`
	DockerImages    []string     `json:"docker_images"`
	StartupCommand  string       `json:"startup_command"`
	StopCommand     string       `json:"stop_command"`
	ScriptInstall   string       `json:"script_install"`
	ScriptContainer string       `json:"script_container"`
	ScriptEntry     string       `json:"script_entry"`
	Variables       []JSONEggVar `json:"variables"`
}

type JSONEggVar struct {
	Name         string `json:"name"`
	EnvVariable  string `json:"env_variable"`
	DefaultValue string `json:"default_value"`
	UserEditable bool   `json:"user_editable"`
	UserViewable bool   `json:"user_viewable"`
	InputType    string `json:"input_type"`
	Description  string `json:"description"`
}

func main() {
	log.Println("[Seed] Starting Atlas Infrastructure Seeder...")

	// Load Config & Database
	config.LoadConfig()
	database.Connect()

	// Ensure Migrations Run (incase seed runs before core)
	database.DB.AutoMigrate(&models.Nest{}, &models.Egg{}, &models.EggVariable{})

	scanEggsDirectory()

	log.Println("[Seed] Seeding complete.")
}

func scanEggsDirectory() {
	root := "eggs"
	if _, err := os.Stat(root); os.IsNotExist(err) {
		root = "Eggs"
		if _, err := os.Stat(root); os.IsNotExist(err) {
			log.Printf("[Seed] CRITICAL: Neither 'eggs' nor 'Eggs' directory found.")
			return
		}
	}

	log.Printf("[Seed] Scanning directory: %s", root)
	topLevels, _ := ioutil.ReadDir(root)

	foundCount := 0
	for _, top := range topLevels {
		if !top.IsDir() {
			continue
		}

		// Try to load Category Metadata
		catName := top.Name()
		catDesc := "Main category managed via file system."
		if data, err := ioutil.ReadFile(filepath.Join(root, top.Name(), "nest.json")); err == nil {
			var jn JSONNest
			if err := json.Unmarshal(data, &jn); err == nil {
				catName = jn.Name
				catDesc = jn.Description
			}
		}

		rootNest := ensureNest(catName, top.Name(), catDesc, nil)

		subPath := filepath.Join(root, top.Name())
		subLevels, _ := ioutil.ReadDir(subPath)

		for _, sub := range subLevels {
			if !sub.IsDir() {
				if filepath.Ext(sub.Name()) == ".json" && sub.Name() != "nest.json" {
					importEgg(filepath.Join(subPath, sub.Name()), rootNest)
					foundCount++
				}
				continue
			}

			// Try to load Sub-Category Metadata
			subName := sub.Name()
			subDesc := "Sub-category managed via file system."
			if data, err := ioutil.ReadFile(filepath.Join(subPath, sub.Name(), "nest.json")); err == nil {
				var jn JSONNest
				if err := json.Unmarshal(data, &jn); err == nil {
					subName = jn.Name
					subDesc = jn.Description
				}
			}

			subNest := ensureNest(subName, sub.Name(), subDesc, &rootNest.ID)

			eggPath := filepath.Join(subPath, sub.Name())
			eggFiles, _ := ioutil.ReadDir(eggPath)
			for _, f := range eggFiles {
				if !f.IsDir() && filepath.Ext(f.Name()) == ".json" && f.Name() != "nest.json" {
					importEgg(filepath.Join(eggPath, f.Name()), subNest)
					foundCount++
				}
			}
		}
	}
	log.Printf("[Seed] Seeding complete. Processed %d eggs.", foundCount)
}

func ensureNest(prettyName, dirName, description string, parentID *uint) models.Nest {
	var nest models.Nest
	slug := strings.ToLower(dirName)

	// In the panel, users might rename 'games' to 'Game Servers'.
	// We lookup by the slug (directory name) so we don't create duplicates.
	query := database.DB.Where("code = ?", slug)
	if parentID != nil {
		query = query.Where("parent_id = ?", parentID)
	} else {
		query = query.Where("parent_id IS NULL")
	}

	// Format pretty name if it's just the directory name
	if prettyName == dirName {
		prettyName = strings.Title(strings.ReplaceAll(dirName, "_", " "))
	}

	if err := query.First(&nest).Error; err != nil {
		nest = models.Nest{
			UUID:        uuid.New().String(),
			Name:        prettyName,
			Code:        slug,
			Description: description,
			ParentID:    parentID,
		}
		if err := database.DB.Create(&nest).Error; err != nil {
			log.Printf("[Seed] Failed to create nest %s: %v", prettyName, err)
		} else {
			log.Printf("[Seed] Created Nest: %s (%s)", prettyName, slug)
		}
	}
	return nest
}

func importEgg(path string, nest models.Nest) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		log.Printf("[Seed] Failed to read egg file %s: %v", path, err)
		return
	}

	var jsonEgg JSONEgg
	if err := json.Unmarshal(data, &jsonEgg); err != nil {
		log.Printf("[Seed] Failed to parse egg JSON %s: %v", path, err)
		return
	}

	if jsonEgg.UUID == "" {
		log.Printf("[Seed] Skipping %s: Missing UUID", jsonEgg.Name)
		return
	}

	dockerImagesJSON, _ := json.Marshal(jsonEgg.DockerImages)

	egg := models.Egg{
		UUID:            jsonEgg.UUID,
		NestID:          nest.ID,
		Name:            jsonEgg.Name,
		Description:     jsonEgg.Description,
		DockerImages:    string(dockerImagesJSON),
		StartupCommand:  jsonEgg.StartupCommand,
		StopCommand:     jsonEgg.StopCommand,
		ScriptInstall:   jsonEgg.ScriptInstall,
		ScriptContainer: jsonEgg.ScriptContainer,
		ScriptEntry:     jsonEgg.ScriptEntry,
	}

	var existing models.Egg
	if err := database.DB.Where("uuid = ?", egg.UUID).First(&existing).Error; err != nil {
		database.DB.Create(&egg)
		log.Printf("[Seed] Imported Egg: %s -> %s", nest.Name, egg.Name)
		existing = egg
	} else {
		// Always update the definition from file
		egg.ID = existing.ID
		database.DB.Save(&egg)
		existing = egg
	}

	// Sync Variables
	for _, v := range jsonEgg.Variables {
		inputType := v.InputType
		if inputType == "" {
			inputType = "text"
		}

		eggVar := models.EggVariable{
			EggID:               existing.ID,
			Name:                v.Name,
			EnvironmentVariable: v.EnvVariable,
			DefaultValue:        v.DefaultValue,
			UserEditable:        v.UserEditable,
			UserViewable:        v.UserViewable,
			InputType:           inputType,
			Description:         v.Description,
		}

		var existingVar models.EggVariable
		if err := database.DB.Where("egg_id = ? AND environment_variable = ?", existing.ID, eggVar.EnvironmentVariable).First(&existingVar).Error; err != nil {
			database.DB.Create(&eggVar)
		} else {
			eggVar.ID = existingVar.ID
			database.DB.Save(&eggVar)
		}
	}
}
