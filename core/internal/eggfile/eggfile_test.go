package eggfile

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/luketaylor45/atlas/core/internal/models"
)

const atlasEgg = `{
  "uuid": "atlas-test",
  "name": " Test ",
  "docker_images": ["image:2", "image:1"],
  "startup_command": "run {{SERVER_PORT}}",
  "stop_command": "stop",
  "startup_done": ["Done (", "regex:^Ready"],
  "config_files": {"server.properties": {"parser": "properties", "find": {"server-port": "{{server.build.default.port}}", "motd": "hi"}}},
  "script_install_files": ["_scripts/steamcmd.sh"],
  "script_install": "echo done",
  "ports": [{"name": "Query", "env": "QUERY_PORT", "default": 27016, "protocol": "udp"}],
  "variables": [
    {"name": "Version", "env_variable": "VERSION", "default_value": "latest", "rules": "required|string"},
    {"name": "Hidden", "env_variable": "APPID", "default_value": 730, "user_viewable": false, "user_editable": false},
    {"name": "Duplicate", "env_variable": "VERSION", "default_value": "x"},
    {"name": "No env", "env_variable": " "}
  ]
}`

func TestParseAtlasEgg(t *testing.T) {
	parsed, err := Parse([]byte("\xef\xbb\xbf" + atlasEgg))
	if err != nil {
		t.Fatal(err)
	}
	egg := parsed.Egg

	if parsed.UUID != "atlas-test" || egg.Name != "Test" {
		t.Fatalf("identity = %q / %q", parsed.UUID, egg.Name)
	}
	if egg.DockerImages != `["image:2","image:1"]` {
		t.Errorf("images = %s", egg.DockerImages)
	}
	if egg.StartupDone != `["Done (","regex:^Ready"]` {
		t.Errorf("startup done = %s", egg.StartupDone)
	}
	// Key order inside "find" is meaningful and must survive.
	if port, motd := strings.Index(egg.ConfigFiles, "server-port"), strings.Index(egg.ConfigFiles, "motd"); port < 0 || motd < port {
		t.Errorf("config files = %s", egg.ConfigFiles)
	}
	if len(parsed.ScriptFiles) != 1 || egg.ScriptInstall != "echo done" {
		t.Errorf("scripts = %v / %q", parsed.ScriptFiles, egg.ScriptInstall)
	}

	var ports []models.EggPort
	if err := json.Unmarshal([]byte(egg.Ports), &ports); err != nil || len(ports) != 1 || ports[0].Env != "QUERY_PORT" {
		t.Errorf("ports = %s", egg.Ports)
	}

	if len(parsed.Variables) != 2 {
		t.Fatalf("variables = %+v", parsed.Variables)
	}
	version, appID := parsed.Variables[0], parsed.Variables[1]
	if !version.UserViewable || !version.UserEditable || version.InputType != "text" {
		t.Errorf("flags default to true: %+v", version)
	}
	if appID.DefaultValue != "730" || appID.UserViewable || appID.UserEditable {
		t.Errorf("hidden numeric variable = %+v", appID)
	}
}

// A PTDL_v2 export: config sections are JSON-encoded strings and images are
// a map of label to image.
const pterodactylEgg = `{
  "_comment": "DO NOT EDIT",
  "meta": {"version": "PTDL_v2"},
  "name": "Paper",
  "author": "someone@example.com",
  "docker_images": {"Java 8": "yolks:java_8", "Java 21": "yolks:java_21", "Java 17": "yolks:java_17"},
  "startup": "java -jar {{SERVER_JARFILE}}",
  "config": {
    "files": "{\r\n    \"server.properties\": {\r\n        \"parser\": \"properties\",\r\n        \"find\": {\r\n            \"server-ip\": \"0.0.0.0\",\r\n            \"server-port\": \"{{server.build.default.port}}\"\r\n        }\r\n    }\r\n}",
    "startup": "{\r\n    \"done\": \")! For help, type \"\r\n}",
    "logs": "{}",
    "stop": "stop"
  },
  "scripts": {"installation": {"script": "#!/bin/ash\necho hi", "container": "installers:alpine", "entrypoint": "ash"}},
  "variables": [
    {"name": "Jar", "env_variable": "SERVER_JARFILE", "default_value": "server.jar", "user_viewable": true, "user_editable": true, "rules": "required|regex:/^([\\w\\d._-]+)(\\.jar)$/", "field_type": "text"},
    {"name": "Query Port", "env_variable": "QUERY_PORT", "default_value": "25566", "rules": "required|integer"},
    {"name": "Transport", "env_variable": "TRANSPORT", "default_value": "", "rules": "nullable|string"}
  ]
}`

