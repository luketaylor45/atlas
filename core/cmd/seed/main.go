package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/google/uuid"
	"github.com/luketaylor45/atlas/core/internal/config"
	"github.com/luketaylor45/atlas/core/internal/database"
	"github.com/luketaylor45/atlas/core/internal/eggfile"
	"github.com/luketaylor45/atlas/core/internal/models"
)

const catalogSource = "catalog"

// JSONNest is the nest.json metadata file.
type JSONNest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

func main() {
	log.Println("[Seed] Starting Atlas catalog seeder...")

	config.LoadConfig()
	database.Connect()

	if err := database.Migrate(); err != nil {
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
			eggUUID, ok := importEgg(root, filepath.Join(dir, entry.Name()), nest)
			if !ok {
				parseFailures++
				continue
			}
			present[eggUUID] = true
		}
	}

	for _, top := range topLevels {
		// Directories starting with "_" hold shared files, not eggs.
		if !top.IsDir() || strings.HasPrefix(top.Name(), "_") {
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
			if !sub.IsDir() || strings.HasPrefix(sub.Name(), "_") {
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
	} else if len(present) == 0 {
		// An empty catalog is far more likely to be a missing mount than a
		// decision to drop every game.
		log.Printf("[Seed] The catalog holds no eggs; nothing was retired.")
	} else {
		retireRemovedEggs(present)
		pruneCatalog(seededNests)
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
// removed here, because existing servers may still depend on them.
func retireRemovedEggs(present map[string]bool) {
	var eggs []models.Egg
	database.DB.Where("available = ?", true).Find(&eggs)
	for _, egg := range eggs {
		if !isCatalogEgg(&egg) || present[egg.UUID] {
			continue
		}
		database.DB.Model(&models.Egg{}).Where("id = ?", egg.ID).Update("available", false)
		log.Printf("[Seed] Retired egg no longer in the catalog: %s", egg.Name)
	}
}

// pruneCatalog removes what the catalog left behind: retired catalog eggs
// that no server uses, and catalog nests whose directory is gone and that
// hold nothing any more. Anything still in use is kept.
func pruneCatalog(seededNests []uint) {
	var retired []models.Egg
	database.DB.Where("available = ?", false).Find(&retired)
	for _, egg := range retired {
		if !isCatalogEgg(&egg) {
			continue
		}
		var inUse int64
		database.DB.Unscoped().Model(&models.Service{}).Where("egg_id = ?", egg.ID).Count(&inUse)
		if inUse > 0 {
			continue
		}
		database.DB.Where("egg_id = ?", egg.ID).Delete(&models.EggVariable{})
		if err := database.DB.Delete(&models.Egg{}, egg.ID).Error; err != nil {
			log.Printf("[Seed] Could not remove retired egg %s: %v", egg.Name, err)
			continue
		}
		log.Printf("[Seed] Removed retired egg: %s", egg.Name)
	}

	// Catalog nests carry the name of their directory as their code; nests
	// made in the panel have none and are never touched. Sub-nests go first
	// so a parent emptied by this run can go too.
	seeded := make(map[uint]bool, len(seededNests))
	for _, id := range seededNests {
		seeded[id] = true
	}
	var nests []models.Nest
	database.DB.Where("code <> ''").Order("parent_id IS NULL, id").Find(&nests)
	for _, nest := range nests {
		if seeded[nest.ID] {
			continue
		}
		var eggs, children int64
		database.DB.Model(&models.Egg{}).Where("nest_id = ?", nest.ID).Count(&eggs)
		database.DB.Model(&models.Nest{}).Where("parent_id = ?", nest.ID).Count(&children)
		if eggs > 0 || children > 0 {
			continue
		}
		if err := database.DB.Delete(&models.Nest{}, nest.ID).Error; err != nil {
			log.Printf("[Seed] Could not remove empty nest %s: %v", nest.Name, err)
			continue
		}
		log.Printf("[Seed] Removed empty catalog nest: %s", nest.Name)
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
func importEgg(root string, path string, nest models.Nest) (string, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		log.Printf("[Seed] Failed to read egg file %s: %v", path, err)
		return "", false
	}

	parsed, err := eggfile.Parse(data)
	if err != nil {
		log.Printf("[Seed] Failed to read egg %s: %v", path, err)
		return "", false
	}
	if parsed.UUID == "" {
		log.Printf("[Seed] Skipping %s: catalog eggs need a uuid", path)
		return "", false
	}

	// Shared install scripts are joined in front of the egg's own script.
	// They are part of the egg, so they are part of its hash too.
	hasher := sha256.New()
	hasher.Write(data)
	var script strings.Builder
	for _, name := range parsed.ScriptFiles {
		content, err := readCatalogScript(root, name)
		if err != nil {
			log.Printf("[Seed] Skipping %s: %v", path, err)
			return "", false
		}
		hasher.Write(content)
		script.Write(content)
		if !strings.HasSuffix(script.String(), "\n") {
			script.WriteString("\n")
		}
	}
	script.WriteString(parsed.Egg.ScriptInstall)
	hash := hex.EncodeToString(hasher.Sum(nil))

	egg := parsed.Egg
	egg.ScriptInstall = script.String()
	if egg.Author == "" {
		egg.Author = "Atlas"
	}

	fields := map[string]interface{}{
		"nest_id":          nest.ID,
		"available":        true,
		"author":           egg.Author,
		"name":             egg.Name,
		"description":      egg.Description,
		"docker_images":    egg.DockerImages,
		"startup_command":  egg.StartupCommand,
		"stop_command":     egg.StopCommand,
		"startup_done":     egg.StartupDone,
		"config":           egg.ConfigFiles,
		"script_install":   egg.ScriptInstall,
		"script_container": egg.ScriptContainer,
		"script_entry":     egg.ScriptEntry,
		"ports":            egg.Ports,
		"source":           catalogSource,
		"seed_hash":        hash,
	}

	var existing models.Egg
	err = database.DB.Where("uuid = ?", parsed.UUID).First(&existing).Error
	switch {
	case err != nil:
		egg.UUID = parsed.UUID
		egg.NestID = nest.ID
		egg.Available = true
		egg.Source = catalogSource
		egg.SeedHash = hash
		if err := database.DB.Create(&egg).Error; err != nil {
			log.Printf("[Seed] Failed to create egg %s: %v", egg.Name, err)
			return "", false
		}
		syncVariables(egg.ID, parsed.Variables)
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
		if err := database.DB.Model(&models.Egg{}).Where("id = ?", existing.ID).Updates(fields).Error; err != nil {
			log.Printf("[Seed] Failed to update egg %s: %v", egg.Name, err)
			return "", false
		}
		syncVariables(existing.ID, parsed.Variables)
		log.Printf("[Seed] Updated egg from catalog: %s", egg.Name)
	}

	return parsed.UUID, true
}

// readCatalogScript reads an install script named by an egg. The name is
// relative to the catalog root and may not lead outside it.
func readCatalogScript(root string, name string) ([]byte, error) {
	clean := filepath.Clean(filepath.FromSlash(name))
	if clean == "." || filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return nil, fmt.Errorf("install script %q is outside the catalog", name)
	}
	content, err := os.ReadFile(filepath.Join(root, clean))
	if err != nil {
		return nil, fmt.Errorf("install script %q could not be read: %w", name, err)
	}
	// Scripts edited on Windows must not reach a Linux shell with CRLF endings.
	return []byte(strings.ReplaceAll(strings.TrimPrefix(string(content), "\ufeff"), "\r\n", "\n")), nil
}

// syncVariables makes an egg's variables match its catalog file: new ones are
// added, changed ones updated and ones no longer listed removed.
func syncVariables(eggID uint, variables []models.EggVariable) {
	keep := make([]string, 0, len(variables))
	for _, variable := range variables {
		keep = append(keep, variable.EnvironmentVariable)

		fields := map[string]interface{}{
			"name":          variable.Name,
			"description":   variable.Description,
			"default_value": variable.DefaultValue,
			"user_editable": variable.UserEditable,
			"user_viewable": variable.UserViewable,
			"rules":         variable.Rules,
			"input_type":    variable.InputType,
		}

		var existing models.EggVariable
		if err := database.DB.Where("egg_id = ? AND environment_variable = ?", eggID, variable.EnvironmentVariable).First(&existing).Error; err != nil {
			created := variable
			created.ID = 0
			created.EggID = eggID
			if err := database.DB.Create(&created).Error; err != nil {
				log.Printf("[Seed] Failed to create variable %s: %v", variable.EnvironmentVariable, err)
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
