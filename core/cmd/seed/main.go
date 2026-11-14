package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/google/uuid"
	"github.com/luketaylor45/atlas/core/internal/config"
	"github.com/luketaylor45/atlas/core/internal/database"
	"github.com/luketaylor45/atlas/core/internal/models"
	"gorm.io/gorm"
)

const catalogSource = "catalog"

// JSONNest is the nest.json metadata file.
type JSONNest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// JSONEgg is an egg file in the catalog.
type JSONEgg struct {
	UUID            string           `json:"uuid"`
	Name            string           `json:"name"`
	Author          string           `json:"author"`
	Description     string           `json:"description"`
	DockerImages    []string         `json:"docker_images"`
	StartupCommand  string           `json:"startup_command"`
	StopCommand     string           `json:"stop_command"`
	ScriptInstall   string           `json:"script_install"`
	ScriptContainer string           `json:"script_container"`
	ScriptEntry     string           `json:"script_entry"`
	Ports           []models.EggPort `json:"ports"`
	Variables       []JSONEggVar     `json:"variables"`
}

// JSONEggVar is one variable of a catalog egg. The visibility flags are
// pointers so that leaving them out means "yes", not "no".
type JSONEggVar struct {
	Name         string `json:"name"`
	EnvVariable  string `json:"env_variable"`
	DefaultValue string `json:"default_value"`
	UserEditable *bool  `json:"user_editable"`
	UserViewable *bool  `json:"user_viewable"`
	InputType    string `json:"input_type"`
	Rules        string `json:"rules"`
	Description  string `json:"description"`
}

func boolOr(value *bool, fallback bool) bool {
	if value == nil {
		return fallback
	}
	return *value
}

func main() {
	log.Println("[Seed] Starting Atlas catalog seeder...")

	config.LoadConfig()
	database.Connect()

	if err := database.DB.AutoMigrate(&models.Nest{}, &models.Egg{}, &models.EggVariable{}); err != nil {
		log.Fatalf("[Seed] Migration failed: %v", err)
	}

	scanEggsDirectory()
	log.Println("[Seed] Seeding complete.")
}

func scanEggsDirectory() {
	root := "eggs"
	if _, err := os.Stat(root); os.IsNotExist(err) {
		root = "Eggs"
		if _, err := os.Stat(root); os.IsNotExist(err) {
			log.Printf("[Seed] No eggs directory found; the catalog was left unchanged.")
			return
		}
	}

	log.Printf("[Seed] Scanning directory: %s", root)
	topLevels, err := os.ReadDir(root)
	if err != nil {
		log.Printf("[Seed] Cannot read %s: %v; the catalog was left unchanged.", root, err)
		return
	}

	// Every catalog egg found on disk. Anything previously seeded that is no
	// longer here is retired at the end.
	present := map[string]bool{}
	var seededNests []uint
	parseFailures := 0

	seedDirectory := func(dir string, nest models.Nest) {
		entries, _ := os.ReadDir(dir)
		for _, entry := range entries {
			if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" || entry.Name() == "nest.json" {
				continue
			}
			eggUUID, ok := importEgg(filepath.Join(dir, entry.Name()), nest)
			if !ok {
				parseFailures++
				continue
			}
			present[eggUUID] = true
		}
	}

	for _, top := range topLevels {
		if !top.IsDir() {
			continue
		}
		topPath := filepath.Join(root, top.Name())
		name, description := readNestMetadata(topPath, top.Name(), "Main category managed via file system.")
		rootNest, ok := ensureNest(name, top.Name(), description, nil)
		if !ok {
			continue
		}
		seededNests = append(seededNests, rootNest.ID)
		seedDirectory(topPath, rootNest)

		subLevels, _ := os.ReadDir(topPath)
		for _, sub := range subLevels {
			if !sub.IsDir() {
				continue
			}
			subPath := filepath.Join(topPath, sub.Name())
			subName, subDescription := readNestMetadata(subPath, sub.Name(), "Sub-category managed via file system.")
			subNest, ok := ensureNest(subName, sub.Name(), subDescription, &rootNest.ID)
			if !ok {
				continue
			}
			seededNests = append(seededNests, subNest.ID)
			seedDirectory(subPath, subNest)
		}
	}

	// A file that failed to parse says nothing about whether its egg was
	// removed, so nothing is retired on a run with failures.
	if parseFailures > 0 {
		log.Printf("[Seed] %d egg file(s) could not be read; no eggs were retired this run.", parseFailures)
	} else {
		retireRemovedEggs(seededNests, present)
	}
	log.Printf("[Seed] Processed %d catalog eggs.", len(present))
}

