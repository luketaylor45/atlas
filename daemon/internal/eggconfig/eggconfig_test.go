package eggconfig

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

var testValues = Values{
	Port:      25565,
	IP:        "0.0.0.0",
	Memory:    2048,
	UUID:      "3f2a9c1e-0000-4000-8000-000000000001",
	Interface: "172.18.0.1",
	Env: map[string]string{
		"MOTD":        `Atlas: a \ test`,
		"MAX_PLAYERS": "20",
		"ONLINE_MODE": "true",
		"SERVER_NAME": `Atlas <Test> & "Co"`,
		"LEVEL":       "world",
	},
}

func writeTestFile(t *testing.T, root, rel, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o770); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o660); err != nil {
		t.Fatal(err)
	}
}

func readTestFile(t *testing.T, root, rel string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func exists(root, rel string) bool {
	_, err := os.Lstat(filepath.Join(root, filepath.FromSlash(rel)))
	return err == nil
}

// config builds a one-file config_files document.
func config(path, parser, find string) string {
	return `{"` + path + `": {"parser": "` + parser + `", "find": ` + find + `}}`
}

func mustParse(t *testing.T, raw string) []File {
	t.Helper()
	files, err := Parse(raw)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	return files
}

func mustApply(t *testing.T, root, raw string) {
	t.Helper()
	if errs := Apply(root, mustParse(t, raw), testValues); errs != nil {
		t.Fatalf("Apply: %v", errs)
	}
}

// assertNoTempFiles fails when an atomic write left something behind.
func assertNoTempFiles(t *testing.T, root string) {
	t.Helper()
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err == nil && strings.HasSuffix(entry.Name(), ".tmp") {
			t.Errorf("leftover temporary file %s", path)
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestResolve(t *testing.T) {
	cases := []struct{ in, want string }{
		{"{{server.build.default.port}}", "25565"},
		{"{{ server.build.default.ip }}:{{server.build.default.port}}", "0.0.0.0:25565"},
		{"{{\tserver.build.default.port\n}}", "25565"},
		{"-Xmx{{server.build.memory}}M", "-Xmx2048M"},
		{"{{server.build.env.MAX_PLAYERS}}", "20"},
		{"{{env.MAX_PLAYERS}}/{{env.LEVEL}}", "20/world"},
		{"{{server.uuid}}", "3f2a9c1e-0000-4000-8000-000000000001"},
		{"{{config.docker.interface}}", "172.18.0.1"},
		{"{{server.allocations.default.port}}", "25565"},
		{"{{server.environment.LEVEL}}", "world"},
		{"{{config.docker.network.interface}}", "172.18.0.1"},
		{"a{{unknown.thing}}b", "ab"},
		{"{{env.MISSING}}x", "x"},
		{"{{}}", ""},
		{"plain text", "plain text"},
		{"{ not a placeholder }", "{ not a placeholder }"},
		{"", ""},
	}
	for _, tc := range cases {
		if got := Resolve(tc.in, testValues); got != tc.want {
			t.Errorf("Resolve(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestParseEmpty(t *testing.T) {
	for _, raw := range []string{"", "   \n", "{}", "[]", "null", "{ }", "[ ]", `"{}"`} {
		files, err := Parse(raw)
		if err != nil || files != nil {
			t.Errorf("Parse(%q) = %v, %v; want nil, nil", raw, files, err)
		}
	}
}

func TestParsePreservesOrderAndTypes(t *testing.T) {
	raw := `{
		"z.yml": {"parser": "YAML", "find": {
			"zulu": "{{server.build.default.port}}",
			"alpha": 5,
			"mike": 1.5,
			"bravo": true,
			"servers.*.address": {"localhost": "{{config.docker.interface}}", "127.0.0.1": 7}
		}},
		"a.properties": {"parser": "properties", "find": {"b": "2", "a": "1"}},
		"empty.ini": {"parser": "ini", "find": []}
	}`
	want := []File{
		{Path: "z.yml", Parser: "yaml", Find: []Replacement{
			{"zulu", "{{server.build.default.port}}"},
			{"alpha", int64(5)},
			{"mike", 1.5},
			{"bravo", true},
			{"servers.*.address", []Replacement{{"localhost", "{{config.docker.interface}}"}, {"127.0.0.1", int64(7)}}},
		}},
		{Path: "a.properties", Parser: "properties", Find: []Replacement{{"b", "2"}, {"a", "1"}}},
		{Path: "empty.ini", Parser: "ini"},
	}
	if got := mustParse(t, raw); !reflect.DeepEqual(got, want) {
		t.Errorf("Parse =\n%#v\nwant\n%#v", got, want)
	}

	// Pterodactyl exports carry the document as a JSON string.
	wrapped := `"{\"server.properties\": {\"parser\": \"properties\", \"find\": {\"server-port\": \"1\"}}}"`
	got := mustParse(t, wrapped)
	if len(got) != 1 || got[0].Path != "server.properties" || len(got[0].Find) != 1 {
		t.Errorf("Parse(string document) = %#v", got)
	}
}

func TestParseErrors(t *testing.T) {
	cases := map[string]string{
		"invalid json":           `{"a": `,
		"array document":         `[1]`,
		"scalar document":        `5`,
		"entry not an object":    `{"a": "properties"}`,
		"missing parser":         `{"a": {"find": {"k": "v"}}}`,
		"unknown parser":         `{"a": {"parser": "toml", "find": {"k": "v"}}}`,
		"parser not a string":    `{"a": {"parser": 1, "find": {}}}`,
		"find not an object":     `{"a": {"parser": "json", "find": "x"}}`,
		"array value":            `{"a": {"parser": "json", "find": {"k": [1]}}}`,
		"nested conditional":     `{"a": {"parser": "json", "find": {"k": {"x": {"y": "z"}}}}}`,
		"conditional properties": `{"a": {"parser": "properties", "find": {"k": {"x": "y"}}}}`,
		"conditional xml":        `{"a": {"parser": "xml", "find": {"k": {"x": "y"}}}}`,
		"bad regex":              `{"a": {"parser": "yaml", "find": {"k": {"regex:(": "y"}}}}`,
		"bad json path":          `{"a": {"parser": "json", "find": {"a..b": "x"}}}`,
		"unterminated index":     `{"a": {"parser": "yaml", "find": {"a[0": "x"}}}`,
		"bad xml path":           `{"a": {"parser": "xml", "find": {"a[b=1]": "x"}}}`,
		"empty key":              `{"a": {"parser": "ini", "find": {"": "x"}}}`,
		"empty path":             `{"": {"parser": "ini", "find": {"a": "x"}}}`,
	}
	for name, raw := range cases {
		if files, err := Parse(raw); err == nil {
			t.Errorf("%s: Parse succeeded with %#v", name, files)
		}
	}
}

const minecraftProperties = `#Minecraft server properties
#Sat Nov 14 10:00:00 UTC 2026
allow-flight=false
enable-query=false
generator-settings={}
level-name=world
level-seed=
max-players=10
motd=A Minecraft Server
online-mode=true
query.port=25565
resource-pack=https\://example.com/pack.zip?a\=b
resource-pack-prompt=\u00A7aPlease accept
server-ip=
server-port=25565
`

func TestPropertiesMinecraft(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "server.properties", strings.ReplaceAll(minecraftProperties, "25565", "25000"))

	raw := config("server.properties", "properties", `{
		"server-ip": "{{server.build.default.ip}}",
		"server-port": "{{server.build.default.port}}",
		"query.port": "{{server.build.default.port}}",
		"max-players": "{{server.build.env.MAX_PLAYERS}}",
		"motd": "{{env.MOTD}}",
		"enable-query": true,
		"level-name": "{{env.LEVEL}}",
		"enable-rcon": "false",
		"rcon.port": 25575
	}`)
	mustApply(t, root, raw)

	want := minecraftProperties
	for _, r := range [][2]string{
		{"server-ip=\n", "server-ip=0.0.0.0\n"},
		{"max-players=10\n", "max-players=20\n"},
		{"motd=A Minecraft Server\n", `motd=Atlas: a \\ test` + "\n"},
		{"enable-query=false\n", "enable-query=true\n"},
	} {
		if !strings.Contains(want, r[0]) {
			t.Fatalf("fixture lacks %q", r[0])
		}
		want = strings.Replace(want, r[0], r[1], 1)
	}
	want += "enable-rcon=false\nrcon.port=25575\n"
	if got := readTestFile(t, root, "server.properties"); got != want {
		t.Errorf("server.properties =\n%s\nwant\n%s", got, want)
	}

	// A second run changes nothing, and neither does a file that already matches.
	before, err := os.Stat(filepath.Join(root, "server.properties"))
	if err != nil {
		t.Fatal(err)
	}
	mustApply(t, root, raw)
	after, err := os.Stat(filepath.Join(root, "server.properties"))
	if err != nil {
		t.Fatal(err)
	}
	if got := readTestFile(t, root, "server.properties"); got != want {
		t.Errorf("second run changed the file:\n%s", got)
	}
	if !after.ModTime().Equal(before.ModTime()) {
		t.Error("second run rewrote an unchanged file")
	}
	assertNoTempFiles(t, root)
}

func TestProperties(t *testing.T) {
	cases := []struct {
		name  string
		input string // "-" means the file does not exist
		find  string
		want  string
	}{
		{"spaced separator", "key = old\n", `{"key": "new"}`, "key = new\n"},
		{"colon separator", "key:old\n", `{"key": "new"}`, "key:new\n"},
		{"spaced colon", "key : old\n", `{"key": "new"}`, "key : new\n"},
		{"whitespace separator", "key old\n", `{"key": "new"}`, "key new\n"},
		{"bare key", "key\n", `{"key": "new"}`, "key=new\n"},
		{"indented key", "  key=old\n", `{"key": "new"}`, "  key=new\n"},
		{"empty value", "key=\nother=1\n", `{"key": "new"}`, "key=new\nother=1\n"},
		{"crlf kept", "a=1\r\nb=2\r\n", `{"b": "3", "c": "4"}`, "a=1\r\nb=3\r\nc=4\r\n"},
		{"no final newline", "a=1", `{"b": "2"}`, "a=1\nb=2\n"},
		{"no final newline update", "a=1\nb=2", `{"b": "3"}`, "a=1\nb=3"},
		{"comments untouched", "# a=1\n! a=2\n  # a=4\na=3\n", `{"a": "9"}`, "# a=1\n! a=2\n  # a=4\na=9\n"},
		{"prefix is not a match", "ab=1\na=2\n", `{"a": "9"}`, "ab=1\na=9\n"},
		{"continuation collapsed", "a=one \\\n   two\nb=1\n", `{"a": "x"}`, "a=x\nb=1\n"},
		{"duplicates all updated", "a=1\nb=2\na=3\n", `{"a": "9"}`, "a=9\nb=2\na=9\n"},
		{"unicode escape kept", "motd=old\n", `{"motd": "\\u00a7aHello \\u00A7lWorld"}`, "motd=\\u00a7aHello \\u00A7lWorld\n"},
		{"backslash escaped", "path=old\n", `{"path": "C:\\dir\\users"}`, "path=C:\\\\dir\\\\users\n"},
		{"newline and tab escaped", "v=old\n", `{"v": "a\nb\tc"}`, "v=a\\nb\\tc\n"},
		{"leading space escaped", "v=old\n", `{"v": " x y"}`, "v=\\ x y\n"},
		{"utf8 kept", "motd=old\n", `{"motd": "Zażółć §a"}`, "motd=Zażółć §a\n"},
		{"special characters in value", "v=old\n", `{"v": "a=b:c#d!e"}`, "v=a=b:c#d!e\n"},
		{"key needing escapes appended", "a=1\n", `{"my key": "v", "#x": "y"}`, "a=1\nmy\\ key=v\n\\#x=y\n"},
		{"escaped key matched", "my\\ key=1\n", `{"my key": "2"}`, "my\\ key=2\n"},
		{"equal after unescape untouched", "url=https\\://x/y\n", `{"url": "https://x/y"}`, "url=https\\://x/y\n"},
		{"appended before trailing blanks", "a=1\n\n", `{"b": "2"}`, "a=1\nb=2\n\n"},
		{"number and bool values", "a=1\n", `{"a": 2, "b": false, "c": 1.5}`, "a=2\nb=false\nc=1.5\n"},
		{"created when missing", "-", `{"server-port": "{{server.build.default.port}}", "motd": "{{env.MOTD}}"}`, "server-port=25565\nmotd=Atlas: a \\\\ test\n"},
		{"empty file", "", `{"a": "1"}`, "a=1\n"},
		{"unknown placeholder is empty", "a=1\n", `{"a": "{{nope}}"}`, "a=\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			if tc.input != "-" {
				writeTestFile(t, root, "config/app.properties", tc.input)
			}
			mustApply(t, root, config("config/app.properties", "properties", tc.find))
			if got := readTestFile(t, root, "config/app.properties"); got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
			assertNoTempFiles(t, root)
		})
	}
}

func TestINI(t *testing.T) {
	cases := []struct {
		name  string
		input string
		find  string
		want  string
	}{
		{
			"update keeps spacing and other sections",
			"; main\n[server]\nport = 1\nname = x\n\n[other]\nport = 2\n",
			`{"server.port": "{{server.build.default.port}}"}`,
			"; main\n[server]\nport = 25565\nname = x\n\n[other]\nport = 2\n",
		},
		{
			"key added to existing section",
			"[server]\nport = 1\n\n[other]\nport = 2\n",
			`{"server.name": "atlas"}`,
			"[server]\nport = 1\nname = atlas\n\n[other]\nport = 2\n",
		},
		{
			"key added to last section",
			"[server]\nport=1\n",
			`{"server.name": "atlas"}`,
			"[server]\nport=1\nname=atlas\n",
		},
		{
			"section created",
			"[a]\nx=1\n",
			`{"b.y": "2", "b.z": 3}`,
			"[a]\nx=1\n\n[b]\ny=2\nz=3\n",
		},
		{
			"global key updated",
			"name=x\n[a]\nname=1\n",
			`{"name": "y"}`,
			"name=y\n[a]\nname=1\n",
		},
		{
			"global key added before first section",
			"[a]\nk=1\n",
			`{"top": "1"}`,
			"top=1\n\n[a]\nk=1\n",
		},
		{
			"global key added after global content",
			"# header\nfirst=1\n\n[a]\nk=1\n",
			`{"second": "2"}`,
			"# header\nfirst=1\nsecond=2\n\n[a]\nk=1\n",
		},
		{
			"dotted global key",
			"server.port=1\n",
			`{"server.port": "2"}`,
			"server.port=2\n",
		},
		{
			"section names with dots",
			"[/Script/Engine.GameSession]\nMaxPlayers=10\n\n[/Script/Engine]\nGameSession.MaxPlayers=1\n",
			`{"/Script/Engine.GameSession.MaxPlayers": "{{env.MAX_PLAYERS}}"}`,
			"[/Script/Engine.GameSession]\nMaxPlayers=20\n\n[/Script/Engine]\nGameSession.MaxPlayers=1\n",
		},
		{
			"comments and unrelated lines kept",
			"; comment\n# another\n[s]\n; port=0\nport=1\njunk line\n",
			`{"s.port": "2"}`,
			"; comment\n# another\n[s]\n; port=0\nport=2\njunk line\n",
		},
		{
			"quoted value stays quoted",
			"[s]\nServerName=\"Old Name\"\n",
			`{"s.ServerName": "{{env.LEVEL}}"}`,
			"[s]\nServerName=\"world\"\n",
		},
		{
			"case insensitive",
			"[Server]\nPort=1\n",
			`{"server.port": "2"}`,
			"[Server]\nPort=2\n",
		},
		{
			"crlf kept",
			"[s]\r\nport=1\r\n",
			`{"s.port": "2", "s.name": "n"}`,
			"[s]\r\nport=2\r\nname=n\r\n",
		},
		{
			"bare key gets a value",
			"[s]\nenabled\n",
			`{"s.enabled": true}`,
			"[s]\nenabled=true\n",
		},
		{
			"created when missing",
			"-",
			`{"name": "x", "server.port": "{{server.build.default.port}}", "server.ip": "{{server.build.default.ip}}"}`,
			"name=x\n\n[server]\nport=25565\nip=0.0.0.0\n",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			if tc.input != "-" {
				writeTestFile(t, root, "game.ini", tc.input)
			}
			mustApply(t, root, config("game.ini", "ini", tc.find))
			if got := readTestFile(t, root, "game.ini"); got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestJSONDocument(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "config.json", `{
  "zeta": 1,
  "server": {
    "port": 1234,
    "ip": "127.0.0.1",
    "online": false,
    "name": "old",
    "html": "x"
  },
  "alpha": [
    {"host": "127.0.0.1", "port": 1},
    {"host": "10.0.0.1", "port": 2}
  ]
}
`)
	mustApply(t, root, config("config.json", "json", `{
		"server.port": "{{server.build.default.port}}",
		"server.ip": "{{server.build.default.ip}}",
		"server.online": "{{env.ONLINE_MODE}}",
		"server.name": "{{env.MAX_PLAYERS}}",
		"server.html": "<b> & \"q\"",
		"server.memory": "{{server.build.memory}}",
		"server.label": "Port {{server.build.default.port}}",
		"server.literal": "25565",
		"server.flag": true,
		"server.ratio": 1.5,
		"alpha[1].port": 99,
		"alpha.*.host": {"127.0.0.1": "{{config.docker.interface}}"},
		"alpha.*.missing.*.x": "never created",
		"new.deep.key": "x",
		"list[0].name": "first"
	}`))

	want := `{
  "zeta": 1,
  "server": {
    "port": 25565,
    "ip": "0.0.0.0",
    "online": true,
    "name": "20",
    "html": "<b> & \"q\"",
    "memory": 2048,
    "label": "Port 25565",
    "literal": "25565",
    "flag": true,
    "ratio": 1.5
  },
  "alpha": [
    {
      "host": "172.18.0.1",
      "port": 1
    },
    {
      "host": "10.0.0.1",
      "port": 99
    }
  ],
  "new": {
    "deep": {
      "key": "x"
    }
  },
  "list": [
    {
      "name": "first"
    }
  ]
}
`
	if got := readTestFile(t, root, "config.json"); got != want {
		t.Errorf("config.json =\n%s\nwant\n%s", got, want)
	}
}

func TestJSON(t *testing.T) {
	cases := []struct {
		name  string
		input string
		find  string
		want  string
	}{
		{"created with four spaces", "-", `{"a.b": "{{server.build.default.port}}", "a.c": "x"}`, "{\n    \"a\": {\n        \"b\": 25565,\n        \"c\": \"x\"\n    }\n}\n"},
		{"empty file", "", `{"a": true}`, "{\n    \"a\": true\n}\n"},
		{"tabs kept", "{\n\t\"a\": 1\n}\n", `{"a": 2}`, "{\n\t\"a\": 2\n}\n"},
		{"crlf and no final newline kept", "{\r\n  \"a\": 1\r\n}", `{"a": 2}`, "{\r\n  \"a\": 2\r\n}"},
		{"compact kept", `{"a":1,"b":{"c":"x"},"d":[1,2]}`, `{"b.c": "y"}`, `{"a":1,"b":{"c":"y"},"d":[1,2]}`},
		{"unchanged file is not reformatted", "{\"a\":   1,\n\n \"b\": \"x\"}", `{"a": "1", "b": "x"}`, "{\"a\":   1,\n\n \"b\": \"x\"}"},
		{"number stays number", `{"port": 1}`, `{"port": "{{server.build.default.port}}"}`, `{"port":25565}`},
		{"float replaced by integer", `{"port": 1.5}`, `{"port": "{{server.build.default.port}}"}`, `{"port":25565}`},
		{"string stays string", `{"port": "1"}`, `{"port": "{{server.build.default.port}}"}`, `{"port":"25565"}`},
		{"number becomes text when not numeric", `{"port": 1}`, `{"port": "{{server.build.default.ip}}"}`, `{"port":"0.0.0.0"}`},
		{"bool stays bool", `{"on": false}`, `{"on": "{{env.ONLINE_MODE}}"}`, `{"on":true}`},
		{"null takes the guessed type", `{"port": null}`, `{"port": "{{server.build.default.port}}"}`, `{"port":25565}`},
		{"literal string on missing key stays text", `{"x": 1}`, `{"port": "25565", "on": "true"}`, `{"x":1,"port":"25565","on":"true"}`},
		{"leading zero is text", `{"x": 1}`, `{"x": "007"}`, `{"x":"007"}`},
		{"native null", `{"x": 1}`, `{"x": null}`, `{"x":null}`},
		{"numeric segment indexes arrays", `{"a": [{"b": 1}]}`, `{"a.0.b": 2}`, `{"a":[{"b":2}]}`},
		{"array extended by one", `{"a": [1]}`, `{"a[1]": 2}`, `{"a":[1,2]}`},
		{"root array", `[{"n": 1}, {"n": 2}]`, `{"[1].n": 5, "*.m": true}`, `[{"n":1,"m":true},{"n":5,"m":true}]`},
		{"wildcard over object", `{"s": {"a": {"p": 1}, "b": {"p": 2}, "c": 3}}`, `{"s.*.p": 9}`, `{"s":{"a":{"p":9},"b":{"p":9},"c":3}}`},
		{"bracket wildcard", `{"s": [{"p": 1}, {"p": 2}]}`, `{"s[*].p": 9}`, `{"s":[{"p":9},{"p":9}]}`},
		{"escaped dot", `{"a.b": 1, "a": {"b": 2}}`, `{"a\\.b": 5}`, `{"a.b":5,"a":{"b":2}}`},
		{"conditional equal", `{"h": "127.0.0.1"}`, `{"h": {"127.0.0.1": "{{server.build.default.ip}}"}}`, `{"h":"0.0.0.0"}`},
		{"conditional contains", `{"h": "localhost:25565"}`, `{"h": {"127.0.0.1": "a", "localhost": "{{config.docker.interface}}"}}`, `{"h":"172.18.0.1:25565"}`},
		{"conditional first match wins", `{"h": "ab"}`, `{"h": {"b": "1", "a": "2"}}`, `{"h":"a1"}`},
		{"conditional no match", `{"h":"other"}`, `{"h": {"127.0.0.1": "x"}}`, `{"h":"other"}`},
		{"conditional never creates", `{"x":1}`, `{"h": {"a": "b"}, "y.z": {"a": "b"}}`, `{"x":1}`},
		{"conditional on number", `{"p": 25565}`, `{"p": {"25565": "{{server.build.memory}}"}}`, `{"p":2048}`},
		{"conditional regex", `{"h": "localhost:25565"}`, `{"h": {"regex:^(127\\.0\\.0\\.1|localhost)(:\\d{1,5})?$": "{{config.docker.interface}}$2"}}`, `{"h":"172.18.0.1:25565"}`},
		{"object replaced by scalar", `{"a": {"b": 1}}`, `{"a": "x"}`, `{"a":"x"}`},
		{"unicode and escapes round trip", `{"a": "é\n\"q\"\\", "b": 1}`, `{"b": 2}`, `{"a":"é\n\"q\"\\","b":2}`},
		{"large numbers untouched", `{"n": 12345678901234567890, "f": 1.0e10, "b": 1}`, `{"b": 2}`, `{"n":12345678901234567890,"f":1.0e10,"b":2}`},
		{"bom kept", "\xef\xbb\xbf{\"a\":1}", `{"a": 2}`, "\xef\xbb\xbf{\"a\":2}"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			if tc.input != "-" {
				writeTestFile(t, root, "data/config.json", tc.input)
			}
			mustApply(t, root, config("data/config.json", "json", tc.find))
			if got := readTestFile(t, root, "data/config.json"); got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestJSONErrorsLeaveFileUntouched(t *testing.T) {
	cases := []struct{ name, input, find string }{
		{"invalid json", `{"a": 1,}`, `{"a": 2}`},
		{"comments", "// c\n{\"a\": 1}", `{"a": 2}`},
		{"trailing data", `{"a": 1} {"b": 2}`, `{"a": 2}`},
		{"scalar in the way", `{"a": 5}`, `{"ok": 1, "a.b": 2}`},
		{"array in the way", `{"a": [1]}`, `{"a.b": 2}`},
		{"index on object", `{"a": {"b": 1}}`, `{"a[0]": 2}`},
		{"index out of range", `{"a": []}`, `{"a[2]": 2}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			writeTestFile(t, root, "config.json", tc.input)
			errs := Apply(root, mustParse(t, config("config.json", "json", tc.find)), testValues)
			if len(errs) != 1 {
				t.Fatalf("errors = %v, want exactly one", errs)
			}
			if !strings.Contains(errs[0].Error(), "config.json") {
				t.Errorf("error %q does not name the file", errs[0])
			}
			if got := readTestFile(t, root, "config.json"); got != tc.input {
				t.Errorf("file changed to %q", got)
			}
			assertNoTempFiles(t, root)
		})
	}
}

const bungeeConfig = `# BungeeCord configuration
server_connect_timeout: 5000
listeners:
- query_port: 25577
  motd: '&1Another Bungee server'
  query_enabled: false
  host: 0.0.0.0:25577
  max_players: 1
  force_default_server: false
ip_forward: false
# Backend servers
servers:
  lobby:
    motd: '&1Just another BungeeCord - Forced Host'
    address: localhost:25565
    restricted: false
  survival:
    address: 127.0.0.1:25566 # game
    restricted: false
  bare:
    address: 127.0.0.1
    restricted: false
  remote:
    address: 10.1.2.3:25565
    restricted: true
player_limit: -1
online_mode: "false"
`

func TestYAMLProxy(t *testing.T) {
	finds := map[string]string{
		"wings regex": `{"regex:^(127\\.0\\.0\\.1|localhost)(:\\d{1,5})?$": "{{config.docker.interface}}$2"}`,
		"plain text":  `{"127.0.0.1": "{{config.docker.interface}}", "localhost": "{{config.docker.interface}}"}`,
	}
	for name, conditional := range finds {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			writeTestFile(t, root, "config.yml", bungeeConfig)
			mustApply(t, root, config("config.yml", "yaml", `{
				"listeners[0].query_port": "{{server.build.default.port}}",
				"listeners[0].host": "0.0.0.0:{{server.build.default.port}}",
				"listeners[0].max_players": "{{env.MAX_PLAYERS}}",
				"ip_forward": "{{env.ONLINE_MODE}}",
				"online_mode": "{{env.ONLINE_MODE}}",
				"servers.*.address": `+conditional+`
			}`))
			got := readTestFile(t, root, "config.yml")

			for _, want := range []string{
				"# BungeeCord configuration\n",
				"# Backend servers\n",
				"# game\n",
				"query_port: 25565\n",
				"host: 0.0.0.0:25565\n",
				"max_players: 20\n",
				"motd: '&1Another Bungee server'\n",
				"ip_forward: true\n",
				"online_mode: \"true\"\n",
				"address: 172.18.0.1:25565\n",
				"address: 172.18.0.1:25566 # game\n",
				"address: 172.18.0.1\n",
				"address: 10.1.2.3:25565\n",
				"player_limit: -1\n",
			} {
				if !strings.Contains(got, want) {
					t.Errorf("output lacks %q:\n%s", want, got)
				}
			}
			for _, gone := range []string{"localhost", "127.0.0.1", "25577"} {
				if strings.Contains(got, gone) {
					t.Errorf("output still contains %q:\n%s", gone, got)
				}
			}

			// Top-level and nested key order is unchanged.
			var before, after yaml.Node
			if err := yaml.Unmarshal([]byte(bungeeConfig), &before); err != nil {
				t.Fatal(err)
			}
			if err := yaml.Unmarshal([]byte(got), &after); err != nil {
				t.Fatalf("output is not valid YAML: %v\n%s", err, got)
			}
			if b, a := yamlKeys(&before), yamlKeys(&after); !reflect.DeepEqual(b, a) {
				t.Errorf("key order changed:\n%v\n%v", b, a)
			}

			var decoded struct {
				Listeners []struct {
					QueryPort  any `yaml:"query_port"`
					MaxPlayers any `yaml:"max_players"`
				}
				IPForward  any `yaml:"ip_forward"`
				OnlineMode any `yaml:"online_mode"`
			}
			if err := yaml.Unmarshal([]byte(got), &decoded); err != nil {
				t.Fatal(err)
			}
			if decoded.Listeners[0].QueryPort != 25565 || decoded.Listeners[0].MaxPlayers != 20 {
				t.Errorf("ports are not integers: %#v", decoded.Listeners[0])
			}
			if decoded.IPForward != true || decoded.OnlineMode != "true" {
				t.Errorf("ip_forward = %#v, online_mode = %#v", decoded.IPForward, decoded.OnlineMode)
			}

			// Idempotent.
			mustApply(t, root, config("config.yml", "yaml", `{"listeners[0].query_port": "{{server.build.default.port}}", "servers.*.address": `+conditional+`}`))
			if again := readTestFile(t, root, "config.yml"); again != got {
				t.Errorf("second run changed the file:\n%s", again)
			}
		})
	}
}

// yamlKeys lists every mapping key of a document in order, as dotted paths.
func yamlKeys(node *yaml.Node) []string {
	var keys []string
	var visit func(node *yaml.Node, prefix string)
	visit = func(node *yaml.Node, prefix string) {
		switch node.Kind {
		case yaml.DocumentNode, yaml.SequenceNode:
			for _, child := range node.Content {
				visit(child, prefix)
			}
		case yaml.MappingNode:
			for i := 0; i+1 < len(node.Content); i += 2 {
				key := prefix + node.Content[i].Value
				keys = append(keys, key)
				visit(node.Content[i+1], key+".")
			}
		}
	}
	visit(node, "")
	return keys
}

func TestYAML(t *testing.T) {
	cases := []struct {
		name  string
		input string
		find  string
		want  string
	}{
		{
			"created when missing", "-",
			`{"network.port": "{{server.build.default.port}}", "network.host": "{{server.build.default.ip}}", "debug": false, "name": "5", "motd": "{{env.MOTD}}"}`,
			"network:\n  port: 25565\n  host: 0.0.0.0\ndebug: false\nname: \"5\"\nmotd: 'Atlas: a \\ test'\n",
		},
		{
			"velocity style", "# Velocity\nbind: 0.0.0.0:25577\nshow-max-players: 500\nonline-mode: true\n",
			`{"bind": "0.0.0.0:{{server.build.default.port}}", "show-max-players": "{{env.MAX_PLAYERS}}", "online-mode": false}`,
			"# Velocity\nbind: 0.0.0.0:25565\nshow-max-players: 20\nonline-mode: false\n",
		},
		{
			"comments kept", "# head\na: 1 # line\n# before b\nb:\n  c: x # deep\n",
			`{"a": 2, "b.c": "y", "b.d": "z"}`,
			"# head\na: 2 # line\n# before b\nb:\n  c: y # deep\n  d: z\n",
		},
		{
			"quoted string stays quoted", "port: \"1\"\nname: 'x'\n",
			`{"port": "{{server.build.default.port}}", "name": "y"}`,
			"port: \"25565\"\nname: 'y'\n",
		},
		{
			"number replaced by text", "port: 1\n",
			`{"port": "{{server.build.default.ip}}"}`,
			"port: 0.0.0.0\n",
		},
		{
			"empty value filled", "port:\nlisteners:\n",
			`{"port": "{{server.build.default.port}}", "listeners[0].host": "{{server.build.default.ip}}"}`,
			"port: 25565\nlisteners:\n  - host: 0.0.0.0\n",
		},
		{
			"four space indent kept", "a:\n    b: 1\n",
			`{"a.b": 2, "a.c.d": 3}`,
			"a:\n    b: 2\n    c:\n        d: 3\n",
		},
		{
			"crlf kept", "a: 1\r\nb: 2\r\n",
			`{"b": 3}`,
			"a: 1\r\nb: 3\r\n",
		},
		{
			"unchanged file is not reformatted", "a:   1\n\n\nb: [ 1,2 ]\n",
			`{"a": "1"}`,
			"a:   1\n\n\nb: [ 1,2 ]\n",
		},
		{
			"comment only file", "# just a comment\n",
			`{"a": 1}`,
			"# just a comment\na: 1\n",
		},
		{
			"multiple documents", "a: 1\n---\nb: 2\n",
			`{"a": 5}`,
			"a: 5\n---\nb: 2\n",
		},
		{
			"wildcard creates under children", "s:\n  x:\n    p: 1\n  y:\n    q: 2\n",
			`{"s.*.p": 9}`,
			"s:\n  x:\n    p: 9\n  y:\n    q: 2\n    p: 9\n",
		},
		{
			"conditional never creates", "a: 1\n",
			`{"b": {"x": "y"}, "c.*.d": "z"}`,
			"a: 1\n",
		},
		{
			"anchors edited through alias", "base: &b\n  port: 1\nother: *b\n",
			`{"other.port": 2}`,
			"base: &b\n  port: 2\nother: *b\n",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			if tc.input != "-" {
				writeTestFile(t, root, "config.yml", tc.input)
			}
			mustApply(t, root, config("config.yml", "yaml", tc.find))
			if got := readTestFile(t, root, "config.yml"); got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestYAMLErrorsLeaveFileUntouched(t *testing.T) {
	cases := []struct{ name, input, find string }{
		{"invalid yaml", "a: [1, 2\nb: {\n", `{"a": 2}`},
		{"tabs", "a:\n\t- b\n", `{"a": 2}`},
		{"scalar in the way", "a: 5\n", `{"a.b": 2}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			writeTestFile(t, root, "config.yml", tc.input)
			errs := Apply(root, mustParse(t, config("config.yml", "yaml", tc.find)), testValues)
			if len(errs) != 1 {
				t.Fatalf("errors = %v, want exactly one", errs)
			}
			if got := readTestFile(t, root, "config.yml"); got != tc.input {
				t.Errorf("file changed to %q", got)
			}
		})
	}
}

const sevenDaysConfig = `<?xml version="1.0"?>
<ServerSettings>
	<!-- GENERAL SERVER SETTINGS -->

	<!-- Server representation -->
	<property name="ServerName"					value="My Game Host"/>			<!-- Whatever you want the name of the server to be. -->
	<property name="ServerPort"					value="26900"/>					<!-- Port you want the server to listen on. -->
	<property name="ServerMaxPlayerCount"		value='8' />
	<property name="ServerPassword"				value=""/>
	<property name="TelnetEnabled"				value="true"/>
	<property name="TelnetPort"					value="8081"/>
	<!-- <property name="ServerPort" value="1"/> -->
</ServerSettings>
`

func TestXMLSevenDaysToDie(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "serverconfig.xml", sevenDaysConfig)
	raw := config("serverconfig.xml", "xml", `{
		"ServerSettings.property[@name='ServerPort']": "{{server.build.default.port}}",
		"//property[@name='ServerName']": "{{env.SERVER_NAME}}",
		"/ServerSettings/property[@name='ServerMaxPlayerCount']/@value": "{{env.MAX_PLAYERS}}",
		"property[@name='TelnetEnabled']": false,
		"property[@name=\"TelnetPort\"].@value": 8082,
		"property[@name='DoesNotExist']": "ignored"
	}`)
	mustApply(t, root, raw)

	want := sevenDaysConfig
	for _, r := range [][2]string{
		{`value="26900"/>`, `value="25565"/>`},
		{`value="My Game Host"/>`, `value="Atlas &lt;Test> &amp; &quot;Co&quot;"/>`},
		{`value='8' />`, `value='20' />`},
		{`value="true"/>`, `value="false"/>`},
		{`value="8081"/>`, `value="8082"/>`},
	} {
		if !strings.Contains(want, r[0]) {
			t.Fatalf("fixture lacks %q", r[0])
		}
		want = strings.Replace(want, r[0], r[1], 1)
	}
	if got := readTestFile(t, root, "serverconfig.xml"); got != want {
		t.Errorf("serverconfig.xml =\n%s\nwant\n%s", got, want)
	}

	mustApply(t, root, raw)
	if got := readTestFile(t, root, "serverconfig.xml"); got != want {
		t.Errorf("second run changed the file:\n%s", got)
	}
}

func TestXML(t *testing.T) {
	const doc = "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<config>\n  <port>1</port>\n  <name/>\n  <server host=\"a\" />\n  <nested>\n    <port>2</port>\n  </nested>\n  <item>one</item>\n  <item>two</item>\n</config>\n"
	cases := []struct {
		name  string
		input string
		find  string
		edits [][2]string // replacements applied to input to get the expected output
	}{
		{"element text by dotted path", doc, `{"config.port": "{{server.build.default.port}}"}`, [][2]string{{"<port>1</port>", "<port>25565</port>"}}},
		{"element text by slash path", doc, `{"/config/nested/port": 7}`, [][2]string{{"<port>2</port>", "<port>7</port>"}}},
		{"descendants", doc, `{"//port": "9"}`, [][2]string{{"<port>1</port>", "<port>9</port>"}, {"<port>2</port>", "<port>9</port>"}}},
		{"relative path found at depth", doc, `{"nested.port": "9"}`, [][2]string{{"<port>2</port>", "<port>9</port>"}}},
		{"wildcard step", doc, `{"/config/*/port": "9"}`, [][2]string{{"<port>2</port>", "<port>9</port>"}}},
		{"self closing element gets text", doc, `{"config/name": "{{env.SERVER_NAME}}"}`, [][2]string{{"<name/>", "<name>Atlas &lt;Test&gt; &amp; \"Co\"</name>"}}},
		{"self closing element left alone for empty text", doc, `{"config/name": ""}`, nil},
		{"position", doc, `{"/config/item[2]": "second"}`, [][2]string{{"<item>two</item>", "<item>second</item>"}}},
		{"attribute path", doc, `{"/config/server/@host": "{{server.build.default.ip}}"}`, [][2]string{{`host="a"`, `host="0.0.0.0"`}}},
		{"attribute added", doc, `{"config.server.@port": "{{server.build.default.port}}"}`, [][2]string{{`<server host="a" />`, `<server host="a" port="25565" />`}}},
		{"wings attribute value", doc, `{"config.server": "[host='{{server.build.default.ip}}']"}`, [][2]string{{`host="a"`, `host="0.0.0.0"`}}},
		{"has-attribute predicate", doc, `{"//*[@host]/@host": "b"}`, [][2]string{{`host="a"`, `host="b"`}}},
		{"predicate without value attribute sets text", "<a><b id=\"x\">old</b><b id=\"y\">keep</b></a>", `{"a.b[@id='x']": "new"}`, [][2]string{{">old<", ">new<"}}},
		{"namespace prefix optional", "<ns:a xmlns:ns=\"urn:x\"><ns:port>1</ns:port></ns:a>", `{"a.port": "2", "/ns:a/@ns:mode": "m"}`, [][2]string{{">1<", ">2<"}, {`xmlns:ns="urn:x"`, `xmlns:ns="urn:x" ns:mode="m"`}}},
		{"no match changes nothing", doc, `{"/config/missing": "x", "/port": "x", "/other/port": "x"}`, nil},
		{"crlf and comments kept", "<a>\r\n  <!-- c -->\r\n  <b>1</b>\r\n</a>\r\n", `{"a.b": "2"}`, [][2]string{{"<b>1</b>", "<b>2</b>"}}},
		{"cdata replaced", "<a><b><![CDATA[x < y]]></b></a>", `{"a.b": "z"}`, [][2]string{{"<![CDATA[x < y]]>", "z"}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			writeTestFile(t, root, "conf/settings.xml", tc.input)
			mustApply(t, root, config("conf/settings.xml", "xml", tc.find))
			want := tc.input
			for _, edit := range tc.edits {
				if !strings.Contains(want, edit[0]) {
					t.Fatalf("fixture lacks %q", edit[0])
				}
				want = strings.Replace(want, edit[0], edit[1], 1)
			}
			if got := readTestFile(t, root, "conf/settings.xml"); got != want {
				t.Errorf("got\n%s\nwant\n%s", got, want)
			}
		})
	}
}

func TestXMLErrorsLeaveFileUntouched(t *testing.T) {
	cases := []struct{ name, input, find string }{
		{"mismatched tags", "<a><b></a>", `{"a.b": "x"}`},
		{"unclosed", "<a><b>1</b>", `{"a.b": "x"}`},
		{"two roots", "<a/><b/>", `{"a": "x"}`},
		{"not xml", "port=1\n", `{"a": "x"}`},
		{"element with children", "<a><b><c/></b><d>1</d></a>", `{"a.d": "2", "a.b": "x"}`},
		{"unsupported encoding", "<?xml version=\"1.0\" encoding=\"UTF-16\"?><a><b>1</b></a>", `{"a.b": "x"}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			writeTestFile(t, root, "settings.xml", tc.input)
			errs := Apply(root, mustParse(t, config("settings.xml", "xml", tc.find)), testValues)
			if len(errs) != 1 {
				t.Fatalf("errors = %v, want exactly one", errs)
			}
			if got := readTestFile(t, root, "settings.xml"); got != tc.input {
				t.Errorf("file changed to %q", got)
			}
			assertNoTempFiles(t, root)
		})
	}
}

func TestFileParser(t *testing.T) {
	cases := []struct {
		name  string
		input string
		find  string
		want  string
	}{
		{"line replaced", "port=1\n# port=2\nname=x\n", `{"port=": "port={{server.build.default.port}}"}`, "port=25565\n# port=2\nname=x\n"},
		{"every matching line", "set a 1\nset b 2\nset a 3\n", `{"set a": "set a 9"}`, "set a 9\nset b 2\nset a 9\n"},
		{"appended when missing", "name=x\n", `{"port=": "port={{server.build.default.port}}"}`, "name=x\nport=25565\n"},
		{"appended without final newline", "name=x", `{"port=": "port=1"}`, "name=x\nport=1\n"},
		{"not appended twice", "name=x\nvalue\n", `{"missing": "value"}`, "name=x\nvalue\n"},
		{"applied in order", "port=1\n", `{"port": "port=2", "port=2": "port=2 # set"}`, "port=2 # set\n"},
		{"prefix must be at line start", " port=1\n", `{"port": "port=2"}`, " port=1\nport=2\n"},
		{"crlf kept", "a=1\r\nb=2\r\n", `{"b=": "b=3", "c=": "c=4"}`, "a=1\r\nb=3\r\nc=4\r\n"},
		{"numbers and booleans", "x\ny\n", `{"x": 5, "y": true}`, "5\ntrue\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			writeTestFile(t, root, "server.cfg", tc.input)
			raw := config("server.cfg", "file", tc.find)
			mustApply(t, root, raw)
			if got := readTestFile(t, root, "server.cfg"); got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
			mustApply(t, root, raw)
			if got := readTestFile(t, root, "server.cfg"); got != tc.want {
				t.Errorf("second run: got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestMissingFiles(t *testing.T) {
	root := t.TempDir()
	mustApply(t, root, `{
		"a/b/server.properties": {"parser": "properties", "find": {"k": "v"}},
		"a/game.ini": {"parser": "ini", "find": {"s.k": "v"}},
		"c/config.json": {"parser": "json", "find": {"k": "v"}},
		"c/d/config.yml": {"parser": "yaml", "find": {"k": "v"}},
		"x/settings.xml": {"parser": "xml", "find": {"k": "v"}},
		"x/server.cfg": {"parser": "file", "find": {"k": "v"}},
		"y/empty.properties": {"parser": "properties", "find": {}}
	}`)
	for _, created := range []string{"a/b/server.properties", "a/game.ini", "c/config.json", "c/d/config.yml"} {
		if !exists(root, created) {
			t.Errorf("%s was not created", created)
		}
	}
	for _, skipped := range []string{"x", "y"} {
		if exists(root, skipped) {
			t.Errorf("%s should not exist", skipped)
		}
	}

	if runtime.GOOS != "windows" {
		for path, want := range map[string]os.FileMode{"a": 0o770, "a/b": 0o770, "c/d": 0o770, "a/b/server.properties": 0o660, "c/config.json": 0o660} {
			info, err := os.Stat(filepath.Join(root, filepath.FromSlash(path)))
			if err != nil {
				t.Fatal(err)
			}
			if got := info.Mode().Perm(); got != want {
				t.Errorf("%s mode = %04o, want %04o", path, got, want)
			}
		}
	}
	assertNoTempFiles(t, root)
}

func TestExistingModeKept(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX permissions are not available on Windows")
	}
	root := t.TempDir()
	writeTestFile(t, root, "server.properties", "a=1\n")
	path := filepath.Join(root, "server.properties")
	if err := os.Chmod(path, 0o640); err != nil {
		t.Fatal(err)
	}
	mustApply(t, root, config("server.properties", "properties", `{"a": "2"}`))
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o640 {
		t.Errorf("mode = %04o, want 0640", got)
	}
}

func TestPathSecurity(t *testing.T) {
	parent := t.TempDir()
	root := filepath.Join(parent, "server")
	if err := os.Mkdir(root, 0o770); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, parent, "outside.properties", "a=1\n")
	writeTestFile(t, root, "dir/keep.txt", "x")

	bad := []string{
		"../outside.properties",
		"a/../../outside.properties",
		"dir/../../outside.properties",
		`..\outside.properties`,
		`dir\..\..\outside.properties`,
		"..",
		".",
		"",
		"   ",
		"/etc/atlas-test.properties",
		`\outside.properties`,
		`C:\outside.properties`,
		"C:outside.properties",
		filepath.Join(parent, "outside.properties"),
		"dir",
		"a\x00b",
	}
	for _, path := range bad {
		files := []File{{Path: path, Parser: "properties", Find: []Replacement{{"a", "2"}}}}
		if errs := Apply(root, files, testValues); len(errs) != 1 {
			t.Errorf("Apply(%q) errors = %v, want exactly one", path, errs)
		}
	}
	if got := readTestFile(t, parent, "outside.properties"); got != "a=1\n" {
		t.Errorf("file outside the root was modified: %q", got)
	}
	entries, err := os.ReadDir(parent)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Errorf("unexpected entries next to the root: %v", entries)
	}

	// Parse accepts the document; the path is rejected per file at Apply time.
	files := mustParse(t, `{
		"../outside.properties": {"parser": "properties", "find": {"a": "2"}},
		"ok.properties": {"parser": "properties", "find": {"a": "2"}}
	}`)
	errs := Apply(root, files, testValues)
	if len(errs) != 1 || !strings.Contains(errs[0].Error(), "../outside.properties") {
		t.Errorf("errors = %v, want one naming the bad path", errs)
	}
	if got := readTestFile(t, root, "ok.properties"); got != "a=2\n" {
		t.Errorf("valid file was not applied: %q", got)
	}
}

func TestSymlinks(t *testing.T) {
	parent := t.TempDir()
	root := filepath.Join(parent, "server")
	outside := filepath.Join(parent, "outside")
	for _, dir := range []string{root, outside, filepath.Join(root, "real")} {
		if err := os.Mkdir(dir, 0o770); err != nil {
			t.Fatal(err)
		}
	}
	writeTestFile(t, outside, "server.properties", "a=1\n")
	writeTestFile(t, root, "real/server.properties", "a=1\n")

	if err := os.Symlink(outside, filepath.Join(root, "escape")); err != nil {
		t.Skipf("symlinks are not available: %v", err)
	}
	links := map[string]string{
		"inside":       filepath.Join(root, "real"),
		"file-escape":  filepath.Join(outside, "server.properties"),
		"dangling":     filepath.Join(outside, "does-not-exist.properties"),
		"file-inside":  filepath.Join(root, "real", "server.properties"),
		"deep-missing": filepath.Join(outside, "nope"),
	}
	for name, target := range links {
		if err := os.Symlink(target, filepath.Join(root, name)); err != nil {
			t.Skipf("symlinks are not available: %v", err)
		}
	}

	find := []Replacement{{"a", "2"}}
	for _, path := range []string{
		"escape/server.properties",
		"escape/new.properties",
		"escape/sub/dir/new.properties",
		"file-escape",
		"dangling",
		"deep-missing/new.properties",
	} {
		if errs := Apply(root, []File{{Path: path, Parser: "properties", Find: find}}, testValues); len(errs) != 1 {
			t.Errorf("Apply(%q) errors = %v, want exactly one", path, errs)
		}
	}
	if got := readTestFile(t, outside, "server.properties"); got != "a=1\n" {
		t.Errorf("file outside the root was modified: %q", got)
	}
	entries, err := os.ReadDir(outside)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Errorf("files were created outside the root: %v", entries)
	}

	// Symlinks that stay inside the root are followed.
	if errs := Apply(root, []File{{Path: "inside/server.properties", Parser: "properties", Find: find}}, testValues); errs != nil {
		t.Fatalf("Apply through an inner symlink: %v", errs)
	}
	if got := readTestFile(t, root, "real/server.properties"); got != "a=2\n" {
		t.Errorf("real/server.properties = %q", got)
	}
	if errs := Apply(root, []File{{Path: "file-inside", Parser: "properties", Find: []Replacement{{"a", "3"}}}}, testValues); errs != nil {
		t.Fatalf("Apply through an inner file symlink: %v", errs)
	}
	if got := readTestFile(t, root, "real/server.properties"); got != "a=3\n" {
		t.Errorf("real/server.properties = %q", got)
	}
	if info, err := os.Lstat(filepath.Join(root, "file-inside")); err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Errorf("the symlink itself was replaced: %v %v", info, err)
	}
}

// Junctions need no privilege on Windows and are not resolved by
// filepath.EvalSymlinks, so they are refused outright.
func TestWindowsJunctions(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("junctions are a Windows feature")
	}
	parent := t.TempDir()
	root := filepath.Join(parent, "server")
	outside := filepath.Join(parent, "outside")
	for _, dir := range []string{root, outside} {
		if err := os.Mkdir(dir, 0o770); err != nil {
			t.Fatal(err)
		}
	}
	writeTestFile(t, outside, "server.properties", "a=1\n")
	if out, err := exec.Command("cmd", "/c", "mklink", "/J", filepath.Join(root, "escape"), outside).CombinedOutput(); err != nil {
		t.Skipf("cannot create a junction: %v: %s", err, out)
	}

	for _, path := range []string{"escape/server.properties", "escape/new.properties", "escape/sub/new.properties"} {
		files := []File{{Path: path, Parser: "properties", Find: []Replacement{{"a", "2"}}}}
		if errs := Apply(root, files, testValues); len(errs) != 1 {
			t.Errorf("Apply(%q) errors = %v, want exactly one", path, errs)
		}
	}
	if got := readTestFile(t, outside, "server.properties"); got != "a=1\n" {
		t.Errorf("file outside the root was modified: %q", got)
	}
	entries, err := os.ReadDir(outside)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Errorf("files were created outside the root: %v", entries)
	}
}

func TestApplyReportsEachFailure(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "broken.json", "{")
	writeTestFile(t, root, "broken.xml", "<a>")
	writeTestFile(t, root, "good.properties", "a=1\n")

	files := mustParse(t, `{
		"broken.json": {"parser": "json", "find": {"a": 1}},
		"good.properties": {"parser": "properties", "find": {"a": "2"}},
		"broken.xml": {"parser": "xml", "find": {"a": "x"}},
		"new.yml": {"parser": "yaml", "find": {"a": "{{server.uuid}}"}}
	}`)
	errs := Apply(root, files, testValues)
	if len(errs) != 2 {
		t.Fatalf("errors = %v, want two", errs)
	}
	if !strings.Contains(errs[0].Error(), "broken.json") || !strings.Contains(errs[1].Error(), "broken.xml") {
		t.Errorf("errors do not name their files: %v", errs)
	}
	if got := readTestFile(t, root, "good.properties"); got != "a=2\n" {
		t.Errorf("good.properties = %q", got)
	}
	if got := readTestFile(t, root, "new.yml"); got != "a: 3f2a9c1e-0000-4000-8000-000000000001\n" {
		t.Errorf("new.yml = %q", got)
	}

	if errs := Apply(root, nil, testValues); errs != nil {
		t.Errorf("Apply with no files = %v", errs)
	}
	if errs := Apply(filepath.Join(root, "missing-root"), files[1:2], testValues); len(errs) != 1 {
		t.Errorf("Apply with a missing root = %v, want one error", errs)
	}
}

// Files built by hand rather than through Parse.
func TestApplyHandBuiltFiles(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "config.json", `{"host":"localhost:1","port":1,"on":false}`)
	files := []File{
		{Path: "config.json", Parser: "JSON", Find: []Replacement{
			{"host", map[string]string{"localhost": "{{config.docker.interface}}"}},
			{"port", 25570},
			{"on", true},
			{"ratio", float32(0.5)},
			{"extra", map[string]any{"never": "created"}},
		}},
	}
	if errs := Apply(root, files, testValues); errs != nil {
		t.Fatal(errs)
	}
	if got, want := readTestFile(t, root, "config.json"), `{"host":"172.18.0.1:1","port":25570,"on":true,"ratio":0.5}`; got != want {
		t.Errorf("got %s, want %s", got, want)
	}

	for name, file := range map[string]File{
		"unknown parser":    {Path: "a", Parser: "toml", Find: []Replacement{{"a", "b"}}},
		"unsupported value": {Path: "a.json", Parser: "json", Find: []Replacement{{"a", []string{"b"}}}},
		"empty key":         {Path: "a.json", Parser: "json", Find: []Replacement{{"", "b"}}},
	} {
		if errs := Apply(root, []File{file}, testValues); len(errs) != 1 {
			t.Errorf("%s: errors = %v, want exactly one", name, errs)
		}
	}
}
