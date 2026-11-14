package eggconfig

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// The xml parser never re-serialises a document. It reads the file with
// encoding/xml to learn where every tag and attribute value sits, then splices
// the new text into the original bytes.

type xmlAttr struct {
	name       string // as written, prefix included
	value      string // decoded
	start, end int    // byte range of the raw value, quotes excluded
	quote      byte
}

type xmlElement struct {
	name     string // as written, prefix included
	local    string
	attrs    []xmlAttr
	children []*xmlElement

	tagStart, tagEnd int // start tag
	closeStart       int // start of the end tag
	selfClosing      bool
	attrInsert       int // where a new attribute goes
}

func (e *xmlElement) attr(name string) *xmlAttr {
	for i := range e.attrs {
		if e.attrs[i].name == name {
			return &e.attrs[i]
		}
	}
	return nil
}

type splice struct {
	start, end int
	text       string
}

var xmlAttrValuePattern = regexp.MustCompile(`^\[([A-Za-z_][\w:.-]*)='(.*)'\]$`)

func applyXML(data []byte, find []Replacement, values Values) ([]byte, error) {
	out := data
	for _, rep := range find {
		text, err := scalarText(rep.Value, values, nil)
		if err != nil {
			return nil, err
		}
		path, err := parseXMLPath(rep.Key)
		if err != nil {
			return nil, err
		}
		// Offsets change with every edit, so each replacement reads the
		// document afresh.
		document, err := parseXML(out)
		if err != nil {
			return nil, fmt.Errorf("invalid XML: %w", err)
		}

		var edits []splice
		for _, element := range path.find(document) {
			var edit *splice
			switch m := xmlAttrValuePattern.FindStringSubmatch(text); {
			case path.attr != "":
				edit = element.setAttr(path.attr, text)
			case m != nil:
				edit = element.setAttr(m[1], m[2])
			case path.selectsByAttr() && element.attr("value") != nil:
				edit = element.setAttr("value", text)
			default:
				if edit, err = element.setText(out, text); err != nil {
					return nil, fmt.Errorf("%s: %w", rep.Key, err)
				}
			}
			if edit != nil {
				edits = append(edits, *edit)
			}
		}
		out = applySplices(out, edits)
	}
	if !bytes.Equal(out, data) {
		if _, err := parseXML(out); err != nil {
			return nil, fmt.Errorf("refusing to write malformed XML: %w", err)
		}
	}
	return out, nil
}

func applySplices(data []byte, edits []splice) []byte {
	if len(edits) == 0 {
		return data
	}
	sort.Slice(edits, func(i, j int) bool { return edits[i].start < edits[j].start })
	var out bytes.Buffer
	position := 0
	for _, edit := range edits {
		if edit.start < position {
			continue // overlapping edit; cannot happen for distinct elements
		}
		out.Write(data[position:edit.start])
		out.WriteString(edit.text)
		position = edit.end
	}
	out.Write(data[position:])
	return out.Bytes()
}

func (e *xmlElement) setAttr(name, value string) *splice {
	if existing := e.attr(name); existing != nil {
		if existing.value == value {
			return nil
		}
		return &splice{start: existing.start, end: existing.end, text: escapeXMLAttr(value, existing.quote)}
	}
	return &splice{start: e.attrInsert, end: e.attrInsert, text: " " + name + `="` + escapeXMLAttr(value, '"') + `"`}
}

func (e *xmlElement) setText(data []byte, value string) (*splice, error) {
	if len(e.children) > 0 {
		return nil, fmt.Errorf("<%s> has child elements, its text cannot be replaced", e.name)
	}
	escaped := escapeXMLText(value)
	if !e.selfClosing {
		return &splice{start: e.tagEnd, end: e.closeStart, text: escaped}, nil
	}
	if value == "" {
		return nil, nil
	}
	tag := data[e.tagStart:e.tagEnd]
	open := string(bytes.TrimRight(bytes.TrimSuffix(tag, []byte("/>")), " \t\r\n"))
	return &splice{start: e.tagStart, end: e.tagEnd, text: open + ">" + escaped + "</" + e.name + ">"}, nil
}

func escapeXMLText(value string) string {
	return strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;").Replace(value)
}

func escapeXMLAttr(value string, quote byte) string {
	pairs := []string{"&", "&amp;", "<", "&lt;", "\n", "&#10;", "\r", "&#13;", "\t", "&#9;"}
	if quote == '\'' {
		pairs = append(pairs, "'", "&apos;")
	} else {
		pairs = append(pairs, `"`, "&quot;")
	}
	return strings.NewReplacer(pairs...).Replace(value)
}

