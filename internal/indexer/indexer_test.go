package indexer

import (
	"os"
	"path/filepath"
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

func TestChunkContent_Empty(t *testing.T) {
	chunks := ChunkContent("", 50, 10)
	if len(chunks) != 0 {
		t.Errorf("expected no chunks for empty content, got %d", len(chunks))
	}
}

func TestChunkContent_Small(t *testing.T) {
	content := "line1\nline2\nline3"
	chunks := ChunkContent(content, 50, 10)
	if len(chunks) != 1 {
		t.Errorf("expected 1 chunk, got %d", len(chunks))
	}
	if chunks[0].Index != 0 {
		t.Errorf("expected index 0, got %d", chunks[0].Index)
	}
	if chunks[0].Content != content {
		t.Errorf("expected content %q, got %q", content, chunks[0].Content)
	}
}

func TestChunkContent_Multiple(t *testing.T) {
	var lines []string
	for i := 0; i < 150; i++ {
		lines = append(lines, "line")
	}
	content := ""
	for _, l := range lines {
		content += l + "\n"
	}
	chunks := ChunkContent(content, 50, 10)
	if len(chunks) < 3 {
		t.Errorf("expected at least 3 chunks, got %d", len(chunks))
	}
	for i, c := range chunks {
		if c.Index != i {
			t.Errorf("chunk %d: expected index %d, got %d", i, i, c.Index)
		}
		if c.Content == "" {
			t.Errorf("chunk %d has empty content", i)
		}
	}
}

func TestChunkContent_Overlap(t *testing.T) {
	lines := make([]string, 60)
	for i := range lines {
		lines[i] = "line"
	}
	content := ""
	for _, l := range lines {
		content += l + "\n"
	}
	chunks := ChunkContent(content, 50, 10)
	if len(chunks) < 2 {
		t.Fatalf("expected at least 2 chunks, got %d", len(chunks))
	}
	if len(chunks) > 4 {
		t.Errorf("expected at most 4 chunks, got %d", len(chunks))
	}
}

func TestChunkContent_InvalidParams(t *testing.T) {
	content := "line1\nline2"
	chunks := ChunkContent(content, 0, -1)
	if len(chunks) != 1 {
		t.Errorf("expected 1 chunk with default params, got %d", len(chunks))
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
