package eggconfig

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// JSON and YAML documents are both edited as yaml.Node trees: JSON is decoded
// into nodes token by token (which keeps key order) and written back by
// writeJSON, YAML goes through the yaml package so comments survive.

type segmentKind int

const (
	segmentKey segmentKind = iota
	segmentIndex
	segmentWildcard
)

type segment struct {
	kind  segmentKind
	key   string
	index int
}

// parseTreePath splits "a.b[0].*.c" into segments.
func parseTreePath(path string) ([]segment, error) {
	var (
		segments     []segment
		buffer       strings.Builder
		haveKey      bool
		afterBracket bool
	)
	pushKey := func() {
		key := buffer.String()
		buffer.Reset()
		haveKey = false
		if key == "*" {
			segments = append(segments, segment{kind: segmentWildcard})
			return
		}
		segments = append(segments, segment{kind: segmentKey, key: key})
	}

	for i := 0; i < len(path); i++ {
		c := path[i]
		switch {
		case c == '\\' && i+1 < len(path) && strings.IndexByte(`.[\`, path[i+1]) >= 0:
			if afterBracket {
				return nil, fmt.Errorf("invalid path %q: expected \".\" after \"]\"", path)
			}
			buffer.WriteByte(path[i+1])
			haveKey = true
			i++
		case c == '.':
			if haveKey {
				pushKey()
			} else if !afterBracket {
				return nil, fmt.Errorf("invalid path %q: empty segment", path)
			}
			afterBracket = false
		case c == '[':
			if haveKey {
				pushKey()
			}
			end := strings.IndexByte(path[i:], ']')
			if end < 0 {
				return nil, fmt.Errorf("invalid path %q: missing \"]\"", path)
			}
			inner := strings.TrimSpace(path[i+1 : i+end])
			if n, err := strconv.Atoi(inner); err == nil && n >= 0 {
				segments = append(segments, segment{kind: segmentIndex, index: n})
			} else if inner == "*" {
				segments = append(segments, segment{kind: segmentWildcard})
			} else if key := strings.Trim(inner, `"'`); key != "" {
				segments = append(segments, segment{kind: segmentKey, key: key})
			} else {
				return nil, fmt.Errorf("invalid path %q: empty \"[]\"", path)
			}
			i += end
			afterBracket = true
		default:
			if afterBracket {
				return nil, fmt.Errorf("invalid path %q: expected \".\" after \"]\"", path)
			}
			buffer.WriteByte(c)
			haveKey = true
		}
	}
	if haveKey {
		pushKey()
	} else if !afterBracket {
		return nil, fmt.Errorf("invalid path %q: empty segment", path)
	}
	return segments, nil
}

// treeEdit applies one replacement to a document.
type treeEdit struct {
	rep         Replacement
	pairs       []Replacement
	conditional bool
	values      Values
	changed     bool
}

func applyTree(root *yaml.Node, find []Replacement, values Values) (bool, error) {
	changed := false
	for _, rep := range find {
		segments, err := parseTreePath(rep.Key)
		if err != nil {
			return false, err
		}
		edit := &treeEdit{rep: rep, values: values}
		edit.pairs, edit.conditional = conditionalPairs(rep.Value)
		if err := edit.walk(root, segments, false, false); err != nil {
			return false, fmt.Errorf("%s: %w", rep.Key, err)
		}
		changed = changed || edit.changed
	}
	return changed, nil
}

// canCreate reports whether missing containers may be created for the rest of
// the path: never for conditional replacements and never past a wildcard.
func (e *treeEdit) canCreate(rest []segment) bool {
	if e.conditional {
		return false
	}
	for _, s := range rest {
		if s.kind == segmentWildcard {
			return false
		}
	}
	return true
}

