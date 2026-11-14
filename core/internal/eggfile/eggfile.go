// Package eggfile reads egg files: the Atlas catalog format and Pterodactyl
// or Pelican exports (PTDL_v1 and PTDL_v2).
package eggfile

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/luketaylor45/atlas/core/internal/models"
)

// Parsed is an egg file turned into database records, not yet saved.
type Parsed struct {
	// UUID is the identifier the file declares. Catalog eggs require one;
	// imports ignore it.
	UUID      string
	Egg       models.Egg
	Variables []models.EggVariable
	// ScriptFiles are install script files to join in front of the inline
	// script, relative to the catalog root. Only catalog eggs may use them.
	ScriptFiles []string
	// Notes tell the admin about anything that was adapted or left out.
	Notes []string
}

type fileVariable struct {
	Name         string `json:"name"`
	Description  string `json:"description"`
	EnvVariable  string `json:"env_variable"`
	DefaultValue any    `json:"default_value"`
	UserViewable *bool  `json:"user_viewable"`
	UserEditable *bool  `json:"user_editable"`
	Rules        string `json:"rules"`
	InputType    string `json:"input_type"`
	FieldType    string `json:"field_type"`
}

type atlasFile struct {
	UUID               string           `json:"uuid"`
	Name               string           `json:"name"`
	Author             string           `json:"author"`
	Description        string           `json:"description"`
	DockerImages       []string         `json:"docker_images"`
	StartupCommand     string           `json:"startup_command"`
	StopCommand        string           `json:"stop_command"`
	StartupDone        json.RawMessage  `json:"startup_done"`
	ConfigFiles        json.RawMessage  `json:"config_files"`
	ScriptInstall      string           `json:"script_install"`
	ScriptInstallFiles []string         `json:"script_install_files"`
	ScriptContainer    string           `json:"script_container"`
	ScriptEntry        string           `json:"script_entry"`
	Ports              []models.EggPort `json:"ports"`
	Variables          []fileVariable   `json:"variables"`
}

type pterodactylFile struct {
	Name         string          `json:"name"`
	Author       string          `json:"author"`
	Description  string          `json:"description"`
	DockerImages json.RawMessage `json:"docker_images"`
	// Very old exports have a single image.
	Image   string `json:"image"`
	Startup string `json:"startup"`
	Config  struct {
		Files   json.RawMessage `json:"files"`
		Startup json.RawMessage `json:"startup"`
		Stop    string          `json:"stop"`
	} `json:"config"`
	Scripts struct {
		Installation struct {
			Script     string `json:"script"`
			Container  string `json:"container"`
			Entrypoint string `json:"entrypoint"`
		} `json:"installation"`
	} `json:"scripts"`
	Variables []fileVariable `json:"variables"`
}

// Parse reads an egg file in either format.
func Parse(content []byte) (*Parsed, error) {
	content = bytes.TrimPrefix(content, []byte("\xef\xbb\xbf"))

	// The two formats share field names with different shapes (docker_images
	// is a list in one and a map in the other), so the file is inspected
	// before it is decoded into either.
	var probe map[string]json.RawMessage
	if err := json.Unmarshal(content, &probe); err != nil {
		return nil, errors.New("the file is not valid JSON")
	}

	var parsed *Parsed
	var variables []fileVariable
	var err error
	if _, isAtlas := probe["startup_command"]; isAtlas {
		parsed, variables, err = parseAtlas(content)
	} else {
		parsed, variables, err = parsePterodactyl(content)
	}
	if err != nil {
		return nil, err
	}

	egg := &parsed.Egg
	egg.Name = strings.TrimSpace(egg.Name)
	if egg.Name == "" {
		return nil, errors.New("the egg has no name")
	}
	if strings.TrimSpace(egg.StartupCommand) == "" {
		return nil, errors.New("the egg has no startup command")
	}
	var images []string
	_ = json.Unmarshal([]byte(egg.DockerImages), &images)
	if len(images) == 0 {
		return nil, errors.New("the egg lists no Docker images")
	}

	seen := map[string]bool{}
	for _, variable := range variables {
		env := strings.TrimSpace(variable.EnvVariable)
		if env == "" || seen[env] {
			continue
		}
		seen[env] = true

		inputType := variable.InputType
		if inputType == "" {
			inputType = variable.FieldType
		}
		if inputType == "" {
			inputType = "text"
		}
		parsed.Variables = append(parsed.Variables, models.EggVariable{
			Name:                variable.Name,
			Description:         variable.Description,
			EnvironmentVariable: env,
			DefaultValue:        scalarText(variable.DefaultValue),
			UserViewable:        flagOr(variable.UserViewable, true),
			UserEditable:        flagOr(variable.UserEditable, true),
			Rules:               variable.Rules,
			InputType:           inputType,
		})
	}
	return parsed, nil
}

