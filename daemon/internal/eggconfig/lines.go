package eggconfig

import (
	"slices"
	"strings"
)

// textFile is a file split into lines, remembering how to put it back together.
type textFile struct {
	lines    []string
	eol      string
	finalEOL bool
}

func splitText(data []byte) *textFile {
	text := &textFile{eol: "\n"}
	content := string(data)
	if i := strings.IndexByte(content, '\n'); i > 0 && content[i-1] == '\r' {
		text.eol = "\r\n"
	}
	if content == "" {
		return text
	}
	if strings.HasSuffix(content, "\n") {
		text.finalEOL = true
		content = content[:len(content)-1]
	}
	text.lines = strings.Split(content, "\n")
	if text.eol == "\r\n" {
		for i, line := range text.lines {
			text.lines[i] = strings.TrimSuffix(line, "\r")
		}
	}
	return text
}

func (t *textFile) bytes() []byte {
	if len(t.lines) == 0 {
		return nil
	}
	out := strings.Join(t.lines, t.eol)
	if t.finalEOL {
		out += t.eol
	}
	return []byte(out)
}

// insertAfterContent inserts lines after the last non-blank line of [start, end).
func (t *textFile) insertAfterContent(start, end int, lines ...string) {
	at := end
	for at > start && strings.TrimSpace(t.lines[at-1]) == "" {
		at--
	}
	t.lines = slices.Insert(t.lines, at, lines...)
	if at+len(lines) == len(t.lines) {
		t.finalEOL = true
	}
}

func isBlank(c byte) bool {
	return c == ' ' || c == '\t' || c == '\f'
}

// ---------------------------------------------------------------------------
// properties

func applyProperties(data []byte, find []Replacement, values Values) ([]byte, error) {
	text := splitText(data)
	for _, rep := range find {
		value, err := scalarText(rep.Value, values, nil)
		if err != nil {
			return nil, err
		}
		setProperty(text, rep.Key, value)
	}
	return text.bytes(), nil
}

type property struct {
	end        int    // index of the line after this entry (continuations included)
	key        string // unescaped
	value      string // unescaped
	valueStart int    // offset of the value in the entry's first line
	bare       bool   // the line is just a key, with nothing after it
}

func setProperty(text *textFile, key, value string) {
	escaped := escapePropertyValue(value)
	found := false
	for i := 0; i < len(text.lines); {
		entry, ok := parseProperty(text.lines, i)
		if !ok {
			i++
			continue
		}
		if entry.key != key {
			i = entry.end
			continue
		}
		found = true
		if entry.value == value {
			i = entry.end
			continue
		}
		prefix := text.lines[i][:entry.valueStart]
		if entry.bare {
			prefix += "="
		}
		text.lines = slices.Replace(text.lines, i, entry.end, prefix+escaped)
		i++
	}
	if !found {
		text.insertAfterContent(0, len(text.lines), escapePropertyKey(key)+"="+escaped)
	}
}

