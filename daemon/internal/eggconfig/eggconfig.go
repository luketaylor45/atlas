// Package eggconfig applies Pterodactyl-style "config.files" rewrites to a game
// server's directory before the server starts.
//
// A config_files document maps a file path (relative to the server directory)
// to a parser and an ordered "find" map:
//
//	{
//	  "server.properties": {
//	    "parser": "properties",
//	    "find": { "server-port": "{{server.build.default.port}}" }
//	  }
//	}
//
// Values may be strings, numbers or booleans. Strings may contain placeholders
// (see Resolve). Files are written atomically, keep their mode and line endings,
// and are left untouched when nothing would change or when they fail to parse.
//
// # Parsers
//
// properties: Java .properties. The key is updated in place (key=value,
// key = value, key:value and "key value" all keep their separator), appended
// when missing, and the file is created when it does not exist. Comments, order
// and unrelated lines are kept byte for byte. Values are escaped minimally:
// backslash, newline, tab and a leading space. A backslash that starts a \uXXXX
// escape is kept so colour codes pass through; nothing else is escaped.
//
// ini: the key is "section.key", or a bare "key" for the global section.
// Section and key names are matched case-insensitively. When several existing
// sections are a prefix of the key the longest wins, so section names may
// contain dots ("/Script/Engine.GameSession.MaxPlayers"); a section that does
// not exist yet is named by the text before the first dot. Missing sections,
// keys and files are created. A value that was wrapped in double quotes stays
// quoted.
//
// json, yaml: the key is a dotted path. "a.b" descends objects, "[n]" (or a
// numeric segment on an array) is a zero-based index, "*" or "[*]" visits every
// child, and "\." is a literal dot. Missing objects are created along the way
// (never past a wildcard), an array can be extended by exactly one element,
// and missing files are created. A scalar in the middle of a path is an error
// rather than being overwritten. JSON keeps key order and indentation; YAML
// keeps comments and order through yaml.Node, but is re-emitted by the encoder
// so blank lines and the indentation of nested sequences are normalised.
//
// Typing for json/yaml: a JSON number or boolean in "find" is written as that
// type. A string is written as a string unless its resolved text is an integer
// or exactly "true"/"false" and either the existing value at that path is
// already a number/boolean, or the key is missing (or null) and the string
// consisted of nothing but one placeholder.
//
// A nested object value is a conditional replacement, in the order written:
// {"old": "new"} replaces the existing string value when it equals "old", or
// replaces every occurrence of "old" inside it; {"regex:PATTERN": "new"}
// applies a regular expression replacement ($1 refers to a group) when the
// pattern matches. The first entry that applies wins. Conditional replacements
// never create a missing key, and are only valid for json and yaml.
//
// xml: implemented on top of encoding/xml as byte-accurate splices, so
// everything that is not edited (comments, whitespace, attribute order,
// declarations) is preserved. Only UTF-8 documents are supported. The key is an
// element path separated by "/" (or by "." when it contains no "/"):
//
//   - "/root/child" or "root.child": the first step names the root element. A
//     path without a leading "/" whose first step is not the root is searched
//     at any depth, as is any step written after "//".
//   - "*" matches any element name; names match with or without their prefix.
//   - predicates: "[@attr='value']", "[@attr]" and the one-based "[n]".
//   - a trailing "/@attr" (or ".@attr") sets that attribute, adding it if
//     needed.
//   - a value of the form "[attr='value']" sets that attribute (Wings).
//   - otherwise, when the last step has an attribute predicate and the element
//     has a "value" attribute, that attribute is set
//     (<property name="ServerPort" value="26900"/>); in every other case the
//     element text is replaced. Replacing the text of an element that has
//     child elements is an error.
//
// Every matching element is updated. Elements are never created: a path that
// matches nothing does nothing. A missing file is skipped.
//
// file: for every line that starts with the key, the whole line is replaced by
// the value. When no line matched, the value is appended as a new line (unless
// that exact line is already present). A missing file is skipped.
package eggconfig

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	parserProperties = "properties"
	parserINI        = "ini"
	parserJSON       = "json"
	parserYAML       = "yaml"
	parserXML        = "xml"
	parserFile       = "file"

	newFileMode = 0o660
	newDirMode  = 0o770
)

