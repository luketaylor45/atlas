package eggfile

import (
	"reflect"
	"testing"
)

func TestValidateValue(t *testing.T) {
	tests := []struct {
		rules string
		value string
		ok    bool
	}{
		{"required|string|max:20", "survival", true},
		{"required|string|max:20", "", false},
		{"required|string|max:5", "toolong", false},
		{"nullable|string", "", true},
		{"string", "", true},
		{"required|nullable", "", true},

		{"required|integer", "25565", true},
		{"required|integer", "25565.5", false},
		{"required|integer", "abc", false},
		{"required|numeric", "1.5", true},
		{"required|numeric|min:1|max:100", "50", true},
		{"required|numeric|min:1|max:100", "0", false},
		{"required|numeric|min:1|max:100", "101", false},
		{"required|integer|between:1,65535", "65535", true},
		{"required|integer|between:1,65535", "65536", false},
		{"required|string|between:3,5", "abcd", true},
		{"required|string|between:3,5", "ab", false},
		{"required|string|min:3", "ab", false},
		{"required|digits:4", "1234", true},
		{"required|digits:4", "123", false},

		{"required|boolean", "1", true},
		{"required|boolean", "true", true},
		{"required|boolean", "yes", false},
		{"required|in:survival,creative,adventure", "creative", true},
		{"required|in:survival,creative,adventure", "hardcore", false},
		{"required|string|in:0,1", "1", true},
		{"required|not_in:root", "root", false},

		{`required|regex:/^([\w\d._-]+)(\.jar)$/`, "server.jar", true},
		{`required|regex:/^([\w\d._-]+)(\.jar)$/`, "server.exe", false},
		// A pipe inside the pattern does not end the rule.
		{`required|regex:/^(a|b)$/|max:1`, "a", true},
		{`required|regex:/^(a|b)$/|max:1`, "c", false},
		{`required|regex:/^(release|beta)$/i`, "BETA", true},
		{`nullable|regex:/^[a-z]+$/`, "", true},
		// A pattern Go cannot compile is not enforced.
		{`required|regex:/^(?!bad).*$/`, "anything", true},

		{"required|alpha_num", "abc123", true},
		{"required|alpha_num", "abc 123", false},
		{"required|alpha_dash", "my-server_1", true},
		{"required|alpha", "abc1", false},
		{"required|url", "https://example.com/file.zip", true},
		{"required|url", "not a url", false},
		{"required|ip", "192.168.1.1", true},
		{"required|ip", "999.1.1.1", false},

		// Unknown rules never make a value invalid.
		{"required|string|some_future_rule:5", "value", true},
		{"", "anything", true},
	}
	for _, test := range tests {
		err := ValidateValue(test.rules, test.value)
		if (err == nil) != test.ok {
			t.Errorf("ValidateValue(%q, %q) = %v, want ok=%v", test.rules, test.value, err, test.ok)
		}
	}
}

func TestOptions(t *testing.T) {
	if got := Options("required|string|in:a,b,c"); !reflect.DeepEqual(got, []string{"a", "b", "c"}) {
		t.Errorf("options = %v", got)
	}
	if got := Options("required|string|max:20"); got != nil {
		t.Errorf("free text has no options: %v", got)
	}
}
