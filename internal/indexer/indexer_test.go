package indexer

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestDetectLanguage(t *testing.T) {
	tests := []struct {
		path     string
		expected string
	}{
		{"main.go", "Go"},
		{"foo.py", "Python"},
		{"bar.js", "JavaScript"},
		{"baz.ts", "TypeScript"},
		{"component.tsx", "TypeScript"},
		{"main.rs", "Rust"},
		{"Main.java", "Java"},
		{"script.rb", "Ruby"},
		{"app.rb", "Ruby"},
		{"page.html", "HTML"},
		{"style.css", "CSS"},
		{"main.c", "C"},
		{"main.h", "C"},
		{"main.cpp", "C++"},
		{"main.hpp", "C++"},
		{"Program.cs", "C#"},
		{"main.swift", "Swift"},
		{"app.kt", "Kotlin"},
		{"module.scala", "Scala"},
		{"app.ex", "Elixir"},
		{"app.exs", "Elixir"},
		{"file.lua", "Lua"},
		{"main.rs", "Rust"},
		{"schema.sql", "SQL"},
		{"deploy.sh", "Shell"},
		{"script.bash", "Shell"},
		{"install.zsh", "Shell"},
		{"config.fish", "Shell"},
		{"deploy.ps1", "PowerShell"},
		{"Makefile", "Makefile"},
		{"src/makefile", "Makefile"},
		{"Dockerfile", "Dockerfile"},
		{"docker/Dockerfile", "Dockerfile"},
		{"config.yaml", "YAML"},
		{"config.yml", "YAML"},
		{"package.json", "JSON"},
		{"config.toml", "TOML"},
		{"README.md", "Markdown"},
		{"main.dart", "Dart"},
		{"Component.vue", "Vue"},
		{"Component.svelte", "Svelte"},
		{"main.zig", "Zig"},
		{"api.proto", "Protocol Buffers"},
		{"api.graphql", "GraphQL"},
		{"file.tex", "LaTeX"},
		{"unknown.xyz", ""},
		{"noextension", ""},
		{"capital.PY", "Python"},
		{"mixed.Js", "JavaScript"},
	}
	for _, tc := range tests {
		t.Run(tc.path, func(t *testing.T) {
			got := DetectLanguage(tc.path)
			if got != tc.expected {
				t.Errorf("DetectLanguage(%q) = %q, want %q", tc.path, got, tc.expected)
			}
		})
	}
}

func TestWalkRepo(t *testing.T) {
	dir := t.TempDir()
	subdir := filepath.Join(dir, "src")
	if err := os.MkdirAll(subdir, 0755); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"main.go":         "package main",
		"src/util.go":     "package src",
		"README.md":       "# Readme",
		".hidden":         "secret",
		".git/HEAD":       "ref: refs/heads/main",
		"node_modules/x/index.js": "module.exports = {}",
	}
	for path, content := range files {
		full := filepath.Join(dir, path)
		if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	got, err := WalkRepo(dir, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	paths := make(map[string]bool)
	for _, f := range got {
		paths[f.RelPath] = true
	}
	if !paths["main.go"] {
		t.Errorf("expected main.go to be found")
	}
	if !paths["src/util.go"] {
		t.Errorf("expected src/util.go to be found")
	}
	if !paths["README.md"] {
		t.Errorf("expected README.md to be found")
	}
	if paths[".hidden"] {
		t.Errorf(".hidden should be skipped (hidden file)")
	}
	if paths[".git/HEAD"] {
		t.Errorf(".git/HEAD should be skipped (inside .git dir)")
	}
	if paths["node_modules/x/index.js"] {
		t.Errorf("node_modules/x/index.js should be skipped (node_modules dir)")
	}
}

func TestWalkRepo_MaxSize(t *testing.T) {
	dir := t.TempDir()
	content := make([]byte, 1000)
	if err := os.WriteFile(filepath.Join(dir, "large.go"), content, 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "small.go"), []byte("small"), 0644); err != nil {
		t.Fatal(err)
	}
	got, err := WalkRepo(dir, 100)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range got {
		if f.Size > 100 {
			t.Errorf("file %s has size %d, larger than max 100", f.RelPath, f.Size)
		}
	}
	if len(got) != 1 {
		t.Errorf("expected 1 file, got %d", len(got))
	}
}