func readNestMetadata(dir, fallbackName, fallbackDescription string) (string, string) {
	data, err := os.ReadFile(filepath.Join(dir, "nest.json"))
	if err != nil {
		return fallbackName, fallbackDescription
	}
	var metadata JSONNest
	if err := json.Unmarshal(data, &metadata); err != nil || metadata.Name == "" {
		return fallbackName, fallbackDescription
	}
	return metadata.Name, metadata.Description
}

// isCatalogEgg reports whether the seeder owns an egg. Eggs seeded before the
// source column existed are recognised by their "atlas-" identifiers. Eggs
// imported through the panel are never touched by the seeder.
func isCatalogEgg(egg *models.Egg) bool {
	return egg.Source == catalogSource || (egg.Source == "" && strings.HasPrefix(egg.UUID, "atlas-"))
}

// retireRemovedEggs hides catalog eggs whose file was deleted. They are not
// removed, because existing servers may still depend on them.
func retireRemovedEggs(nestIDs []uint, present map[string]bool) {
	if len(nestIDs) == 0 {
		return
	}
	var eggs []models.Egg
	database.DB.Where("nest_id IN ? AND available = ?", nestIDs, true).Find(&eggs)
	for _, egg := range eggs {
		if !isCatalogEgg(&egg) || present[egg.UUID] {
			continue
		}
		database.DB.Model(&models.Egg{}).Where("id = ?", egg.ID).Update("available", false)
		log.Printf("[Seed] Retired egg no longer in the catalog: %s", egg.Name)
	}
}

func ensureNest(prettyName, dirName, description string, parentID *uint) (models.Nest, bool) {
	var nest models.Nest
	slug := strings.ToLower(dirName)

	// Nests are matched by directory name, so renaming one in the panel does
	// not create a duplicate.
	query := database.DB.Where("code = ?", slug)
	if parentID != nil {
		query = query.Where("parent_id = ?", *parentID)
	} else {
		query = query.Where("parent_id IS NULL")
	}

	if prettyName == dirName {
		prettyName = titleCase(strings.ReplaceAll(dirName, "_", " "))
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
			return nest, false
		}
		log.Printf("[Seed] Created nest: %s (%s)", prettyName, slug)
	}
	return nest, true
}

func titleCase(value string) string {
	words := strings.Fields(value)
	for i, word := range words {
		words[i] = strings.ToUpper(word[:1]) + word[1:]
	}
	return strings.Join(words, " ")
}