// parseXML returns a virtual node whose only child is the root element.
func parseXML(data []byte) (*xmlElement, error) {
	decoder := xml.NewDecoder(bytes.NewReader(data))
	document := &xmlElement{}
	stack := []*xmlElement{document}

	for {
		start := int(decoder.InputOffset())
		token, err := decoder.RawToken()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		end := int(decoder.InputOffset())
		parent := stack[len(stack)-1]

		switch typed := token.(type) {
		case xml.StartElement:
			if start < 0 || end > len(data) || end <= start || data[start] != '<' {
				return nil, errors.New("cannot locate element in source")
			}
			element := &xmlElement{name: xmlName(typed.Name), local: typed.Name.Local, tagStart: start, tagEnd: end}
			if err := element.scanTag(data, typed.Attr); err != nil {
				return nil, err
			}
			parent.children = append(parent.children, element)
			stack = append(stack, element)
		case xml.EndElement:
			if len(stack) == 1 || parent.name != xmlName(typed.Name) {
				return nil, fmt.Errorf("unexpected </%s>", xmlName(typed.Name))
			}
			parent.closeStart = start
			// A self-closing tag yields a synthetic end token that consumes nothing.
			parent.selfClosing = start == end
			if parent.selfClosing && !bytes.HasSuffix(data[parent.tagStart:parent.tagEnd], []byte("/>")) {
				return nil, errors.New("cannot locate element in source")
			}
			stack = stack[:len(stack)-1]
		}
	}
	if len(stack) != 1 {
		return nil, fmt.Errorf("<%s> is not closed", stack[len(stack)-1].name)
	}
	if len(document.children) != 1 {
		return nil, errors.New("document must have exactly one root element")
	}
	return document, nil
}

func xmlName(name xml.Name) string {
	if name.Space != "" {
		return name.Space + ":" + name.Local
	}
	return name.Local
}

func isXMLSpace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\r' || c == '\n'
}

// scanTag records where each attribute value of the start tag sits.
func (e *xmlElement) scanTag(data []byte, decoded []xml.Attr) error {
	i, end := e.tagStart+1, e.tagEnd
	for i < end && !isXMLSpace(data[i]) && data[i] != '/' && data[i] != '>' {
		i++
	}
	e.attrInsert = i
	for {
		for i < end && isXMLSpace(data[i]) {
			i++
		}
		if i >= end || data[i] == '/' || data[i] == '>' {
			break
		}
		nameStart := i
		for i < end && !isXMLSpace(data[i]) && data[i] != '=' && data[i] != '/' && data[i] != '>' {
			i++
		}
		name := string(data[nameStart:i])
		for i < end && isXMLSpace(data[i]) {
			i++
		}
		if i >= end || data[i] != '=' {
			return fmt.Errorf("<%s>: attribute %q has no value", e.name, name)
		}
		i++
		for i < end && isXMLSpace(data[i]) {
			i++
		}
		if i >= end || (data[i] != '"' && data[i] != '\'') {
			return fmt.Errorf("<%s>: attribute %q is not quoted", e.name, name)
		}
		quote := data[i]
		i++
		valueStart := i
		for i < end && data[i] != quote {
			i++
		}
		if i >= end {
			return fmt.Errorf("<%s>: attribute %q is not terminated", e.name, name)
		}
		e.attrs = append(e.attrs, xmlAttr{name: name, start: valueStart, end: i, quote: quote})
		i++
		e.attrInsert = i
	}
	if len(e.attrs) != len(decoded) {
		return fmt.Errorf("<%s>: cannot locate attributes in source", e.name)
	}
	for i := range e.attrs {
		if e.attrs[i].name != xmlName(decoded[i].Name) {
			return fmt.Errorf("<%s>: cannot locate attributes in source", e.name)
		}
		e.attrs[i].value = decoded[i].Value
	}
	return nil
}

// ---------------------------------------------------------------------------
// paths

type xmlPredicate struct {
	index    int // one-based position; 0 when this is an attribute test
	attr     string
	value    string
	hasValue bool
}

type xmlStep struct {
	name       string
	descendant bool
	predicates []xmlPredicate
}

type xmlPath struct {
	steps    []xmlStep
	attr     string // trailing @attr
	absolute bool
}

var (
	xmlTrailingAttr  = regexp.MustCompile(`^(.*?)[./]?@([A-Za-z_][\w:.-]*)$`)
	xmlAttrPredicate = regexp.MustCompile(`^@([A-Za-z_][\w:.-]*)\s*(?:=\s*(?:'([^']*)'|"([^"]*)"))?$`)
)

func (p *xmlPath) selectsByAttr() bool {
	for _, predicate := range p.steps[len(p.steps)-1].predicates {
		if predicate.attr != "" {
			return true
		}
	}
	return false
}