func TestParsePterodactylEgg(t *testing.T) {
	parsed, err := Parse([]byte(pterodactylEgg))
	if err != nil {
		t.Fatal(err)
	}
	egg := parsed.Egg

	if egg.DockerImages != `["yolks:java_21","yolks:java_17","yolks:java_8"]` {
		t.Errorf("images are newest first: %s", egg.DockerImages)
	}
	if egg.StartupCommand != "java -jar {{SERVER_JARFILE}}" || egg.StopCommand != "stop" {
		t.Errorf("startup/stop = %q / %q", egg.StartupCommand, egg.StopCommand)
	}
	if egg.StartupDone != `[")! For help, type "]` {
		t.Errorf("startup done = %s", egg.StartupDone)
	}
	if !strings.HasPrefix(egg.ConfigFiles, `{"server.properties":{"parser":"properties"`) {
		t.Errorf("config files = %s", egg.ConfigFiles)
	}
	if egg.ScriptContainer != "installers:alpine" || egg.ScriptEntry != "ash" {
		t.Errorf("installer = %q / %q", egg.ScriptContainer, egg.ScriptEntry)
	}

	// QUERY_PORT is an extra port, not a free-text variable.
	if !strings.Contains(egg.Ports, `"env":"QUERY_PORT"`) || !strings.Contains(egg.Ports, `"default":25566`) {
		t.Errorf("ports = %s", egg.Ports)
	}
	if len(parsed.Variables) != 2 || parsed.Variables[0].EnvironmentVariable != "SERVER_JARFILE" {
		t.Fatalf("variables = %+v", parsed.Variables)
	}
	if len(parsed.Notes) != 1 || !strings.Contains(parsed.Notes[0], "QUERY_PORT") {
		t.Errorf("notes = %v", parsed.Notes)
	}
}

func TestParsePterodactylVariants(t *testing.T) {
	tests := []struct {
		name       string
		file       string
		wantDone   string
		wantConfig string
		wantImages string
	}{
		{
			name:       "config sections as objects and done as a list",
			file:       `{"name":"A","startup":"run","docker_images":{"x":"img"},"config":{"files":{"a.json":{"parser":"json","find":{"port":1}}},"startup":{"done":["one","two"]}}}`,
			wantDone:   `["one","two"]`,
			wantConfig: `{"a.json":{"parser":"json","find":{"port":1}}}`,
			wantImages: `["img"]`,
		},
		{
			name:       "empty sections",
			file:       `{"name":"A","startup":"run","docker_images":{"x":"img"},"config":{"files":"[]","startup":"{}"}}`,
			wantImages: `["img"]`,
		},
		{
			name:       "legacy single image",
			file:       `{"name":"A","startup":"run","image":"old:image","config":{"files":"{}","startup":"{\"done\": \"ok\"}"}}`,
			wantDone:   `["ok"]`,
			wantImages: `["old:image"]`,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			parsed, err := Parse([]byte(test.file))
			if err != nil {
				t.Fatal(err)
			}
			if parsed.Egg.StartupDone != test.wantDone {
				t.Errorf("done = %s, want %s", parsed.Egg.StartupDone, test.wantDone)
			}
			if parsed.Egg.ConfigFiles != test.wantConfig {
				t.Errorf("config = %s, want %s", parsed.Egg.ConfigFiles, test.wantConfig)
			}
			if parsed.Egg.DockerImages != test.wantImages {
				t.Errorf("images = %s, want %s", parsed.Egg.DockerImages, test.wantImages)
			}
		})
	}
}

func TestParseKeepsImportUsableWhenOptionalSectionsAreBroken(t *testing.T) {
	file := `{"name":"A","startup":"run","docker_images":{"x":"img"},"config":{"files":"{not json","startup":"{\"done\": 5}"}}`
	parsed, err := Parse([]byte(file))
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Egg.ConfigFiles != "" || parsed.Egg.StartupDone != "" || len(parsed.Notes) != 2 {
		t.Errorf("config=%q done=%q notes=%v", parsed.Egg.ConfigFiles, parsed.Egg.StartupDone, parsed.Notes)
	}
}

func TestParseRejects(t *testing.T) {
	tests := map[string]string{
		"not json":       `{`,
		"no name":        `{"startup_command":"run","docker_images":["a"]}`,
		"no startup":     `{"name":"A","startup":" ","docker_images":{"x":"img"}}`,
		"no images":      `{"name":"A","startup_command":"run","docker_images":[]}`,
		"unknown parser": `{"name":"A","startup_command":"run","docker_images":["a"],"config_files":{"a":{"parser":"toml","find":{}}}}`,
		"bad done":       `{"name":"A","startup_command":"run","docker_images":["a"],"startup_done":{"a":1}}`,
	}
	for name, file := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := Parse([]byte(file)); err == nil {
				t.Error("expected an error")
			}
		})
	}
}

func TestPortFromVariable(t *testing.T) {
	tests := []struct {
		env      string
		value    any
		wantPort bool
	}{
		{"QUERY_PORT", "27016", true},
		{"RCON_PORT", float64(25575), true},
		{"SERVER_PORT", "25565", false},
		{"QUERY_PORT", "", false},
		{"QUERY_PORT", "{{SERVER_PORT}}", false},
		{"PORT_RANGE", "27016", false},
		{"BEACON_PORT", "70000", false},
	}
	for _, test := range tests {
		_, ok := portFromVariable(fileVariable{EnvVariable: test.env, DefaultValue: test.value})
		if ok != test.wantPort {
			t.Errorf("%s=%v: port=%v, want %v", test.env, test.value, ok, test.wantPort)
		}
	}
}
