package main

import "testing"

func TestParseSourcePath(t *testing.T) {
	tests := []struct {
		input      string
		wantBase   string
		wantInRepo string
	}{
		// No // separator
		{"/local/path", "/local/path", ""},
		{"git@github.com:org/repo.git", "git@github.com:org/repo.git", ""},

		// SSH git URL with // separator
		{"git@github.com:org/repo.git//scripts/check.py", "git@github.com:org/repo.git", "scripts/check.py"},
		{"git@github.com:org/repo.git//subdir", "git@github.com:org/repo.git", "subdir"},
		{"git@github.com:org/repo.git//deep/nested/dir", "git@github.com:org/repo.git", "deep/nested/dir"},

		// HTTPS git URL with // separator — should not confuse https:// with //
		{"https://github.com/org/repo.git//scripts/check.py", "https://github.com/org/repo.git", "scripts/check.py"},
		{"https://github.com/org/repo", "https://github.com/org/repo", ""},
		{"https://github.com/org/repo//tools", "https://github.com/org/repo", "tools"},

		// HTTP
		{"http://github.com/org/repo//tools/main.sh", "http://github.com/org/repo", "tools/main.sh"},
	}

	for _, tt := range tests {
		base, inRepo := parseSourcePath(tt.input)
		if base != tt.wantBase || inRepo != tt.wantInRepo {
			t.Errorf("parseSourcePath(%q) = (%q, %q), want (%q, %q)",
				tt.input, base, inRepo, tt.wantBase, tt.wantInRepo)
		}
	}
}

func TestExtractJSON(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{`{"key": "value"}`, `{"key": "value"}`},
		{`some text {"key": "value"} more text`, `{"key": "value"}`},
		{`{"nested": {"inner": "val"}}`, `{"nested": {"inner": "val"}}`},
		{`{"with_braces": "a{b}c"}`, `{"with_braces": "a{b}c"}`},
		{`no json here`, ""},
		{`{"escaped": "line\"end"}`, `{"escaped": "line\"end"}`},
	}

	for _, tt := range tests {
		got := extractJSON(tt.input)
		if got != tt.want {
			t.Errorf("extractJSON(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