// Values resolves placeholders.
type Values struct {
	Port      int
	IP        string // e.g. "0.0.0.0"
	Memory    int64  // MB
	UUID      string
	Interface string            // docker bridge gateway, e.g. "172.17.0.1"
	Env       map[string]string // server environment
}

// File is one entry of a config_files document.
type File struct {
	Path   string        // relative path inside the server directory
	Parser string        // properties | ini | json | yaml | xml | file
	Find   []Replacement // in the order written in the JSON (order matters for "file")
}

// Replacement is one entry of a "find" map.
//
// Value is a string, a number (int64 or float64 from Parse; any Go integer or
// float type, or json.Number, is accepted), a bool, nil, or a conditional
// replacement. Parse represents a conditional (nested object) as an ordered
// []Replacement whose Key is the text to look for and whose Value is what to
// write; map[string]any and map[string]string are also accepted and applied in
// sorted key order.
type Replacement struct {
	Key   string
	Value any
}

var (
	placeholderPattern     = regexp.MustCompile(`\{\{\s*([^{}]*?)\s*\}\}`)
	placeholderOnlyPattern = regexp.MustCompile(`^\s*\{\{[^{}]*\}\}\s*$`)
	integerPattern         = regexp.MustCompile(`^-?(0|[1-9][0-9]{0,17})$`)
	drivePattern           = regexp.MustCompile(`^[A-Za-z]:`)
)

// Resolve replaces placeholders in a string (exported for tests/other use).
//
// Supported: {{server.build.default.port}}, {{server.build.default.ip}},
// {{server.build.memory}}, {{server.build.env.NAME}}, {{env.NAME}},
// {{server.uuid}} and {{config.docker.interface}}, plus the names Wings uses
// for the same things. Whitespace inside the braces is ignored and unknown
// placeholders resolve to an empty string.
func Resolve(text string, values Values) string {
	return resolve(text, values, nil)
}

func resolve(text string, values Values, escape func(string) string) string {
	if !strings.Contains(text, "{{") {
		return text
	}
	return placeholderPattern.ReplaceAllStringFunc(text, func(match string) string {
		value := values.lookup(placeholderPattern.FindStringSubmatch(match)[1])
		if escape != nil {
			value = escape(value)
		}
		return value
	})
}

func (v Values) lookup(name string) string {
	switch name {
	case "server.build.default.port", "server.allocations.default.port":
		return strconv.Itoa(v.Port)
	case "server.build.default.ip", "server.allocations.default.ip":
		return v.IP
	case "server.build.memory", "server.build.memory_limit":
		return strconv.FormatInt(v.Memory, 10)
	case "server.uuid":
		return v.UUID
	case "config.docker.interface", "config.docker.network.interface":
		return v.Interface
	}
	for _, prefix := range []string{"server.build.env.", "server.environment.", "env."} {
		if rest, ok := strings.CutPrefix(name, prefix); ok {
			return v.Env[rest]
		}
	}
	return ""
}

// Parse validates a config_files JSON document. An empty/blank string, "{}",
// "[]" or "null" means no files (nil, nil).
func Parse(raw string) ([]File, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, nil
	}
	root, err := decodeJSON([]byte(trimmed))
	if err != nil {
		return nil, fmt.Errorf("eggconfig: invalid config_files: %w", err)
	}
	switch {
	case isNull(root):
		return nil, nil
	case root.Kind == yaml.SequenceNode && len(root.Content) == 0:
		return nil, nil
	case root.Kind == yaml.ScalarNode && root.Tag == "!!str":
		// Pterodactyl exports store config.files as a JSON string.
		if inner := strings.TrimSpace(root.Value); !strings.HasPrefix(inner, `"`) {
			return Parse(inner)
		}
	}
	if root.Kind != yaml.MappingNode {
		return nil, errors.New("eggconfig: config_files must be a JSON object keyed by file path")
	}

	var files []File
	for i := 0; i+1 < len(root.Content); i += 2 {
		file, err := parseFile(root.Content[i].Value, root.Content[i+1])
		if err != nil {
			return nil, fmt.Errorf("eggconfig: %s: %w", root.Content[i].Value, err)
		}
		files = append(files, file)
	}
	return files, nil
}