// walk descends to the nodes named by segments. missing is true when node was
// created for this replacement; soft is true below a wildcard or for a
// conditional replacement, where a shape mismatch is skipped instead of failing.
func (e *treeEdit) walk(node *yaml.Node, segments []segment, missing, soft bool) error {
	for node.Kind == yaml.AliasNode && node.Alias != nil {
		node = node.Alias
	}
	if len(segments) == 0 {
		return e.assign(node, missing)
	}
	current, rest := segments[0], segments[1:]
	soft = soft || e.conditional

	switch current.kind {
	case segmentWildcard:
		switch node.Kind {
		case yaml.MappingNode:
			for i := 1; i < len(node.Content); i += 2 {
				if err := e.walk(node.Content[i], rest, false, true); err != nil {
					return err
				}
			}
		case yaml.SequenceNode:
			for _, child := range node.Content {
				if err := e.walk(child, rest, false, true); err != nil {
					return err
				}
			}
		}
		return nil

	case segmentIndex:
		return e.walkIndex(node, current.index, rest, soft)
	}

	if node.Kind == yaml.SequenceNode {
		if index, err := strconv.Atoi(current.key); err == nil && index >= 0 {
			return e.walkIndex(node, index, rest, soft)
		}
	}
	if isNull(node) {
		if !e.canCreate(rest) {
			return nil
		}
		makeContainer(node, yaml.MappingNode)
		e.changed = true
	}
	if node.Kind != yaml.MappingNode {
		if soft {
			return nil
		}
		return fmt.Errorf("cannot set %q: the parent is not an object", current.key)
	}
	for i := 0; i+1 < len(node.Content); i += 2 {
		if node.Content[i].Value == current.key {
			return e.walk(node.Content[i+1], rest, false, soft)
		}
	}
	if !e.canCreate(rest) {
		return nil
	}
	child := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!null", Value: "null"}
	node.Content = append(node.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: current.key}, child)
	e.changed = true
	return e.walk(child, rest, true, soft)
}

func (e *treeEdit) walkIndex(node *yaml.Node, index int, rest []segment, soft bool) error {
	if isNull(node) {
		if !e.canCreate(rest) {
			return nil
		}
		makeContainer(node, yaml.SequenceNode)
		e.changed = true
	}
	if node.Kind != yaml.SequenceNode {
		if soft {
			return nil
		}
		return fmt.Errorf("cannot set index %d: the parent is not an array", index)
	}
	if index < len(node.Content) {
		return e.walk(node.Content[index], rest, false, soft)
	}
	if !e.canCreate(rest) || (soft && index > len(node.Content)) {
		return nil
	}
	if index > len(node.Content) {
		return fmt.Errorf("index %d is out of range (length %d)", index, len(node.Content))
	}
	child := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!null", Value: "null"}
	node.Content = append(node.Content, child)
	e.changed = true
	return e.walk(child, rest, true, soft)
}

func (e *treeEdit) assign(node *yaml.Node, missing bool) error {
	if !e.conditional {
		text, err := scalarText(e.rep.Value, e.values, nil)
		if err != nil {
			return err
		}
		e.set(node, scalarTag(e.rep.Value, text, node, missing), text)
		return nil
	}

	if missing || node.Kind != yaml.ScalarNode || isNull(node) {
		return nil
	}
	current := node.Value
	for _, pair := range e.pairs {
		whole, err := scalarText(pair.Value, e.values, nil)
		if err != nil {
			return err
		}
		var replaced string
		if pattern, ok := strings.CutPrefix(pair.Key, "regex:"); ok {
			re, err := regexp.Compile(pattern)
			if err != nil {
				return err
			}
			if !re.MatchString(current) {
				continue
			}
			// Placeholder values are literal text, only "$n" written in the
			// egg refers to a capture group.
			template, err := scalarText(pair.Value, e.values, func(s string) string {
				return strings.ReplaceAll(s, "$", "$$")
			})
			if err != nil {
				return err
			}
			replaced = re.ReplaceAllString(current, template)
		} else {
			match := Resolve(pair.Key, e.values)
			switch {
			case current == match:
				replaced = whole
			case match != "" && strings.Contains(current, match):
				replaced = strings.ReplaceAll(current, match, whole)
			default:
				continue
			}
		}
		// Only a value that replaced the scalar as a whole carries its own type.
		raw := pair.Value
		if replaced != whole {
			raw = ""
		}
		e.set(node, scalarTag(raw, replaced, node, false), replaced)
		return nil
	}
	return nil
}

// scalarTag decides the type a value is written as.
//
// Numbers, booleans and null given natively in "find" keep their type. A
// string stays a string unless its resolved text is an integer or exactly
// "true"/"false" and either the value it replaces already is a number/boolean,
// or there is nothing to replace (missing key or null) and the string was a
// single placeholder such as "{{server.build.default.port}}".
func scalarTag(raw any, text string, existing *yaml.Node, missing bool) string {
	switch raw.(type) {
	case nil:
		return "!!null"
	case bool:
		return "!!bool"
	case string:
	default:
		if integerPattern.MatchString(text) {
			return "!!int"
		}
		return "!!float"
	}

	existingTag := "!!null"
	if !missing && existing != nil && existing.Kind != 0 {
		existingTag = existing.ShortTag()
	}
	guess := existingTag == "!!null" && placeholderOnlyPattern.MatchString(raw.(string))
	switch {
	case integerPattern.MatchString(text) && (existingTag == "!!int" || existingTag == "!!float" || guess):
		return "!!int"
	case (text == "true" || text == "false") && (existingTag == "!!bool" || guess):
		return "!!bool"
	}
	return "!!str"
}

