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
	// 1. Get Root "eggs" directory
	// In production, this might be /var/lib/atlas/eggs, or ./eggs locally
	root := "eggs"
	if _, err := os.Stat(root); os.IsNotExist(err) {
		log.Printf("[Seed] Warning: '%s' directory not found. Skipping egg import.", root)
		return
	}

	// 2. Read Top-Level Directories (Root Nests, e.g. "games", "voice")
	topLevels, err := ioutil.ReadDir(root)
	if err != nil {
		log.Fatalf("[Seed] Failed to read eggs directory: %v", err)
	}

	for _, top := range topLevels {
		if !top.IsDir() {
			continue
		}

		// Create/Update Root Nest
		rootNest := ensureNest(top.Name(), "", nil)

		// 3. Read Sub-Directories (Sub Nests, e.g. "gmod", "minecraft")
		subPath := filepath.Join(root, top.Name())
		subLevels, err := ioutil.ReadDir(subPath)
		if err != nil {
			log.Printf("[Seed] Failed to read subdirectory %s: %v", subPath, err)
			continue
		}

		for _, sub := range subLevels {
			if !sub.IsDir() {
				// Files in top-level folder? Maybe generic eggs directly under category
				if filepath.Ext(sub.Name()) == ".json" {
					importEgg(filepath.Join(subPath, sub.Name()), rootNest)
				}
				continue
			}

			// Create/Update Sub Nest
			subNest := ensureNest(sub.Name(), "", &rootNest.ID)

			// 4. Read Files in Sub-Directory (The Eggs)
			eggPath := filepath.Join(subPath, sub.Name())
			eggFiles, err := ioutil.ReadDir(eggPath)
			if err != nil {
				continue
			}

			for _, f := range eggFiles {
				if !f.IsDir() && filepath.Ext(f.Name()) == ".json" {
					importEgg(filepath.Join(eggPath, f.Name()), subNest)
				}
			}
		}
	}
}

func ensureNest(name, description string, parentID *uint) models.Nest {
	var nest models.Nest
	slug := strings.ToLower(name)

	// Try to find by name within parent scope
	query := database.DB.Where("name = ?", name)
	if parentID != nil {
		query = query.Where("parent_id = ?", parentID)
	} else {
		query = query.Where("parent_id IS NULL")
	}

	if err := query.First(&nest).Error; err != nil {
		nest = models.Nest{
			UUID:        uuid.New().String(),
			Name:        name, // Capitalize first letter? Using directory name as is for now
			Code:        slug,
			Description: description,
			ParentID:    parentID,
		}
		database.DB.Create(&nest)
		log.Printf("[Seed] Created Nest: %s", name)
	} else {
		// Update existing? Unnecessary for now unless we add metadata files for nests
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