func parseXMLPath(key string) (*xmlPath, error) {
	path := &xmlPath{}
	rest := strings.TrimSpace(key)
	if m := xmlTrailingAttr.FindStringSubmatch(rest); m != nil && !strings.HasSuffix(m[1], "[") {
		rest, path.attr = m[1], m[2]
	}
	if strings.HasPrefix(rest, "/") {
		path.absolute = true
		rest = rest[1:]
	}

	// Split on "/" when the path uses it, otherwise on ".", ignoring both
	// inside predicates.
	separator := byte('.')
	depth := 0
	for i := 0; i < len(rest); i++ {
		switch rest[i] {
		case '[':
			depth++
		case ']':
			depth--
		case '/':
			if depth == 0 {
				separator = '/'
			}
		}
	}
	if path.absolute {
		separator = '/'
	}

	var parts []string
	depth = 0
	begin := 0
	for i := 0; i < len(rest); i++ {
		switch c := rest[i]; {
		case c == '[':
			depth++
		case c == ']':
			depth--
		case c == separator && depth == 0:
			parts = append(parts, rest[begin:i])
			begin = i + 1
		}
	}
	if depth != 0 {
		return nil, fmt.Errorf("invalid xml path %q: unbalanced brackets", key)
	}
	parts = append(parts, rest[begin:])

	descendant := false
	for _, part := range parts {
		if part == "" {
			if separator != '/' || descendant {
				return nil, fmt.Errorf("invalid xml path %q: empty step", key)
			}
			descendant = true
			continue
		}
		step, err := parseXMLStep(part)
		if err != nil {
			return nil, fmt.Errorf("invalid xml path %q: %w", key, err)
		}
		step.descendant = descendant
		descendant = false
		path.steps = append(path.steps, step)
	}
	if len(path.steps) == 0 || descendant {
		return nil, fmt.Errorf("invalid xml path %q: no element", key)
	}
	return path, nil
}

func parseXMLStep(part string) (xmlStep, error) {
	name, predicates, _ := strings.Cut(part, "[")
	step := xmlStep{name: strings.TrimSpace(name)}
	if step.name == "" || strings.ContainsAny(step.name, "@]'\" ") {
		return step, fmt.Errorf("bad element name %q", name)
	}
	if predicates == "" {
		if strings.Contains(part, "[") {
			return step, errors.New("empty predicate")
		}
		return step, nil
	}
	predicates = "[" + predicates
	for predicates != "" {
		if predicates[0] != '[' {
			return step, fmt.Errorf("unexpected %q", predicates)
		}
		end := strings.IndexByte(predicates, ']')
		if end < 0 {
			return step, errors.New("missing \"]\"")
		}
		inner := strings.TrimSpace(predicates[1:end])
		predicates = predicates[end+1:]

		if n, err := strconv.Atoi(inner); err == nil {
			if n < 1 {
				return step, errors.New("positions start at 1")
			}
			step.predicates = append(step.predicates, xmlPredicate{index: n})
			continue
		}
		m := xmlAttrPredicate.FindStringSubmatch(inner)
		if m == nil {
			return step, fmt.Errorf("unsupported predicate [%s]", inner)
		}
		predicate := xmlPredicate{attr: m[1]}
		if strings.Contains(inner, "=") {
			predicate.hasValue = true
			predicate.value = m[2] + m[3]
		}
		step.predicates = append(step.predicates, predicate)
	}
	return step, nil
}

func (p *xmlPath) find(document *xmlElement) []*xmlElement {
	found := p.run(document, p.steps)
	if len(found) == 0 && !p.absolute && !p.steps[0].descendant {
		steps := append([]xmlStep(nil), p.steps...)
		steps[0].descendant = true
		found = p.run(document, steps)
	}
	return found
}

func (p *xmlPath) run(document *xmlElement, steps []xmlStep) []*xmlElement {
	context := []*xmlElement{document}
	for _, step := range steps {
		var next []*xmlElement
		seen := map[*xmlElement]bool{}
		for _, parent := range context {
			for _, element := range step.match(parent) {
				if !seen[element] {
					seen[element] = true
					next = append(next, element)
				}
			}
		}
		context = next
	}
	return context
}

// match returns the children of parent (or, for a descendant step, the
// children of every element below it) selected by the step.
func (s xmlStep) match(parent *xmlElement) []*xmlElement {
	var selected []*xmlElement
	for _, child := range parent.children {
		if s.name == "*" || s.name == child.name || s.name == child.local {
			selected = append(selected, child)
		}
	}
	for _, predicate := range s.predicates {
		if predicate.index > 0 {
			if predicate.index > len(selected) {
				selected = nil
			} else {
				selected = selected[predicate.index-1 : predicate.index]
			}
			continue
		}
		var kept []*xmlElement
		for _, element := range selected {
			attr := element.attr(predicate.attr)
			if attr != nil && (!predicate.hasValue || attr.value == predicate.value) {
				kept = append(kept, element)
			}
		}
		selected = kept
	}
	if s.descendant {
		for _, child := range parent.children {
			selected = append(selected, s.match(child)...)
		}
	}
	return selected
}