func parseFile(path string, body *yaml.Node) (File, error) {
	file := File{Path: path}
	if strings.TrimSpace(path) == "" {
		return file, errors.New("empty file path")
	}
	if body.Kind != yaml.MappingNode {
		return file, errors.New("entry must be an object with \"parser\" and \"find\"")
	}
	for i := 0; i+1 < len(body.Content); i += 2 {
		value := body.Content[i+1]
		switch body.Content[i].Value {
		case "parser":
			if value.Kind != yaml.ScalarNode || value.Tag != "!!str" {
				return file, errors.New("\"parser\" must be a string")
			}
			file.Parser = strings.ToLower(strings.TrimSpace(value.Value))
		case "find":
			find, err := parseFind(value)
			if err != nil {
				return file, err
			}
			file.Find = find
		}
	}
	if err := validateFile(file.Parser, file); err != nil {
		return file, err
	}
	return file, nil
}

func parseFind(node *yaml.Node) ([]Replacement, error) {
	switch {
	case isNull(node), node.Kind == yaml.SequenceNode && len(node.Content) == 0:
		return nil, nil
	case node.Kind != yaml.MappingNode:
		return nil, errors.New("\"find\" must be an object")
	}
	var find []Replacement
	for i := 0; i+1 < len(node.Content); i += 2 {
		key, value := node.Content[i].Value, node.Content[i+1]
		switch value.Kind {
		case yaml.ScalarNode:
			find = append(find, Replacement{Key: key, Value: nativeScalar(value)})
		case yaml.MappingNode:
			var pairs []Replacement
			for j := 0; j+1 < len(value.Content); j += 2 {
				if value.Content[j+1].Kind != yaml.ScalarNode {
					return nil, fmt.Errorf("find %q: conditional replacements must map text to a string, number or boolean", key)
				}
				pairs = append(pairs, Replacement{Key: value.Content[j].Value, Value: nativeScalar(value.Content[j+1])})
			}
			find = append(find, Replacement{Key: key, Value: pairs})
		default:
			return nil, fmt.Errorf("find %q: arrays are not supported as values", key)
		}
	}
	return find, nil
}

// nativeScalar converts a decoded JSON scalar to string, bool, int64, float64 or nil.
func nativeScalar(node *yaml.Node) any {
	switch node.Tag {
	case "!!bool":
		return node.Value == "true"
	case "!!int":
		if n, err := strconv.ParseInt(node.Value, 10, 64); err == nil {
			return n
		}
		fallthrough
	case "!!float":
		if f, err := strconv.ParseFloat(node.Value, 64); err == nil {
			return f
		}
		return node.Value
	case "!!null":
		return nil
	}
	return node.Value
}

func validateFile(parser string, file File) error {
	switch parser {
	case parserProperties, parserINI, parserJSON, parserYAML, parserXML, parserFile:
	case "":
		return errors.New("missing parser")
	default:
		return fmt.Errorf("unknown parser %q", file.Parser)
	}
	for _, rep := range file.Find {
		if rep.Key == "" {
			return errors.New("empty find key")
		}
		if pairs, ok := conditionalPairs(rep.Value); ok {
			if parser != parserJSON && parser != parserYAML {
				return fmt.Errorf("find %q: conditional replacements are only supported by the json and yaml parsers", rep.Key)
			}
			for _, pair := range pairs {
				if _, err := scalarText(pair.Value, Values{}, nil); err != nil {
					return fmt.Errorf("find %q: %w", rep.Key, err)
				}
				if pattern, ok := strings.CutPrefix(pair.Key, "regex:"); ok {
					if _, err := regexp.Compile(pattern); err != nil {
						return fmt.Errorf("find %q: %w", rep.Key, err)
					}
				}
			}
		} else if _, err := scalarText(rep.Value, Values{}, nil); err != nil {
			return fmt.Errorf("find %q: %w", rep.Key, err)
		}
		var err error
		switch parser {
		case parserJSON, parserYAML:
			_, err = parseTreePath(rep.Key)
		case parserXML:
			_, err = parseXMLPath(rep.Key)
		}
		if err != nil {
			return fmt.Errorf("find %q: %w", rep.Key, err)
		}
	}
	return nil
}