// importEgg loads one catalog file. It returns the egg's UUID and whether the
// file was understood.
//
// An egg is only written when its file changed since it was last loaded, and
// never when an admin has edited it in the panel: restarting Core must not
// undo an admin's work.
func importEgg(path string, nest models.Nest) (string, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		log.Printf("[Seed] Failed to read egg file %s: %v", path, err)
		return "", false
	}

	var file JSONEgg
	if err := json.Unmarshal(data, &file); err != nil {
		log.Printf("[Seed] Failed to parse egg JSON %s: %v", path, err)
		return "", false
	}
	if file.UUID == "" || file.Name == "" || file.StartupCommand == "" {
		log.Printf("[Seed] Skipping %s: uuid, name and startup_command are required", path)
		return "", false
	}

	sum := sha256.Sum256(data)
	hash := hex.EncodeToString(sum[:])

	imagesJSON, _ := json.Marshal(file.DockerImages)
	portsJSON := ""
	if len(file.Ports) > 0 {
		encoded, _ := json.Marshal(file.Ports)
		portsJSON = string(encoded)
	}
	author := file.Author
	if author == "" {
		author = "Atlas"
	}

	fields := map[string]interface{}{
		"nest_id":          nest.ID,
		"available":        true,
		"author":           author,
		"name":             file.Name,
		"description":      file.Description,
		"docker_images":    string(imagesJSON),
		"startup_command":  file.StartupCommand,
		"stop_command":     file.StopCommand,
		"script_install":   file.ScriptInstall,
		"script_container": file.ScriptContainer,
		"script_entry":     file.ScriptEntry,
		"ports":            portsJSON,
		"source":           catalogSource,
		"seed_hash":        hash,
	}

	var existing models.Egg
	err = database.DB.Where("uuid = ?", file.UUID).First(&existing).Error
	switch {
	case err != nil:
		egg := models.Egg{
			UUID:            file.UUID,
			NestID:          nest.ID,
			Available:       true,
			Author:          author,
			Name:            file.Name,
			Description:     file.Description,
			DockerImages:    string(imagesJSON),
			StartupCommand:  file.StartupCommand,
			StopCommand:     file.StopCommand,
			ScriptInstall:   file.ScriptInstall,
			ScriptContainer: file.ScriptContainer,
			ScriptEntry:     file.ScriptEntry,
			Ports:           portsJSON,
			Source:          catalogSource,
			SeedHash:        hash,
		}
		if err := database.DB.Create(&egg).Error; err != nil {
			log.Printf("[Seed] Failed to create egg %s: %v", file.Name, err)
			return "", false
		}
		syncVariables(egg.ID, file.Variables)
		log.Printf("[Seed] Added egg: %s -> %s", nest.Name, egg.Name)

	case existing.Customized:
		// Keep the admin's version; only make sure it is still offered.
		database.DB.Model(&models.Egg{}).Where("id = ?", existing.ID).Updates(map[string]interface{}{
			"available": true,
			"source":    catalogSource,
		})
		if existing.SeedHash != hash {
			log.Printf("[Seed] %s has a newer catalog version, but it was edited in the panel and was left as it is.", existing.Name)
		}

	case existing.SeedHash == hash && existing.Source == catalogSource:
		// Unchanged file: nothing to write beyond keeping it available.
		if !existing.Available {
			database.DB.Model(&models.Egg{}).Where("id = ?", existing.ID).Update("available", true)
		}

	default:
		err := database.DB.Transaction(func(tx *gorm.DB) error {
			return tx.Model(&models.Egg{}).Where("id = ?", existing.ID).Updates(fields).Error
		})
		if err != nil {
			log.Printf("[Seed] Failed to update egg %s: %v", file.Name, err)
			return "", false
		}
		syncVariables(existing.ID, file.Variables)
		log.Printf("[Seed] Updated egg from catalog: %s", file.Name)
	}

	return file.UUID, true
}

// syncVariables makes an egg's variables match its catalog file: new ones are
// added, changed ones updated and ones no longer listed removed.
func syncVariables(eggID uint, variables []JSONEggVar) {
	keep := make([]string, 0, len(variables))
	for _, variable := range variables {
		if variable.EnvVariable == "" {
			continue
		}
		keep = append(keep, variable.EnvVariable)

		inputType := variable.InputType
		if inputType == "" {
			inputType = "text"
		}
		fields := map[string]interface{}{
			"name":          variable.Name,
			"description":   variable.Description,
			"default_value": variable.DefaultValue,
			"user_editable": boolOr(variable.UserEditable, true),
			"user_viewable": boolOr(variable.UserViewable, true),
			"rules":         variable.Rules,
			"input_type":    inputType,
		}

		var existing models.EggVariable
		if err := database.DB.Where("egg_id = ? AND environment_variable = ?", eggID, variable.EnvVariable).First(&existing).Error; err != nil {
			created := models.EggVariable{
				EggID:               eggID,
				Name:                variable.Name,
				Description:         variable.Description,
				EnvironmentVariable: variable.EnvVariable,
				DefaultValue:        variable.DefaultValue,
				Rules:               variable.Rules,
				InputType:           inputType,
			}
			if err := database.DB.Create(&created).Error; err != nil {
				log.Printf("[Seed] Failed to create variable %s: %v", variable.EnvVariable, err)
				continue
			}
			// Written separately: a false value would otherwise be replaced
			// by the column default on insert.
			database.DB.Model(&models.EggVariable{}).Where("id = ?", created.ID).Updates(fields)
			continue
		}
		database.DB.Model(&models.EggVariable{}).Where("id = ?", existing.ID).Updates(fields)
	}

	sort.Strings(keep)
	stale := database.DB.Where("egg_id = ?", eggID)
	if len(keep) > 0 {
		stale = stale.Where("environment_variable NOT IN ?", keep)
	}
	stale.Delete(&models.EggVariable{})
}
