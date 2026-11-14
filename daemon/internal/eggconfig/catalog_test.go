package eggconfig

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// catalogConfigFiles returns the config_files section of every egg shipped
// in the repository, keyed by the egg's path.
func catalogConfigFiles(t *testing.T) map[string]string {
	t.Helper()
	root := filepath.Join("..", "..", "..", "eggs")
	if _, err := os.Stat(root); err != nil {
		t.Skip("the eggs directory is not next to the source tree")
	}

	found := map[string]string{}
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || filepath.Ext(path) != ".json" || entry.Name() == "nest.json" {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		var egg struct {
			ConfigFiles json.RawMessage `json:"config_files"`
		}
		if err := json.Unmarshal(data, &egg); err != nil {
			t.Errorf("%s: %v", path, err)
			return nil
		}
		if len(egg.ConfigFiles) > 0 {
			relative, _ := filepath.Rel(root, path)
			found[filepath.ToSlash(relative)] = string(egg.ConfigFiles)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return found
}

// An egg whose rules do not parse would have all of its config files skipped
// at every start, so every shipped egg is checked here.
func TestCatalogConfigFilesParse(t *testing.T) {
	sections := catalogConfigFiles(t)
	if len(sections) == 0 {
		t.Fatal("no shipped egg has config files; the catalog was not found")
	}
	for egg, raw := range sections {
		files, err := Parse(raw)
		if err != nil {
			t.Errorf("%s: %v", egg, err)
			continue
		}
		for _, file := range files {
			if len(file.Find) == 0 {
				t.Errorf("%s: %s has nothing to replace", egg, file.Path)
			}
		}
	}
}

func TestCatalogEnshroudedKeepsNumbersAsNumbers(t *testing.T) {
	raw, ok := catalogConfigFiles(t)["games/survival/enshrouded.json"]
	if !ok {
		t.Skip("the Enshrouded egg is not in the catalog")
	}
	files, err := Parse(raw)
	if err != nil {
		t.Fatal(err)
	}

	root := t.TempDir()
	original := "{\n\t\"name\": \"Enshrouded Server\",\n\t\"ip\": \"0.0.0.0\",\n\t\"queryPort\": 15637,\n\t\"slotCount\": 16,\n\t\"gameSettingsPreset\": \"Default\",\n\t\"userGroups\": [\n\t\t{\"name\": \"Admin\", \"password\": \"secret\"}\n\t]\n}\n"
	if err := os.WriteFile(filepath.Join(root, "enshrouded_server.json"), []byte(original), 0660); err != nil {
		t.Fatal(err)
	}

	failures := Apply(root, files, Values{Port: 15700, IP: "0.0.0.0", Env: map[string]string{
		"SERVER_NAME": "12345", "MAX_PLAYERS": "8", "DIFFICULTY_PRESET": "Hard",
	}})
	if len(failures) > 0 {
		t.Fatal(failures)
	}

	var written struct {
		Name       any `json:"name"`
		QueryPort  any `json:"queryPort"`
		SlotCount  any `json:"slotCount"`
		Preset     any `json:"gameSettingsPreset"`
		UserGroups []struct {
			Password string `json:"password"`
		} `json:"userGroups"`
	}
	content, err := os.ReadFile(filepath.Join(root, "enshrouded_server.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(content, &written); err != nil {
		t.Fatalf("the file is no longer valid JSON: %v\n%s", err, content)
	}
	// The game rejects a port or slot count written as text, and a server
	// named with digits must stay text.
	if written.QueryPort != float64(15700) || written.SlotCount != float64(8) {
		t.Errorf("queryPort=%#v slotCount=%#v, want numbers", written.QueryPort, written.SlotCount)
	}
	if written.Name != "12345" || written.Preset != "Hard" {
		t.Errorf("name=%#v preset=%#v, want text", written.Name, written.Preset)
	}
	if len(written.UserGroups) != 1 || written.UserGroups[0].Password != "secret" {
		t.Errorf("unrelated settings changed: %s", content)
	}
}

func TestCatalogZomboidCreatesItsSettingsFile(t *testing.T) {
	raw, ok := catalogConfigFiles(t)["games/survival/zomboid.json"]
	if !ok {
		t.Skip("the Project Zomboid egg is not in the catalog")
	}
	files, err := Parse(raw)
	if err != nil {
		t.Fatal(err)
	}

	// Before the first start the file does not exist yet.
	root := t.TempDir()
	failures := Apply(root, files, Values{Port: 16261, Env: map[string]string{
		"UDP_PORT": "16262", "SERVER_PUBLIC_NAME": "Atlas: Knox County #1", "SERVER_PASSWORD": "", "MAX_PLAYERS": "32", "PUBLIC_SERVER": "false",
	}})
	if len(failures) > 0 {
		t.Fatal(failures)
	}
	content, err := os.ReadFile(filepath.Join(root, ".cache", "Server", "servertest.ini"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(content)
	for _, want := range []string{"DefaultPort=16261", "UDPPort=16262", "PublicName=Atlas: Knox County #1", "MaxPlayers=32", "Public=false", "Password="} {
		if !strings.Contains(text, want) {
			t.Errorf("servertest.ini is missing %q:\n%s", want, text)
		}
	}
}