// conditionalPairs returns the ordered entries of a nested-object value.
func conditionalPairs(value any) ([]Replacement, bool) {
	switch typed := value.(type) {
	case []Replacement:
		return typed, true
	case map[string]any:
		pairs := make([]Replacement, 0, len(typed))
		for key, v := range typed {
			pairs = append(pairs, Replacement{Key: key, Value: v})
		}
		sort.Slice(pairs, func(i, j int) bool { return pairs[i].Key < pairs[j].Key })
		return pairs, true
	case map[string]string:
		pairs := make([]Replacement, 0, len(typed))
		for key, v := range typed {
			pairs = append(pairs, Replacement{Key: key, Value: v})
		}
		sort.Slice(pairs, func(i, j int) bool { return pairs[i].Key < pairs[j].Key })
		return pairs, true
	}
	return nil, false
}

// scalarText renders a scalar find value as text, resolving placeholders in strings.
func scalarText(value any, values Values, escape func(string) string) (string, error) {
	switch typed := value.(type) {
	case nil:
		return "", nil
	case string:
		return resolve(typed, values, escape), nil
	case bool:
		return strconv.FormatBool(typed), nil
	case json.Number:
		return typed.String(), nil
	case int:
		return strconv.FormatInt(int64(typed), 10), nil
	case int8:
		return strconv.FormatInt(int64(typed), 10), nil
	case int16:
		return strconv.FormatInt(int64(typed), 10), nil
	case int32:
		return strconv.FormatInt(int64(typed), 10), nil
	case int64:
		return strconv.FormatInt(typed, 10), nil
	case uint:
		return strconv.FormatUint(uint64(typed), 10), nil
	case uint8:
		return strconv.FormatUint(uint64(typed), 10), nil
	case uint16:
		return strconv.FormatUint(uint64(typed), 10), nil
	case uint32:
		return strconv.FormatUint(uint64(typed), 10), nil
	case uint64:
		return strconv.FormatUint(typed, 10), nil
	case float32:
		return strconv.FormatFloat(float64(typed), 'f', -1, 32), nil
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64), nil
	}
	return "", fmt.Errorf("unsupported value type %T", value)
}

// Apply rewrites every file under root. It returns one error per file that
// failed (others are still applied); nil when all succeeded.
func Apply(root string, files []File, values Values) []error {
	var errs []error
	for _, file := range files {
		if err := applyFile(root, file, values); err != nil {
			errs = append(errs, fmt.Errorf("eggconfig: %s: %w", file.Path, err))
		}
	}
	return errs
}

func applyFile(root string, file File, values Values) error {
	parser := strings.ToLower(strings.TrimSpace(file.Parser))
	if err := validateFile(parser, file); err != nil {
		return err
	}
	path, info, err := securePath(root, file.Path)
	if err != nil {
		return err
	}
	if len(file.Find) == 0 {
		return nil
	}

	var data []byte
	switch {
	case info != nil:
		if data, err = os.ReadFile(path); err != nil {
			return err
		}
	case parser == parserXML || parser == parserFile:
		return nil
	}

	var out []byte
	switch parser {
	case parserProperties:
		out, err = applyProperties(data, file.Find, values)
	case parserINI:
		out, err = applyINI(data, file.Find, values)
	case parserJSON:
		out, err = applyJSON(data, file.Find, values)
	case parserYAML:
		out, err = applyYAML(data, file.Find, values)
	case parserXML:
		out, err = applyXML(data, file.Find, values)
	case parserFile:
		out, err = applyLines(data, file.Find, values)
	}
	if err != nil {
		return err
	}
	if info != nil && bytes.Equal(out, data) {
		return nil
	}
	if info == nil && len(out) == 0 {
		return nil
	}
	return writeAtomic(path, out, info)
}