// set turns node into a scalar, keeping its comments and (for strings) quoting.
func (e *treeEdit) set(node *yaml.Node, tag, text string) {
	if tag == "!!null" {
		text = "null"
	}
	if node.Kind == yaml.ScalarNode && node.ShortTag() == tag && (node.Value == text || tag == "!!null") {
		return
	}
	style := yaml.Style(0)
	if tag == "!!str" && node.Kind == yaml.ScalarNode {
		style = node.Style & (yaml.DoubleQuotedStyle | yaml.SingleQuotedStyle)
	}
	node.Kind = yaml.ScalarNode
	node.Tag = tag
	node.Value = text
	node.Style = style
	node.Content = nil
	node.Alias = nil
	e.changed = true
}

func isNull(node *yaml.Node) bool {
	return node.Kind == 0 || (node.Kind == yaml.ScalarNode && node.ShortTag() == "!!null")
}

func makeContainer(node *yaml.Node, kind yaml.Kind) {
	node.Kind = kind
	node.Tag = "!!map"
	if kind == yaml.SequenceNode {
		node.Tag = "!!seq"
	}
	node.Value = ""
	node.Style = 0
	node.Content = nil
}

// ---------------------------------------------------------------------------
// json

var utf8BOM = []byte{0xEF, 0xBB, 0xBF}

var jsonIndentPattern = regexp.MustCompile(`\n([ \t]+)[^\s]`)

func applyJSON(data []byte, find []Replacement, values Values) ([]byte, error) {
	body := bytes.TrimPrefix(data, utf8BOM)
	hasBOM := len(body) != len(data)

	var root *yaml.Node
	if len(bytes.TrimSpace(body)) == 0 {
		root = &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	} else {
		var err error
		if root, err = decodeJSON(body); err != nil {
			return nil, fmt.Errorf("invalid JSON: %w", err)
		}
	}

	changed, err := applyTree(root, find, values)
	if err != nil {
		return nil, err
	}
	if !changed {
		return data, nil
	}

	// Keep the document's own layout: its indent, line endings, final newline,
	// and single-line form if that is how it was written.
	format := jsonFormat{indent: "    ", eol: "\n"}
	finalEOL := true
	if trimmed := bytes.TrimSpace(body); len(trimmed) > 0 {
		if bytes.Contains(body, []byte("\r\n")) {
			format.eol = "\r\n"
		}
		if m := jsonIndentPattern.FindSubmatch(body); m != nil {
			format.indent = string(m[1])
		}
		format.compact = !bytes.ContainsRune(trimmed, '\n') && len(trimmed) > 2
		finalEOL = bytes.HasSuffix(body, []byte("\n"))
	}

	var out bytes.Buffer
	if hasBOM {
		out.Write(utf8BOM)
	}
	if err := format.write(&out, root, 0); err != nil {
		return nil, err
	}
	if finalEOL {
		out.WriteString(format.eol)
	}
	return out.Bytes(), nil
}

// decodeJSON reads one JSON value into a node tree, preserving object order.
func decodeJSON(data []byte) (*yaml.Node, error) {
	if !json.Valid(data) {
		// Decode again purely for a useful error message.
		var discard any
		if err := json.Unmarshal(data, &discard); err != nil {
			return nil, err
		}
		return nil, errors.New("invalid JSON")
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	node, err := decodeJSONValue(decoder)
	if err != nil {
		return nil, err
	}
	if _, err := decoder.Token(); err != io.EOF {
		return nil, errors.New("unexpected data after the JSON document")
	}
	return node, nil
}

func decodeJSONValue(decoder *json.Decoder) (*yaml.Node, error) {
	token, err := decoder.Token()
	if err != nil {
		return nil, err
	}
	switch typed := token.(type) {
	case json.Delim:
		switch typed {
		case '{':
			node := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
			for decoder.More() {
				keyToken, err := decoder.Token()
				if err != nil {
					return nil, err
				}
				key, ok := keyToken.(string)
				if !ok {
					return nil, errors.New("object key is not a string")
				}
				value, err := decodeJSONValue(decoder)
				if err != nil {
					return nil, err
				}
				node.Content = append(node.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key}, value)
			}
			_, err := decoder.Token()
			return node, err
		case '[':
			node := &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
			for decoder.More() {
				value, err := decodeJSONValue(decoder)
				if err != nil {
					return nil, err
				}
				node.Content = append(node.Content, value)
			}
			_, err := decoder.Token()
			return node, err
		}
		return nil, fmt.Errorf("unexpected %q", typed.String())
	case string:
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: typed}, nil
	case json.Number:
		tag := "!!int"
		if strings.ContainsAny(typed.String(), ".eE") {
			tag = "!!float"
		}
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: tag, Value: typed.String()}, nil
	case bool:
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!bool", Value: strconv.FormatBool(typed)}, nil
	case nil:
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!null", Value: "null"}, nil
	}
	return nil, fmt.Errorf("unexpected token %v", token)
}