func parseAtlas(content []byte) (*Parsed, []fileVariable, error) {
	var file atlasFile
	if err := json.Unmarshal(content, &file); err != nil {
		return nil, nil, errors.New("the file is not a valid Atlas egg: " + err.Error())
	}

	configFiles, err := normalizeConfigFiles(file.ConfigFiles)
	if err != nil {
		return nil, nil, err
	}
	done, err := normalizeDone(file.StartupDone)
	if err != nil {
		return nil, nil, err
	}

	images, _ := json.Marshal(file.DockerImages)
	parsed := &Parsed{
		UUID:        strings.TrimSpace(file.UUID),
		ScriptFiles: file.ScriptInstallFiles,
		Egg: models.Egg{
			Name:            file.Name,
			Author:          file.Author,
			Description:     file.Description,
			DockerImages:    string(images),
			StartupCommand:  file.StartupCommand,
			StopCommand:     file.StopCommand,
			StartupDone:     done,
			ConfigFiles:     configFiles,
			ScriptInstall:   file.ScriptInstall,
			ScriptContainer: file.ScriptContainer,
			ScriptEntry:     file.ScriptEntry,
			Ports:           encodePorts(file.Ports),
		},
	}
	return parsed, file.Variables, nil
}

func parsePterodactyl(content []byte) (*Parsed, []fileVariable, error) {
	var file pterodactylFile
	if err := json.Unmarshal(content, &file); err != nil {
		return nil, nil, errors.New("the file is not a valid Pterodactyl egg: " + err.Error())
	}

	parsed := &Parsed{}

	configFiles, err := normalizeConfigFiles(file.Config.Files)
	if err != nil {
		// A broken optional section should not block the import.
		parsed.Notes = append(parsed.Notes, "The egg's config file rules could not be read and were left out.")
		configFiles = ""
	}
	done, err := pterodactylDone(file.Config.Startup)
	if err != nil {
		parsed.Notes = append(parsed.Notes, "The egg's startup detection could not be read and was left out.")
		done = ""
	}

	// Extra ports arrive as plain variables in Pterodactyl eggs. The ones
	// that are unmistakably ports become allocations, so two servers of the
	// same game can never be given the same one.
	var ports []models.EggPort
	variables := make([]fileVariable, 0, len(file.Variables))
	for _, variable := range file.Variables {
		if port, ok := portFromVariable(variable); ok {
			ports = append(ports, port)
			continue
		}
		variables = append(variables, variable)
	}
	if len(ports) > 0 {
		names := make([]string, 0, len(ports))
		for _, port := range ports {
			names = append(names, port.Env)
		}
		parsed.Notes = append(parsed.Notes, "These variables became allocated ports: "+strings.Join(names, ", ")+".")
	}

	images, _ := json.Marshal(pterodactylImages(file.DockerImages, file.Image))
	parsed.Egg = models.Egg{
		Name:            file.Name,
		Author:          file.Author,
		Description:     file.Description,
		DockerImages:    string(images),
		StartupCommand:  file.Startup,
		StopCommand:     file.Config.Stop,
		StartupDone:     done,
		ConfigFiles:     configFiles,
		ScriptInstall:   file.Scripts.Installation.Script,
		ScriptContainer: file.Scripts.Installation.Container,
		ScriptEntry:     file.Scripts.Installation.Entrypoint,
		Ports:           encodePorts(ports),
	}
	return parsed, variables, nil
}