func parseProperty(lines []string, index int) (property, bool) {
	line := lines[index]
	pos := 0
	for pos < len(line) && isBlank(line[pos]) {
		pos++
	}
	if pos == len(line) || line[pos] == '#' || line[pos] == '!' {
		return property{}, false
	}

	entry := property{end: index + 1}
	for entry.end < len(lines) && continues(lines[entry.end-1]) {
		entry.end++
	}

	keyStart := pos
	for pos < len(line) {
		c := line[pos]
		if c == '\\' {
			pos += 2
			continue
		}
		if c == '=' || c == ':' || isBlank(c) {
			break
		}
		pos++
	}
	pos = min(pos, len(line))
	keyEnd := pos
	entry.key = unescapeProperty(line[keyStart:keyEnd])

	for pos < len(line) && isBlank(line[pos]) {
		pos++
	}
	separator := false
	if pos < len(line) && (line[pos] == '=' || line[pos] == ':') {
		separator = true
		pos++
		for pos < len(line) && isBlank(line[pos]) {
			pos++
		}
	}
	entry.valueStart = pos
	entry.bare = !separator && pos == keyEnd

	raw := line[pos:]
	for i := index + 1; i < entry.end; i++ {
		raw = strings.TrimSuffix(raw, `\`) + strings.TrimLeft(lines[i], " \t\f")
	}
	entry.value = unescapeProperty(raw)
	return entry, true
}

// continues reports whether a line ends in an unescaped backslash.
func continues(line string) bool {
	count := 0
	for i := len(line) - 1; i >= 0 && line[i] == '\\'; i-- {
		count++
	}
	return count%2 == 1
}

func unescapeProperty(raw string) string {
	if !strings.Contains(raw, `\`) {
		return raw
	}
	var out strings.Builder
	for i := 0; i < len(raw); i++ {
		c := raw[i]
		if c != '\\' || i+1 >= len(raw) {
			out.WriteByte(c)
			continue
		}
		i++
		switch raw[i] {
		case 't':
			out.WriteByte('\t')
		case 'n':
			out.WriteByte('\n')
		case 'r':
			out.WriteByte('\r')
		case 'f':
			out.WriteByte('\f')
		case 'u':
			if r, ok := unicodeEscape(raw[i-1:]); ok {
				out.WriteRune(r)
				i += 4
			} else {
				out.WriteByte('u')
			}
		default:
			out.WriteByte(raw[i])
		}
	}
	return out.String()
}

// unicodeEscape decodes a leading \uXXXX.
func unicodeEscape(text string) (rune, bool) {
	if len(text) < 6 || text[0] != '\\' || text[1] != 'u' {
		return 0, false
	}
	var r rune
	for _, c := range []byte(text[2:6]) {
		switch {
		case c >= '0' && c <= '9':
			r = r<<4 | rune(c-'0')
		case c >= 'a' && c <= 'f':
			r = r<<4 | rune(c-'a'+10)
		case c >= 'A' && c <= 'F':
			r = r<<4 | rune(c-'A'+10)
		default:
			return 0, false
		}
	}
	return r, true
}

// escapePropertyValue applies the minimum escaping a Java properties value
// needs. A backslash that starts a \uXXXX escape is left alone so that escapes
// typed into a variable (Minecraft colour codes) survive.
func escapePropertyValue(value string) string {
	var out strings.Builder
	for i := 0; i < len(value); i++ {
		switch c := value[i]; c {
		case '\\':
			if _, ok := unicodeEscape(value[i:]); ok {
				out.WriteByte('\\')
			} else {
				out.WriteString(`\\`)
			}
		case '\n':
			out.WriteString(`\n`)
		case '\r':
			out.WriteString(`\r`)
		case '\t':
			out.WriteString(`\t`)
		case '\f':
			out.WriteString(`\f`)
		case ' ':
			if i == 0 {
				out.WriteString(`\ `)
			} else {
				out.WriteByte(c)
			}
		default:
			out.WriteByte(c)
		}
	}
	return out.String()
}

func escapePropertyKey(key string) string {
	var out strings.Builder
	for i := 0; i < len(key); i++ {
		switch c := key[i]; c {
		case '\\', ' ', '=', ':':
			out.WriteByte('\\')
			out.WriteByte(c)
		case '#', '!':
			if i == 0 {
				out.WriteByte('\\')
			}
			out.WriteByte(c)
		case '\n':
			out.WriteString(`\n`)
		case '\r':
			out.WriteString(`\r`)
		case '\t':
			out.WriteString(`\t`)
		case '\f':
			out.WriteString(`\f`)
		default:
			out.WriteByte(c)
		}
	}
	return out.String()
}

// ---------------------------------------------------------------------------
// ini

const (
	iniOther = iota
	iniHeader
	iniKey
)

type iniLine struct {
	kind       int
	name       string // section name or key
	valueStart int
	separator  string // text between the key and the value; "" for a bare key
}

type iniSection struct {
	name       string
	start, end int // body lines, header excluded
}

func applyINI(data []byte, find []Replacement, values Values) ([]byte, error) {
	text := splitText(data)
	for _, rep := range find {
		value, err := scalarText(rep.Value, values, nil)
		if err != nil {
			return nil, err
		}
		value = strings.NewReplacer("\r\n", `\n`, "\n", `\n`, "\r", `\n`).Replace(value)
		setINI(text, rep.Key, value)
	}
	return text.bytes(), nil
}

func scanINI(lines []string) ([]iniLine, []iniSection) {
	infos := make([]iniLine, len(lines))
	sections := []iniSection{{}}
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		switch {
		case trimmed == "" || trimmed[0] == ';' || trimmed[0] == '#':
			continue
		case trimmed[0] == '[' && strings.Contains(trimmed, "]"):
			name := strings.TrimSpace(trimmed[1:strings.IndexByte(trimmed, ']')])
			infos[i] = iniLine{kind: iniHeader, name: name}
			sections[len(sections)-1].end = i
			sections = append(sections, iniSection{name: name, start: i + 1})
			continue
		}
		at := strings.IndexByte(line, '=')
		if at < 0 {
			at = strings.IndexByte(line, ':')
		}
		if at < 0 {
			infos[i] = iniLine{kind: iniKey, name: trimmed, valueStart: len(line)}
			continue
		}
		keyEnd := len(strings.TrimRight(line[:at], " \t"))
		valueStart := at + 1
		for valueStart < len(line) && (line[valueStart] == ' ' || line[valueStart] == '\t') {
			valueStart++
		}
		infos[i] = iniLine{
			kind:       iniKey,
			name:       strings.TrimSpace(line[:at]),
			valueStart: valueStart,
			separator:  line[keyEnd:valueStart],
		}
	}
	sections[len(sections)-1].end = len(lines)
	return infos, sections
}

func setINI(text *textFile, match, value string) {
	infos, sections := scanINI(text.lines)

	// Named sections whose name is a prefix of the key, longest first.
	type candidate struct {
		section int
		key     string
	}
	var candidates []candidate
	for i := 1; i < len(sections); i++ {
		name := sections[i].name
		if len(match) > len(name)+1 && match[len(name)] == '.' && strings.EqualFold(match[:len(name)], name) {
			candidates = append(candidates, candidate{section: i, key: match[len(name)+1:]})
		}
	}
	slices.SortStableFunc(candidates, func(a, b candidate) int {
		return len(sections[b.section].name) - len(sections[a.section].name)
	})

	update := func(section iniSection, key string) bool {
		found := false
		for i := section.start; i < section.end; i++ {
			if infos[i].kind == iniKey && strings.EqualFold(infos[i].name, key) {
				text.lines[i] = replaceINIValue(text.lines[i], infos[i], value)
				found = true
			}
		}
		return found
	}

	for _, c := range candidates {
		if update(sections[c.section], c.key) {
			return
		}
	}
	if update(sections[0], match) {
		return
	}

	separator := "="
	for i, info := range infos {
		if info.kind == iniKey && info.separator != "" && info.valueStart < len(text.lines[i]) {
			separator = info.separator
			break
		}
	}

	switch {
	case len(candidates) > 0:
		c := candidates[0]
		text.insertAfterContent(sections[c.section].start, sections[c.section].end, c.key+separator+value)
	case !strings.Contains(match, "."):
		global := sections[0]
		added := []string{match + separator + value}
		if global.end < len(text.lines) && strings.TrimSpace(strings.Join(text.lines[global.start:global.end], "")) == "" {
			// Nothing but blank lines before the first section header.
			added = append(added, "")
			global.end = global.start
		}
		text.insertAfterContent(global.start, global.end, added...)
	default:
		section, key, _ := strings.Cut(match, ".")
		if n := len(text.lines); n > 0 && strings.TrimSpace(text.lines[n-1]) != "" {
			text.lines = append(text.lines, "")
		}
		text.lines = append(text.lines, "["+section+"]", key+separator+value)
		text.finalEOL = true
	}
}

func replaceINIValue(line string, info iniLine, value string) string {
	if info.separator == "" {
		return strings.TrimRight(line, " \t") + "=" + value
	}
	old := strings.TrimRight(line[info.valueStart:], " \t")
	quoted := func(s string) bool { return len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' }
	if quoted(old) && !quoted(value) {
		value = `"` + value + `"`
	}
	return line[:info.valueStart] + value
}

// ---------------------------------------------------------------------------
// file

func applyLines(data []byte, find []Replacement, values Values) ([]byte, error) {
	text := splitText(data)
	for _, rep := range find {
		value, err := scalarText(rep.Value, values, nil)
		if err != nil {
			return nil, err
		}
		matched := false
		for i, line := range text.lines {
			if strings.HasPrefix(line, rep.Key) || line == value {
				if line != value {
					text.lines[i] = value
				}
				matched = true
			}
		}
		if !matched {
			text.insertAfterContent(0, len(text.lines), value)
		}
	}
	return text.bytes(), nil
}
