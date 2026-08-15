package search

import "testing"

func TestBuildMatchQuery(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "empty", input: "", want: ""},
		{name: "whitespace only", input: "   ", want: ""},
		{name: "single term", input: "sqlite", want: `"sqlite"*`},
		{name: "single term with internal quote passes through", input: `us"er`, want: `us"er`},
		{name: "multi term", input: "index build", want: `"index" "build"`},
		{name: "multi term collapses whitespace", input: "  index   build  ", want: `"index" "build"`},
		{name: "AND operator passthrough", input: "foo AND bar", want: "foo AND bar"},
		{name: "OR operator passthrough", input: "foo OR bar", want: "foo OR bar"},
		{name: "NOT operator passthrough", input: "foo NOT bar", want: "foo NOT bar"},
		{name: "NEAR operator passthrough", input: "foo NEAR bar", want: "foo NEAR bar"},
		{name: "quoted phrase passthrough", input: `"index build"`, want: `"index build"`},
		{name: "lowercase operators passthrough", input: "foo and bar", want: "foo and bar"},
		{name: "parentheses passthrough", input: "(foo OR bar)", want: "(foo OR bar)"},
		{name: "star passthrough", input: "foo*", want: "foo*"},
		{name: "leading dash passthrough", input: "-foo", want: "-foo"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := buildMatchQuery(tt.input); got != tt.want {
				t.Errorf("buildMatchQuery(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestEscapeFTS5Term(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "plain", input: "foo", want: `"foo"`},
		{name: "internal double quote", input: `f"o`, want: `"f""o"`},
		{name: "parens preserved", input: "f(o)o", want: `"f(o)o"`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := escapeFTS5Term(tt.input); got != tt.want {
				t.Errorf("escapeFTS5Term(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestHasFTS5Operators(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{input: "plain text", want: false},
		{input: "foo AND bar", want: true},
		{input: "foo and bar", want: true},
		{input: "foo OR bar", want: true},
		{input: "foo NOT bar", want: true},
		{input: "foo NEAR bar", want: true},
		{input: "foo*", want: true},
		{input: `"phrase"`, want: true},
		{input: "(foo)", want: true},
	}

	for _, tt := range tests {
		if got := hasFTS5Operators(tt.input); got != tt.want {
			t.Errorf("hasFTS5Operators(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestLikeEscape(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{input: "plain", want: "plain"},
		{input: "100%", want: `100\%`},
		{input: "a_b", want: `a\_b`},
		{input: `back\slash`, want: `back\\slash`},
		{input: `%_\%_`, want: `\%\_\\\%\_`},
	}

	for _, tt := range tests {
		if got := likeEscape(tt.input); got != tt.want {
			t.Errorf("likeEscape(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestTruncateContent(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		maxLen int
		want   string
	}{
		{name: "short content unchanged", input: "abc", maxLen: 10, want: "abc"},
		{name: "exact length unchanged", input: "abcd", maxLen: 4, want: "abcd"},
		{name: "long content truncated", input: "abcdef", maxLen: 3, want: "abc..."},
		{name: "unicode respects runes", input: "héllo", maxLen: 3, want: "hél..."},
		{name: "zero max len", input: "abc", maxLen: 0, want: "..."},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := truncateContent(tt.input, tt.maxLen); got != tt.want {
				t.Errorf("truncateContent(%q, %d) = %q, want %q", tt.input, tt.maxLen, got, tt.want)
			}
		})
	}
}
