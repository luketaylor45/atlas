package api

import (
	"bufio"
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/luketaylor45/atlas/daemon/internal/config"
)

const testService = "11111111-2222-3333-4444-555555555555"

func useTempDataPath(t *testing.T) string {
	t.Helper()
	previous, previousHost := config.NodeConfig.DataPath, config.NodeConfig.HostDataPath
	config.NodeConfig.DataPath = t.TempDir()
	config.NodeConfig.HostDataPath = config.NodeConfig.DataPath
	t.Cleanup(func() {
		config.NodeConfig.DataPath, config.NodeConfig.HostDataPath = previous, previousHost
	})
	return config.NodeConfig.DataPath
}

func TestRuntimeSettingsAreStoredPerServer(t *testing.T) {
	useTempDataPath(t)

	if _, ok := loadRuntime(testService); ok {
		t.Fatal("a server with no stored settings reported some")
	}

	want := serverRuntime{
		ConfigFiles: `{"server.properties":{"parser":"properties","find":{"server-port":"{{server.build.default.port}}"}}}`,
		StartupDone: []string{"Done (", "regex:^Ready"},
		StopCommand: "stop",
		Port:        25565,
		Memory:      2048,
		Environment: `{"MOTD":"hello"}`,
	}
	saveRuntime(testService, want)

	got, ok := loadRuntime(testService)
	if !ok || !reflect.DeepEqual(got, want) {
		t.Fatalf("loaded %+v (ok=%v), want %+v", got, ok, want)
	}

	// The settings live with the files Atlas manages, not among the game files.
	if _, err := os.Stat(filepath.Join(config.NodeConfig.DataPath, ".atlas", "servers", testService+".json")); err != nil {
		t.Fatalf("settings file: %v", err)
	}

	removeRuntime(testService)
	if _, ok := loadRuntime(testService); ok {
		t.Fatal("settings survived removal")
	}
}

func TestConfigFilesAreRewrittenFromTheServerSettings(t *testing.T) {
	root := useTempDataPath(t)
	serverDir := filepath.Join(root, testService)
	if err := os.MkdirAll(serverDir, 0770); err != nil {
		t.Fatal(err)
	}
	properties := filepath.Join(serverDir, "server.properties")
	if err := os.WriteFile(properties, []byte("# Minecraft server properties\nserver-port=25565\nmotd=A Minecraft Server\nmax-players=20\n"), 0660); err != nil {
		t.Fatal(err)
	}

	applyConfigFiles(context.Background(), testService, serverRuntime{
		ConfigFiles: `{
			"server.properties": {"parser": "properties", "find": {
				"server-ip": "{{server.build.default.ip}}",
				"server-port": "{{server.build.default.port}}",
				"query.port": "{{env.SERVER_PORT}}",
				"motd": "{{server.build.env.MOTD}}"
			}},
			"config/settings.json": {"parser": "json", "find": {"network.port": "{{server.build.default.port}}", "memory": "{{server.build.memory}}"}},
			"../outside.properties": {"parser": "properties", "find": {"a": "b"}}
		}`,
		Port:        25600,
		Memory:      4096,
		Environment: `{"MOTD":"Atlas rocks","SERVER_PORT":"1"}`,
	})

	content, err := os.ReadFile(properties)
	if err != nil {
		t.Fatal(err)
	}
	text := string(content)
	for _, want := range []string{"# Minecraft server properties", "server-port=25600", "server-ip=0.0.0.0", "query.port=25600", "motd=Atlas rocks", "max-players=20"} {
		if !strings.Contains(text, want) {
			t.Errorf("server.properties is missing %q:\n%s", want, text)
		}
	}

	settings, err := os.ReadFile(filepath.Join(serverDir, "config", "settings.json"))
	if err != nil {
		t.Fatalf("a missing JSON file should be created: %v", err)
	}
	if !strings.Contains(string(settings), "25600") || !strings.Contains(string(settings), "4096") {
		t.Errorf("settings.json = %s", settings)
	}

	if _, err := os.Stat(filepath.Join(root, "outside.properties")); err == nil {
		t.Error("a config file was written outside the server directory")
	}

	// Broken rules are skipped without touching anything.
	applyConfigFiles(context.Background(), testService, serverRuntime{ConfigFiles: `{not json`, Port: 1})
	applyConfigFiles(context.Background(), testService, serverRuntime{})
	after, _ := os.ReadFile(properties)
	if string(after) != text {
		t.Error("invalid rules changed a file")
	}
}