// securePath maps a relative config path to a location that is guaranteed to
// be inside root, following symlinks only while they stay inside it. The
// returned info is nil when the file does not exist yet.
func securePath(root, rel string) (string, fs.FileInfo, error) {
	if strings.TrimSpace(rel) == "" {
		return "", nil, errors.New("empty file path")
	}
	if strings.ContainsRune(rel, 0) {
		return "", nil, errors.New("invalid file path")
	}
	if strings.HasPrefix(rel, "/") || strings.HasPrefix(rel, `\`) || drivePattern.MatchString(rel) || filepath.IsAbs(rel) {
		return "", nil, errors.New("absolute paths are not allowed")
	}
	for _, part := range strings.FieldsFunc(rel, func(r rune) bool { return r == '/' || r == '\\' }) {
		if part == ".." {
			return "", nil, errors.New("path escapes the server directory")
		}
	}
	local := filepath.Clean(filepath.FromSlash(rel))
	if local == "." || !filepath.IsLocal(local) {
		return "", nil, errors.New("path escapes the server directory")
	}

	absRoot, err := filepath.Abs(root)
	if err != nil {
		return "", nil, err
	}
	realRoot, err := filepath.EvalSymlinks(absRoot)
	if err != nil {
		return "", nil, fmt.Errorf("server directory: %w", err)
	}

	// Resolve the deepest part of the path that exists; anything below it is
	// created by us and therefore cannot be a symlink.
	existing := filepath.Join(realRoot, local)
	var missing []string
	for {
		_, err := os.Lstat(existing)
		if err == nil {
			break
		}
		if !errors.Is(err, fs.ErrNotExist) {
			return "", nil, err
		}
		parent := filepath.Dir(existing)
		if parent == existing {
			return "", nil, err
		}
		missing = append([]string{filepath.Base(existing)}, missing...)
		existing = parent
	}
	resolved, err := filepath.EvalSymlinks(existing)
	if err != nil {
		return "", nil, fmt.Errorf("cannot resolve path: %w", err)
	}
	if !within(realRoot, resolved) {
		return "", nil, errors.New("path escapes the server directory through a symlink")
	}
	if err := plainBelow(realRoot, resolved); err != nil {
		return "", nil, err
	}
	final := filepath.Join(append([]string{resolved}, missing...)...)

	info, err := os.Stat(final)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return final, nil, nil
	case err != nil:
		return "", nil, err
	case info.IsDir():
		return "", nil, errors.New("path is a directory")
	case !info.Mode().IsRegular():
		return "", nil, errors.New("path is not a regular file")
	}
	return final, info, nil
}

// plainBelow rejects links that EvalSymlinks does not resolve, which on Windows
// means junctions and other mount points: every component between root and
// path must be an ordinary file or directory.
func plainBelow(root, path string) error {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return err
	}
	if rel == "." {
		return nil
	}
	current := root
	for _, part := range strings.Split(rel, string(filepath.Separator)) {
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if err != nil {
			return err
		}
		if info.Mode()&(fs.ModeSymlink|fs.ModeIrregular) != 0 {
			return errors.New("path crosses a link or mount point")
		}
	}
	return nil
}

func within(root, path string) bool {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel))
}

// writeAtomic replaces path through a temporary file in the same directory.
// Existing files keep their mode (and, where the platform allows, owner); new
// files get 0660 and the owner of their directory.
func writeAtomic(path string, data []byte, info fs.FileInfo) error {
	dir := filepath.Dir(path)
	mode := fs.FileMode(newFileMode)
	owner := info
	if info != nil {
		mode = info.Mode().Perm()
	} else {
		if err := makeDirs(dir); err != nil {
			return err
		}
		if dirInfo, err := os.Stat(dir); err == nil {
			owner = dirInfo
		}
	}

	tmp, err := os.CreateTemp(dir, "."+filepath.Base(path)+".*.tmp")
	if err != nil {
		return err
	}
	name := tmp.Name()
	done := false
	defer func() {
		if !done {
			_ = os.Remove(name)
		}
	}()

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(name, mode); err != nil {
		return err
	}
	if owner != nil {
		copyOwner(name, owner)
	}
	if err := os.Rename(name, path); err != nil {
		return err
	}
	done = true
	return nil
}

// makeDirs creates dir and its missing parents with 0770, owned like the
// nearest directory that already exists.
func makeDirs(dir string) error {
	var missing []string
	current := dir
	for {
		info, err := os.Stat(current)
		if err == nil {
			if !info.IsDir() {
				return fmt.Errorf("%s is not a directory", current)
			}
			break
		}
		if !errors.Is(err, fs.ErrNotExist) {
			return err
		}
		parent := filepath.Dir(current)
		if parent == current {
			return err
		}
		missing = append(missing, current)
		current = parent
	}
	owner, _ := os.Stat(current)
	for i := len(missing) - 1; i >= 0; i-- {
		if err := os.Mkdir(missing[i], newDirMode); err != nil && !errors.Is(err, fs.ErrExist) {
			return err
		}
		_ = os.Chmod(missing[i], newDirMode)
		if owner != nil {
			copyOwner(missing[i], owner)
		}
	}
	return nil
}
