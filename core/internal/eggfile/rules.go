package eggfile

import (
	"fmt"
	"net"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

// ValidateValue checks a variable's value against its rules. The rules use
// the Laravel syntax that Pterodactyl eggs are written in
// ("required|string|max:20"). Rules Atlas does not know are ignored, so an
// imported egg never becomes unusable because of one.
//
// The returned error is a sentence fragment that follows the variable's
// name: "must be a whole number".
func ValidateValue(rules string, value string) error {
	parts := splitRules(rules)

	nullable := false
	required := false
	numeric := false
	for _, rule := range parts {
		switch rule.name {
		case "nullable", "sometimes":
			nullable = true
		case "required":
			required = true
		case "numeric", "integer", "int", "digits":
			numeric = true
		}
	}

	if value == "" {
		if required && !nullable {
			return fmt.Errorf("is required")
		}
		// An empty optional value has nothing left to check.
		return nil
	}

	for _, rule := range parts {
		if err := rule.check(value, numeric); err != nil {
			return err
		}
	}
	return nil
}

// Options returns the choices of an "in:" rule, or nil when the variable is
// free text.
func Options(rules string) []string {
	for _, rule := range splitRules(rules) {
		if rule.name == "in" && rule.argument != "" {
			return strings.Split(rule.argument, ",")
		}
	}
	return nil
}

type rule struct {
	name     string
	argument string
}

// splitRules splits on "|", except inside a regex rule, whose pattern may
// contain that character and therefore runs to the end of the string.
func splitRules(rules string) []rule {
	var result []rule
	rest := strings.TrimSpace(rules)
	for rest != "" {
		var part string
		if strings.HasPrefix(rest, "regex:") || strings.HasPrefix(rest, "not_regex:") {
			part, rest = regexPart(rest)
		} else if cut := strings.Index(rest, "|"); cut >= 0 {
			part, rest = rest[:cut], rest[cut+1:]
		} else {
			part, rest = rest, ""
		}

		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		name, argument, _ := strings.Cut(part, ":")
		result = append(result, rule{name: strings.ToLower(strings.TrimSpace(name)), argument: argument})
	}
	return result
}

// regexPart takes a regex rule off the front of a rule string. The pattern
// is delimited ("/…/flags"), so it ends at the last delimiter that is
// followed by a "|" or the end of the string.
func regexPart(rules string) (string, string) {
	_, pattern, _ := strings.Cut(rules, ":")
	if pattern == "" {
		return rules, ""
	}
	delimiter := pattern[0]
	for end := len(pattern) - 1; end > 0; end-- {
		if pattern[end] != delimiter {
			continue
		}
		tail := pattern[end+1:]
		flags, remaining, found := strings.Cut(tail, "|")
		if strings.Trim(flags, "imsxuADSUXJ") != "" {
			continue
		}
		consumed := len(rules) - len(tail) + len(flags)
		if found {
			return rules[:consumed], remaining
		}
		return rules[:consumed], ""
	}
	return rules, ""
}

func (r rule) check(value string, numeric bool) error {
	switch r.name {
	case "numeric":
		if _, err := strconv.ParseFloat(value, 64); err != nil {
			return fmt.Errorf("must be a number")
		}
	case "integer", "int":
		if _, err := strconv.ParseInt(value, 10, 64); err != nil {
			return fmt.Errorf("must be a whole number")
		}
	case "digits":
		if !isDigits(value) {
			return fmt.Errorf("must contain only digits")
		}
		if length, err := strconv.Atoi(r.argument); err == nil && len(value) != length {
			return fmt.Errorf("must be %d digits long", length)
		}
	case "boolean", "bool":
		switch strings.ToLower(value) {
		case "true", "false", "1", "0":
		default:
			return fmt.Errorf("must be true or false")
		}
	case "in":
		for _, option := range strings.Split(r.argument, ",") {
			if value == option {
				return nil
			}
		}
		return fmt.Errorf("must be one of: %s", strings.ReplaceAll(r.argument, ",", ", "))
	case "not_in":
		for _, option := range strings.Split(r.argument, ",") {
			if value == option {
				return fmt.Errorf("cannot be %s", value)
			}
		}
	case "min", "max", "between", "size":
		return r.checkSize(value, numeric)
	case "regex", "not_regex":
		expression, ok := compilePattern(r.argument)
		if !ok {
			// A pattern Go cannot express is not enforced.
			return nil
		}
		if expression.MatchString(value) != (r.name == "regex") {
			return fmt.Errorf("is not in the expected format")
		}
	case "alpha":
		if !matchesAll(value, isLetter) {
			return fmt.Errorf("may only contain letters")
		}
	case "alpha_num":
		if !matchesAll(value, func(c rune) bool { return isLetter(c) || isDigit(c) }) {
			return fmt.Errorf("may only contain letters and numbers")
		}
	case "alpha_dash":
		if !matchesAll(value, func(c rune) bool { return isLetter(c) || isDigit(c) || c == '-' || c == '_' }) {
			return fmt.Errorf("may only contain letters, numbers, dashes and underscores")
		}
	case "url", "active_url":
		parsed, err := url.Parse(value)
		if err != nil || parsed.Scheme == "" || parsed.Host == "" {
			return fmt.Errorf("must be a URL")
		}
	case "ip":
		if net.ParseIP(value) == nil {
			return fmt.Errorf("must be an IP address")
		}
	case "ipv4":
		if ip := net.ParseIP(value); ip == nil || ip.To4() == nil {
			return fmt.Errorf("must be an IPv4 address")
		}
	}
	return nil
}

// checkSize applies min, max, between and size. For a numeric variable they
// compare the number; for text, its length in characters.
func (r rule) checkSize(value string, numeric bool) error {
	bounds := strings.Split(r.argument, ",")
	limits := make([]float64, 0, len(bounds))
	for _, bound := range bounds {
		limit, err := strconv.ParseFloat(strings.TrimSpace(bound), 64)
		if err != nil {
			return nil
		}
		limits = append(limits, limit)
	}
	if len(limits) == 0 || (r.name == "between" && len(limits) < 2) {
		return nil
	}

	measure := float64(len([]rune(value)))
	unit := " characters"
	if numeric {
		number, err := strconv.ParseFloat(value, 64)
		if err != nil {
			// The numeric rule reports this.
			return nil
		}
		measure = number
		unit = ""
	}
	format := func(limit float64) string {
		if !numeric && limit == 1 {
			return "1 character"
		}
		return strconv.FormatFloat(limit, 'f', -1, 64) + unit
	}

	switch r.name {
	case "min":
		if measure < limits[0] {
			return fmt.Errorf("must be at least %s", format(limits[0]))
		}
	case "max":
		if measure > limits[0] {
			return fmt.Errorf("must be at most %s", format(limits[0]))
		}
	case "between":
		if measure < limits[0] || measure > limits[1] {
			return fmt.Errorf("must be between %s and %s", strconv.FormatFloat(limits[0], 'f', -1, 64), format(limits[1]))
		}
	case "size":
		if measure != limits[0] {
			return fmt.Errorf("must be exactly %s", format(limits[0]))
		}
	}
	return nil
}

// compilePattern turns a PHP-style delimited pattern ("/^a+$/i") into a Go
// regular expression.
func compilePattern(pattern string) (*regexp.Regexp, bool) {
	if len(pattern) < 2 {
		return nil, false
	}
	delimiter := pattern[0]
	end := strings.LastIndexByte(pattern, delimiter)
	if end <= 0 {
		return nil, false
	}
	body, flags := pattern[1:end], pattern[end+1:]

	goFlags := ""
	for _, flag := range flags {
		switch flag {
		case 'i', 'm', 's':
			goFlags += string(flag)
		}
	}
	if goFlags != "" {
		body = "(?" + goFlags + ")" + body
	}
	expression, err := regexp.Compile(body)
	if err != nil {
		return nil, false
	}
	return expression, true
}

func isLetter(c rune) bool { return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') }
func isDigit(c rune) bool  { return c >= '0' && c <= '9' }

func isDigits(value string) bool { return value != "" && matchesAll(value, isDigit) }

func matchesAll(value string, allowed func(rune) bool) bool {
	for _, c := range value {
		if !allowed(c) {
			return false
		}
	}
	return true
}