type jsonFormat struct {
	indent  string
	eol     string
	compact bool
}

func (f jsonFormat) write(out *bytes.Buffer, node *yaml.Node, depth int) error {
	newline := func(depth int) {
		if f.compact {
			return
		}
		out.WriteString(f.eol)
		for range depth {
			out.WriteString(f.indent)
		}
	}

	switch node.Kind {
	case yaml.MappingNode:
		if len(node.Content) == 0 {
			out.WriteString("{}")
			return nil
		}
		out.WriteByte('{')
		for i := 0; i+1 < len(node.Content); i += 2 {
			if i > 0 {
				out.WriteByte(',')
			}
			newline(depth + 1)
			if err := writeJSONString(out, node.Content[i].Value); err != nil {
				return err
			}
			out.WriteByte(':')
			if !f.compact {
				out.WriteByte(' ')
			}
			if err := f.write(out, node.Content[i+1], depth+1); err != nil {
				return err
			}
		}
		newline(depth)
		out.WriteByte('}')
	case yaml.SequenceNode:
		if len(node.Content) == 0 {
			out.WriteString("[]")
			return nil
		}
		out.WriteByte('[')
		for i, child := range node.Content {
			if i > 0 {
				out.WriteByte(',')
			}
			newline(depth + 1)
			if err := f.write(out, child, depth+1); err != nil {
				return err
			}
		}
		newline(depth)
		out.WriteByte(']')
	case yaml.ScalarNode:
		switch node.Tag {
		case "!!null":
			out.WriteString("null")
		case "!!bool", "!!int", "!!float":
			out.WriteString(node.Value)
		default:
			return writeJSONString(out, node.Value)
		}
	default:
		return errors.New("unexpected node in JSON document")
	}
	return nil
}

func writeJSONString(out *bytes.Buffer, value string) error {
	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		return err
	}
	out.Write(bytes.TrimRight(buffer.Bytes(), "\n"))
	return nil
}

// ---------------------------------------------------------------------------
// yaml

func applyYAML(data []byte, find []Replacement, values Values) ([]byte, error) {
	var documents []*yaml.Node
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	for {
		var document yaml.Node
		err := decoder.Decode(&document)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("invalid YAML: %w", err)
		}
		documents = append(documents, &document)
	}

	// A file without a document (empty, or nothing but comments) keeps its
	// text and gets a new document after it.
	var prefix []byte
	if len(documents) == 0 {
		prefix = data
		if len(prefix) > 0 && !bytes.HasSuffix(prefix, []byte("\n")) {
			prefix = append(bytes.Clone(prefix), '\n')
		}
		documents = []*yaml.Node{{Kind: yaml.DocumentNode}}
	}
	document := documents[0]
	if document.Kind != yaml.DocumentNode {
		return nil, errors.New("invalid YAML: no document")
	}
	if len(document.Content) == 0 {
		document.Content = []*yaml.Node{{Kind: yaml.MappingNode, Tag: "!!map"}}
	}

	changed, err := applyTree(document.Content[0], find, values)
	if err != nil {
		return nil, err
	}
	if !changed {
		return data, nil
	}

	var out bytes.Buffer
	encoder := yaml.NewEncoder(&out)
	encoder.SetIndent(yamlIndent(data))
	for _, doc := range documents {
		if err := encoder.Encode(doc); err != nil {
			return nil, err
		}
	}
	if err := encoder.Close(); err != nil {
		return nil, err
	}
	result := append(bytes.Clone(prefix), out.Bytes()...)
	if bytes.Contains(data, []byte("\r\n")) {
		result = bytes.ReplaceAll(bytes.ReplaceAll(result, []byte("\r\n"), []byte("\n")), []byte("\n"), []byte("\r\n"))
	}
	return result, nil
}

// yamlIndent returns the smallest indentation the document uses (default 2).
func yamlIndent(data []byte) int {
	indent := 0
	for _, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimLeft(line, " ")
		width := len(line) - len(trimmed)
		if width == 0 || strings.TrimSpace(trimmed) == "" || trimmed[0] == '#' {
			continue
		}
		if indent == 0 || width < indent {
			indent = width
		}
	}
	if indent < 2 || indent > 8 {
		return 2
	}
	return indent
}