// pterodactylImages returns an egg's images with the default first.
// PTDL_v2 stores a map of label to image; older files a list or one image.
func pterodactylImages(raw json.RawMessage, single string) []string {
	var list []string
	if json.Unmarshal(raw, &list) == nil && len(list) > 0 {
		return list
	}

	var labelled map[string]string
	if json.Unmarshal(raw, &labelled) == nil && len(labelled) > 0 {
		// A JSON object has no order, and the first image becomes the
		// default. Sorting by label, newest first, makes that choice stable
		// and matches how these eggs list their images ("Java 21", "Java 17").
		labels := make([]string, 0, len(labelled))
		for label := range labelled {
			labels = append(labels, label)
		}
		sort.Sort(sort.Reverse(naturalOrder(labels)))
		ordered := make([]string, 0, len(labels))
		for _, label := range labels {
			ordered = append(ordered, labelled[label])
		}
		return ordered
	}

	if single = strings.TrimSpace(single); single != "" {
		return []string{single}
	}
	return nil
}

// unwrapJSON returns the document inside a value that Pterodactyl exported
// as a JSON-encoded string, or the value itself when it is already a
// document. Blank and null values come back as nil.
func unwrapJSON(raw json.RawMessage) (json.RawMessage, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return nil, nil
	}
	if trimmed[0] == '"' {
		var inner string
		if err := json.Unmarshal(trimmed, &inner); err != nil {
			return nil, err
		}
		trimmed = bytes.TrimSpace([]byte(inner))
		if len(trimmed) == 0 {
			return nil, nil
		}
	}
	if !json.Valid(trimmed) {
		return nil, errors.New("not valid JSON")
	}
	return json.RawMessage(trimmed), nil
}

// normalizeConfigFiles checks a config-files section and returns it as a
// compact JSON object, or "" when there is nothing in it. Key order is kept:
// it is meaningful to the parsers.
func normalizeConfigFiles(raw json.RawMessage) (string, error) {
	document, err := unwrapJSON(raw)
	if err != nil {
		return "", errors.New("config_files is not valid JSON")
	}
	if document == nil {
		return "", nil
	}

	var files map[string]struct {
		Parser string          `json:"parser"`
		Find   json.RawMessage `json:"find"`
	}
	if err := json.Unmarshal(document, &files); err != nil {
		// Pterodactyl writes "[]" for an egg with no config files.
		var empty []json.RawMessage
		if json.Unmarshal(document, &empty) == nil && len(empty) == 0 {
			return "", nil
		}
		return "", errors.New("config_files must be an object of file names")
	}
	if len(files) == 0 {
		return "", nil
	}
	for name, file := range files {
		switch file.Parser {
		case "properties", "ini", "json", "yaml", "yml", "xml", "file":
		default:
			return "", fmt.Errorf("config file %s uses unknown parser %q", name, file.Parser)
		}
	}

	var compact bytes.Buffer
	if err := json.Compact(&compact, document); err != nil {
		return "", err
	}
	return compact.String(), nil
}

// normalizeDone accepts one string or a list of strings.
func normalizeDone(raw json.RawMessage) (string, error) {
	document, err := unwrapDone(raw)
	if err != nil {
		return "", errors.New("startup_done must be a string or a list of strings")
	}
	return document, nil
}

