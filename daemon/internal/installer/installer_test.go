package installer

import "testing"

func TestResolveEntry(t *testing.T) {
	tests := []struct {
		name   string
		entry  string
		script string
		want   string
	}{
		{"egg entry wins over shebang", "bash", "#!/bin/ash\necho hi", "bash"},
		{"bash shebang", "", "#!/bin/bash\nset -e", "bash"},
		{"ash shebang", "", "#!/bin/ash\n# comment", "ash"},
		{"env shebang", "", "#!/usr/bin/env bash\necho hi", "bash"},
		{"windows line endings", "", "#!/bin/bash\r\necho hi\r\n", "bash"},
		{"leading blank lines", "", "\n\n#!/bin/ash\necho hi", "ash"},
		{"no shebang falls back to sh", "", "echo hi", "sh"},
		{"blank entry is ignored", "  ", "#!/bin/bash\n", "bash"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := ResolveEntry(test.entry, test.script); got != test.want {
				t.Fatalf("ResolveEntry(%q, %q) = %q, want %q", test.entry, test.script, got, test.want)
			}
		})
	}
}

func TestNormalizeScriptRemovesWindowsLineEndings(t *testing.T) {
	got := NormalizeScript("\ufeff#!/bin/bash\r\necho one\r\necho two")
	want := "#!/bin/bash\necho one\necho two\n"
	if got != want {
		t.Fatalf("NormalizeScript = %q, want %q", got, want)
	}
}

func TestIsInstallerContainer(t *testing.T) {
	if !IsInstallerContainer("/" + ContainerName("abc")) {
		t.Fatal("installer container was not recognised")
	}
	if IsInstallerContainer("abc") {
		t.Fatal("a game container was treated as an installer")
	}
}
