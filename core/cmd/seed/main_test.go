package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/luketaylor45/atlas/core/internal/database"
	"github.com/luketaylor45/atlas/core/internal/eggfile"
	"github.com/luketaylor45/atlas/core/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func testDatabase(t *testing.T) {
	t.Helper()
	dsn := "file:" + filepath.ToSlash(filepath.Join(t.TempDir(), "atlas.db")) + "?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	database.DB = db
	t.Cleanup(func() {
		if sqlDB, err := db.DB(); err == nil {
			sqlDB.Close()
		}
	})
	if err := database.Migrate(); err != nil {
		t.Fatal(err)
	}
}

func writeFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

const testEgg = `{
  "uuid": "atlas-test-game",
  "name": "Test Game",
  "docker_images": ["image:1"],
  "startup_command": "./run {{SERVER_PORT}}",
  "stop_command": "quit",
  "startup_done": ["Ready"],
  "config_files": {"game.ini": {"parser": "ini", "find": {"net.port": "{{server.build.default.port}}"}}},
  "script_install_files": ["_scripts/shared.sh"],
  "script_install": "echo own-step\n",
  "ports": [{"name": "Query", "env": "QUERY_PORT", "default": 27016, "protocol": "udp"}],
  "variables": [
    {"name": "Name", "env_variable": "NAME", "default_value": "Atlas", "rules": "required|string"},
    {"name": "App", "env_variable": "APPID", "default_value": "1", "user_viewable": false, "user_editable": false}
  ]
}`

func TestCatalogIsSeededUpdatedAndPruned(t *testing.T) {
	testDatabase(t)
	t.Chdir(t.TempDir())

	writeFile(t, "eggs/_scripts/shared.sh", "#!/bin/bash\r\necho shared-step\r\n")
	writeFile(t, "eggs/games/nest.json", `{"name": "Game Servers", "description": "All of them"}`)
	writeFile(t, "eggs/games/test/game.json", testEgg)
	writeFile(t, "eggs/games/old/old.json", strings.ReplaceAll(strings.ReplaceAll(testEgg, "atlas-test-game", "atlas-old-game"), "Test Game", "Old Game"))

	scanEggsDirectory()

	var egg models.Egg
	if err := database.DB.Preload("Variables").Preload("Nest").Where("uuid = ?", "atlas-test-game").First(&egg).Error; err != nil {
		t.Fatal(err)
	}
	// The shared script comes first, with Unix line endings, then the egg's own.
	if egg.ScriptInstall != "#!/bin/bash\necho shared-step\necho own-step\n" {
		t.Errorf("install script = %q", egg.ScriptInstall)
	}
	if egg.StartupDone != `["Ready"]` || !strings.Contains(egg.ConfigFiles, `"parser":"ini"`) || !strings.Contains(egg.Ports, "QUERY_PORT") {
		t.Errorf("runtime fields: done=%s config=%s ports=%s", egg.StartupDone, egg.ConfigFiles, egg.Ports)
	}
	if egg.Source != catalogSource || !egg.Available || egg.Author != "Atlas" || egg.Nest.Code != "test" {
		t.Errorf("egg = %+v", egg)
	}
	if len(egg.Variables) != 2 {
		t.Fatalf("variables = %+v", egg.Variables)
	}
	for _, variable := range egg.Variables {
		if variable.EnvironmentVariable == "APPID" && (variable.UserViewable || variable.UserEditable) {
			t.Errorf("hidden variable is visible: %+v", variable)
		}
	}
	// "_scripts" is not a nest.
	var scriptNests int64
	database.DB.Model(&models.Nest{}).Where("code = ?", "_scripts").Count(&scriptNests)
	if scriptNests != 0 {
		t.Error("the shared script directory became a nest")
	}

	// Changing only the shared script updates the eggs that use it.
	writeFile(t, "eggs/_scripts/shared.sh", "#!/bin/bash\necho shared-step-v2\n")
	scanEggsDirectory()
	database.DB.Where("uuid = ?", "atlas-test-game").First(&egg)
	if !strings.Contains(egg.ScriptInstall, "shared-step-v2") {
		t.Errorf("shared script change was not picked up: %q", egg.ScriptInstall)
	}

	// An admin's edit survives a restart, even when the file changes.
	database.DB.Model(&models.Egg{}).Where("id = ?", egg.ID).Updates(map[string]interface{}{"customized": true, "startup_command": "./custom"})
	writeFile(t, "eggs/games/test/game.json", strings.ReplaceAll(testEgg, "./run {{SERVER_PORT}}", "./run-v2"))
	scanEggsDirectory()
	database.DB.Where("uuid = ?", "atlas-test-game").First(&egg)
	if egg.StartupCommand != "./custom" {
		t.Errorf("customized egg was overwritten: %q", egg.StartupCommand)
	}

	// A removed egg with no servers disappears with its now-empty nest; one
	// that a server still uses is only hidden.
	var old models.Egg
	database.DB.Where("uuid = ?", "atlas-old-game").First(&old)
	if err := os.RemoveAll("eggs/games/old"); err != nil {
		t.Fatal(err)
	}
	scanEggsDirectory()
	var remaining int64
	database.DB.Model(&models.Egg{}).Where("uuid = ?", "atlas-old-game").Count(&remaining)
	database.DB.Model(&models.Nest{}).Where("code = ?", "old").Count(&scriptNests)
	if remaining != 0 || scriptNests != 0 {
		t.Errorf("unused retired egg/nest remain: eggs=%d nests=%d", remaining, scriptNests)
	}

	user := models.User{Username: "owner", Password: "x"}
	node := models.Node{Name: "n", Address: "a", Token: "t"}
	if err := database.DB.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	if err := database.DB.Create(&node).Error; err != nil {
		t.Fatal(err)
	}
	service := models.Service{UUID: "svc", Name: "s", UserID: user.ID, NodeID: node.ID, EggID: egg.ID, NestID: egg.NestID, Memory: 1, Disk: 1, Cpu: 1, Port: 1}
	if err := database.DB.Create(&service).Error; err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll("eggs/games/test"); err != nil {
		t.Fatal(err)
	}

	// A catalog with nothing in it looks like a missing mount: nothing changes.
	scanEggsDirectory()
	if database.DB.Where("uuid = ?", "atlas-test-game").First(&egg); !egg.Available {
		t.Error("an empty catalog must not retire anything")
	}

	writeFile(t, "eggs/games/other/other.json", strings.ReplaceAll(strings.ReplaceAll(testEgg, "atlas-test-game", "atlas-other-game"), "Test Game", "Other Game"))
	scanEggsDirectory()
	if err := database.DB.Where("uuid = ?", "atlas-test-game").First(&egg).Error; err != nil || egg.Available {
		t.Errorf("an egg in use must be kept but hidden: err=%v available=%v", err, egg.Available)
	}
	var testNests int64
	database.DB.Model(&models.Nest{}).Where("code = ?", "test").Count(&testNests)
	if testNests != 1 {
		t.Errorf("a nest that still holds an egg must be kept, found %d", testNests)
	}
}