func TestDoneMatcher(t *testing.T) {
	if newDoneMatcher(nil) != nil || newDoneMatcher([]string{"", "  "}) != nil {
		t.Fatal("an egg with no done-strings must not be tracked")
	}
	if newDoneMatcher([]string{"regex:("}) != nil {
		t.Fatal("an egg with only an invalid pattern must not be tracked")
	}

	matcher := newDoneMatcher([]string{"Done (", `regex:^\[\d+\] Server is ready$`, "regex:("})
	tests := []struct {
		line string
		want bool
	}{
		{`[12:00:01 INFO]: Done (12.345s)! For help, type "help"`, true},
		{"[12:00:01 INFO]: Preparing spawn area: 80%", false},
		{"[42] Server is ready", true},
		{"prefix [42] Server is ready", false},
		// Colour codes in the middle of the text do not hide it.
		{"\x1b[32m[12:00:01 INFO]: \x1b[0mDone\x1b[1m (3.2s)!\x1b[0m", true},
		{"done (lowercase does not count)", false},
	}
	for _, test := range tests {
		if got := matcher.matches(test.line); got != test.want {
			t.Errorf("matches(%q) = %v, want %v", test.line, got, test.want)
		}
	}
}

func TestConsoleOutputIsSplitOnNewlinesAndCarriageReturns(t *testing.T) {
	scanner := bufio.NewScanner(strings.NewReader("one\r\ntwo\rthree\nfour"))
	scanner.Split(scanLinesOrReturns)
	var lines []string
	for scanner.Scan() {
		if scanner.Text() != "" {
			lines = append(lines, scanner.Text())
		}
	}
	if want := []string{"one", "two", "three", "four"}; !reflect.DeepEqual(lines, want) {
		t.Fatalf("lines = %q, want %q", lines, want)
	}
}

func TestReadinessTracker(t *testing.T) {
	tracker := &readinessTracker{starting: map[string]context.CancelFunc{}}

	if tracker.isStarting("a") {
		t.Fatal("an unknown server is starting")
	}
	ctx := tracker.begin("a")
	if !tracker.isStarting("a") {
		t.Fatal("a server that began starting is not tracked")
	}
	if !tracker.finish("a", ctx) || tracker.isStarting("a") {
		t.Fatal("finishing did not mark the server as ready")
	}
	if tracker.finish("a", ctx) {
		t.Fatal("a server was reported ready twice")
	}

	// A server that stops while starting is never reported as running.
	ctx = tracker.begin("b")
	tracker.forget("b")
	if ctx.Err() == nil {
		t.Fatal("stopping did not end the watcher")
	}
	if tracker.finish("b", ctx) {
		t.Fatal("a stopped server was reported as running")
	}

	// A restart replaces the watcher; the old one must stay silent.
	first := tracker.begin("c")
	second := tracker.begin("c")
	if first.Err() == nil {
		t.Fatal("the first watcher was not stopped by the restart")
	}
	if tracker.finish("c", first) {
		t.Fatal("the replaced watcher reported the server as running")
	}
	if !tracker.isStarting("c") || !tracker.finish("c", second) {
		t.Fatal("the current watcher could not finish")
	}
}

func TestStartScriptSubstitutesVariablesAndProtectsAtlasValues(t *testing.T) {
	root := useTempDataPath(t)

	err := writeStartScript(testService, "./run -port {{SERVER_PORT}} -ip {{SERVER_IP}} -mem {{SERVER_MEMORY}} -name {{NAME}} -q {{QUERY_PORT}} {{UNKNOWN}}end", 27015, 2048,
		`{"NAME":"Atlas","QUERY_PORT":"27016","SERVER_PORT":"1","SERVER_MEMORY":"1"}`)
	if err != nil {
		t.Fatal(err)
	}
	script, err := os.ReadFile(filepath.Join(root, testService, "start.sh"))
	if err != nil {
		t.Fatal(err)
	}
	if want := "./run -port 27015 -ip 0.0.0.0 -mem 2048 -name Atlas -q 27016 end\n"; !strings.Contains(string(script), want) {
		t.Fatalf("start script does not contain %q:\n%s", want, script)
	}
	if strings.Contains(string(script), "\r") {
		t.Fatal("start script has Windows line endings")
	}
}