func unwrapDone(raw json.RawMessage) (string, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return "", nil
	}

	var list []string
	if trimmed[0] == '"' {
		var single string
		if err := json.Unmarshal(trimmed, &single); err != nil {
			return "", err
		}
		list = []string{single}
	} else if err := json.Unmarshal(trimmed, &list); err != nil {
		return "", err
	}

	kept := list[:0]
	for _, item := range list {
		if strings.TrimSpace(item) != "" {
			kept = append(kept, item)
		}
	}
	if len(kept) == 0 {
		return "", nil
	}
	encoded, _ := json.Marshal(kept)
	return string(encoded), nil
}

// pterodactylDone reads config.startup, which holds {"done": ...} either as
// an object or as a JSON-encoded string.
func pterodactylDone(raw json.RawMessage) (string, error) {
	document, err := unwrapJSON(raw)
	if err != nil || document == nil {
		return "", err
	}
	var startup struct {
		Done json.RawMessage `json:"done"`
	}
	if err := json.Unmarshal(document, &startup); err != nil {
		return "", err
	}
	return unwrapDone(startup.Done)
}

// portFromVariable recognises a variable that holds an extra port: its name
// ends in _PORT and its default is a port number.
func portFromVariable(variable fileVariable) (models.EggPort, bool) {
	env := strings.TrimSpace(variable.EnvVariable)
	upper := strings.ToUpper(env)
	if env == models.PrimaryPortEnv || !(strings.HasSuffix(upper, "_PORT") || upper == "PORT") {
		return models.EggPort{}, false
	}
	number, err := strconv.Atoi(strings.TrimSpace(scalarText(variable.DefaultValue)))
	if err != nil || number < 1 || number > 65535 {
		return models.EggPort{}, false
	}
	name := strings.TrimSpace(variable.Name)
	if name == "" {
		name = env
	}
	return models.EggPort{Name: name, Env: env, Default: number, Protocol: "both"}, true
}

func encodePorts(ports []models.EggPort) string {
	if len(ports) == 0 {
		return ""
	}
	encoded, _ := json.Marshal(ports)
	return string(encoded)
}

// scalarText renders a default value, which exports write as a string,
// number or boolean.
func scalarText(value any) string {
	switch typed := value.(type) {
	case nil:
		return ""
	case string:
		return typed
	case bool:
		if typed {
			return "true"
		}
		return "false"
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64)
	default:
		return fmt.Sprint(typed)
	}
}

func flagOr(value *bool, fallback bool) bool {
	if value == nil {
		return fallback
	}
	return *value
}

// naturalOrder sorts labels so that numbers inside them compare by value
// ("Java 8" before "Java 17").
type naturalOrder []string

func (n naturalOrder) Len() int      { return len(n) }
func (n naturalOrder) Swap(i, j int) { n[i], n[j] = n[j], n[i] }
func (n naturalOrder) Less(i, j int) bool {
	a, b := n[i], n[j]
	for a != "" && b != "" {
		aDigits, bDigits := leadingDigits(a), leadingDigits(b)
		if aDigits > 0 && bDigits > 0 {
			aNumber, bNumber := strings.TrimLeft(a[:aDigits], "0"), strings.TrimLeft(b[:bDigits], "0")
			if len(aNumber) != len(bNumber) {
				return len(aNumber) < len(bNumber)
			}
			if aNumber != bNumber {
				return aNumber < bNumber
			}
			a, b = a[aDigits:], b[bDigits:]
			continue
		}
		if a[0] != b[0] {
			return a[0] < b[0]
		}
		a, b = a[1:], b[1:]
	}
	return len(a) < len(b)
}

func leadingDigits(value string) int {
	count := 0
	for count < len(value) && value[count] >= '0' && value[count] <= '9' {
		count++
	}
	return count
}

// NormalizeConfigFiles checks config-file rules typed by an admin and
// returns them in stored form ("" when there are none).
func NormalizeConfigFiles(text string) (string, error) {
	return normalizeConfigFiles(json.RawMessage(text))
}

// NormalizeStartupDone checks a list of startup-done strings typed by an
// admin and returns it in stored form ("" when there are none).
func NormalizeStartupDone(text string) (string, error) {
	return normalizeDone(json.RawMessage(text))
}