func TestScriptFilesCannotLeaveTheCatalog(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "_scripts", "ok.sh"), "echo ok\n")
	writeFile(t, filepath.Join(filepath.Dir(root), "outside.sh"), "echo outside\n")

	if _, err := readCatalogScript(root, "_scripts/ok.sh"); err != nil {
		t.Errorf("valid script: %v", err)
	}
	for _, name := range []string{"../outside.sh", "_scripts/../../outside.sh", "/etc/passwd", "", "."} {
		if _, err := readCatalogScript(root, name); err == nil {
			t.Errorf("%q should be refused", name)
		}
	}
}

// TestRepositoryCatalog checks every egg shipped in the repository: it must
// parse, name install scripts that exist and be complete enough to run.
func TestRepositoryCatalog(t *testing.T) {
	root := filepath.Join("..", "..", "..", "eggs")
	if _, err := os.Stat(root); err != nil {
		t.Skip("the eggs directory is not next to the source tree")
	}

	seen := map[string]string{}
	count := 0
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || filepath.Ext(path) != ".json" || entry.Name() == "nest.json" {
			return nil
		}
		count++
		relative, _ := filepath.Rel(root, path)

		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		parsed, err := eggfile.Parse(data)
		if err != nil {
			t.Errorf("%s: %v", relative, err)
			return nil
		}
		egg := parsed.Egg

		if !strings.HasPrefix(parsed.UUID, "atlas-") {
			t.Errorf("%s: uuid %q must start with atlas-", relative, parsed.UUID)
		}
		if other, duplicate := seen[parsed.UUID]; duplicate {
			t.Errorf("%s: uuid %q is already used by %s", relative, parsed.UUID, other)
		}
		seen[parsed.UUID] = relative

		script := egg.ScriptInstall
		for _, name := range parsed.ScriptFiles {
			content, err := readCatalogScript(root, name)
			if err != nil {
				t.Errorf("%s: %v", relative, err)
				continue
			}
			script = string(content) + script
		}
		if strings.TrimSpace(script) != "" && (egg.ScriptContainer == "" || egg.ScriptEntry == "") {
			t.Errorf("%s: an install script needs script_container and script_entry", relative)
		}
		if strings.TrimSpace(egg.Description) == "" {
			t.Errorf("%s: missing description", relative)
		}

		// Every {{NAME}} in the startup command must be something Atlas
		// provides, a variable or a port; anything else would silently
		// become an empty string.
		known := map[string]bool{"SERVER_PORT": true, "SERVER_MEMORY": true, "SERVER_IP": true, "SERVER_UUID": true}
		for _, variable := range parsed.Variables {
			known[variable.EnvironmentVariable] = true
			if err := eggfile.ValidateValue(variable.Rules, variable.DefaultValue); err != nil && variable.DefaultValue != "" {
				t.Errorf("%s: default of %s breaks its own rules: %v", relative, variable.EnvironmentVariable, err)
			}
			if strings.TrimSpace(variable.Name) == "" {
				t.Errorf("%s: variable %s has no name", relative, variable.EnvironmentVariable)
			}
		}
		var ports []models.EggPort
		if egg.Ports != "" {
			if err := json.Unmarshal([]byte(egg.Ports), &ports); err != nil {
				t.Errorf("%s: ports: %v", relative, err)
			}
		}
		for _, port := range ports {
			if port.Env == "" || port.Default < 1 || port.Default > 65535 {
				t.Errorf("%s: port %+v needs an env name and a default", relative, port)
			}
			if known[port.Env] {
				t.Errorf("%s: %s is both a port and a variable", relative, port.Env)
			}
			known[port.Env] = true
		}
		rest := egg.StartupCommand
		for {
			start := strings.Index(rest, "{{")
			if start < 0 {
				break
			}
			end := strings.Index(rest[start:], "}}")
			if end < 0 {
				t.Errorf("%s: unclosed {{ in the startup command", relative)
				break
			}
			name := strings.TrimSpace(rest[start+2 : start+end])
			if !known[name] {
				t.Errorf("%s: startup command uses {{%s}}, which is not a variable or port of this egg", relative, name)
			}
			rest = rest[start+end+2:]
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if count == 0 {
		t.Fatal("no eggs found in the repository catalog")
	}
}