func TestExtractReferences(t *testing.T) {
	code := `func hello() {
	fmt.Println("hello")
	process(data)
}

func main() {
	hello()
	greet("world")
}`
	refs := ExtractReferences(code)
	names := make([]string, len(refs))
	for i, r := range refs {
		names[i] = r.Name
	}

	if !containsStr(names, "process") {
		t.Errorf("expected process, got %v", names)
	}
	if !containsStr(names, "hello") {
		t.Errorf("expected hello, got %v", names)
	}
	if !containsStr(names, "greet") {
		t.Errorf("expected greet, got %v", names)
	}

	// Control keywords should not appear
	if containsStr(names, "if") {
		t.Errorf("'if' should not be in refs: %v", names)
	}
	if containsStr(names, "for") {
		t.Errorf("'for' should not be in refs: %v", names)
	}

	// Each ref should have context
	for _, r := range refs {
		if r.Context == "" {
			t.Errorf("ref %s should have context", r.Name)
		}
	}
}

func containsStr(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func BenchmarkExtractReferences(b *testing.B) {
	code := `package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Println("hello")
	process(data)
	handleRequest(ctx, req)
	validateInput(name, age)
	calculateSum(a, b, c)
	transformResult(input, options)
	doSomething()
	for i := 0; i < n; i++ {
		if condition {
			processItem(items[i])
		}
	}
	switch val {
	case 1:
		handleOne()
	case 2:
		handleTwo()
	}
}

func helper() {
	anotherCall(x, y, z)
	deeplyNested(a, b, c, d, e)
	lastOne()
}
`
	var buf strings.Builder
	for i := 0; i < 200; i++ {
		buf.WriteString(code)
	}
	bigCode := buf.String()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		refs := ExtractReferences(bigCode)
		_ = refs
	}
}

var wordRe = regexp.MustCompile(`\b(\w+)\s*\(`)

func extractRefsOld(content string) []Ref {
	var refs []Ref
	seen := make(map[string]bool)
	line := 0
	matches := wordRe.FindAllStringSubmatchIndex(content, -1)
	for _, m := range matches {
		nameStart, nameEnd := m[2], m[3]
		name := content[nameStart:nameEnd]
		line++
		// skip control keywords
		switch name {
		case "if", "for", "while", "switch", "case", "return", "catch",
			"throw", "new", "delete", "func", "var", "let", "const",
			"import", "export", "class", "def", "fn", "struct", "enum",
			"trait", "interface", "impl", "type", "package", "go", "defer":
			continue
		}
		lineStr := content[m[0]:m[1]]
		trimmed := stringsTrimSpace(lineStr)
		if len(trimmed) > 120 {
			trimmed = trimmed[:117] + "..."
		}
		key := name + ":" + trimmed
		if seen[key] {
			continue
		}
		seen[key] = true
		refs = append(refs, Ref{
			Name:    name,
			Line:    line,
			Col:     nameStart - m[0],
			Context: trimmed,
		})
	}
	return refs
}

func BenchmarkExtractReferencesOld(b *testing.B) {
	code := `package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Println("hello")
	process(data)
	handleRequest(ctx, req)
	validateInput(name, age)
	calculateSum(a, b, c)
	transformResult(input, options)
	doSomething()
	for i := 0; i < n; i++ {
		if condition {
			processItem(items[i])
		}
	}
	switch val {
	case 1:
		handleOne()
	case 2:
		handleTwo()
	}
}

func helper() {
	anotherCall(x, y, z)
	deeplyNested(a, b, c, d, e)
	lastOne()
}
`
	var buf strings.Builder
	for i := 0; i < 200; i++ {
		buf.WriteString(code)
	}
	bigCode := buf.String()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		refs := extractRefsOld(bigCode)
		_ = refs
	}
}

func TestIsGitRepo(t *testing.T) {
	dir := t.TempDir()
	if IsGitRepo(dir) {
		t.Error("non-git dir should not be a git repo")
	}
	gitDir := filepath.Join(dir, ".git")
	if err := os.MkdirAll(gitDir, 0755); err != nil {
		t.Fatal(err)
	}
	if !IsGitRepo(dir) {
		t.Error("dir with .git should be a git repo")
	}
}
