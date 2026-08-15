package git

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()

	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func newTestRepo(t *testing.T) string {
	t.Helper()

	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}

	dir := t.TempDir()
	runGit(t, dir, "init", "-b", "main")
	runGit(t, dir, "config", "user.email", "test@example.com")
	runGit(t, dir, "config", "user.name", "Test User")

	writeTestFile(t, dir, "main.go", "package main\nfunc main() {}\n")
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-m", "initial commit")

	return dir
}

func writeTestFile(t *testing.T, dir, name, content string) {
	t.Helper()

	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestIsRepo(t *testing.T) {
	dir := t.TempDir()
	if IsRepo(dir) {
		t.Error("expected non-repo dir to be false")
	}

	repo := newTestRepo(t)
	if !IsRepo(repo) {
		t.Error("expected git repo to be true")
	}
}

func TestItoa(t *testing.T) {
	tests := []struct {
		input int
		want  string
	}{
		{input: 0, want: "0"},
		{input: 7, want: "7"},
		{input: 12345, want: "12345"},
	}
	for _, tt := range tests {
		if got := itoa(tt.input); got != tt.want {
			t.Errorf("itoa(%d) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestRecentCommits(t *testing.T) {
	repo := newTestRepo(t)

	out, err := RecentCommits(context.Background(), repo, 5)
	if err != nil {
		t.Fatalf("RecentCommits: %v", err)
	}
	if !strings.Contains(out, "initial commit") {
		t.Errorf("expected 'initial commit' in output, got: %q", out)
	}
	if !strings.Contains(out, "Test User") {
		t.Errorf("expected author 'Test User' in output, got: %q", out)
	}
}

func TestFileHistory(t *testing.T) {
	repo := newTestRepo(t)

	out, err := FileHistory(context.Background(), repo, "main.go", 5)
	if err != nil {
		t.Fatalf("FileHistory: %v", err)
	}
	if !strings.Contains(out, "initial commit") {
		t.Errorf("expected 'initial commit' in output, got: %q", out)
	}
}

func TestFileHistoryMissingFile(t *testing.T) {
	repo := newTestRepo(t)

	out, err := FileHistory(context.Background(), repo, "does-not-exist.go", 5)
	if err != nil {
		t.Fatalf("FileHistory: %v", err)
	}
	if strings.Contains(out, "initial commit") {
		t.Errorf("expected no commits for missing file, got output: %q", out)
	}
}

func TestBlame(t *testing.T) {
	repo := newTestRepo(t)

	out, err := Blame(context.Background(), repo, "main.go", 0, 0)
	if err != nil {
		t.Fatalf("Blame: %v", err)
	}
	if !strings.Contains(out, "Test User") {
		t.Errorf("expected author 'Test User' in blame output, got: %q", out)
	}
}

func TestBlameLineRange(t *testing.T) {
	repo := newTestRepo(t)

	out, err := Blame(context.Background(), repo, "main.go", 1, 1)
	if err != nil {
		t.Fatalf("Blame: %v", err)
	}
	if !strings.Contains(out, "Test User") {
		t.Errorf("expected author 'Test User' in blame output, got: %q", out)
	}
}

func TestDiff(t *testing.T) {
	repo := newTestRepo(t)

	writeTestFile(t, repo, "main.go", "package main\nfunc main() { println(\"v2\") }\n")
	runGit(t, repo, "add", ".")
	runGit(t, repo, "commit", "-m", "second commit")

	out, err := Diff(context.Background(), repo, "HEAD~1", "HEAD", 120)
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}
	if !strings.Contains(out, "second commit") && !strings.Contains(out, "v2") {
		t.Errorf("expected change in diff output, got: %q", out)
	}
	if !strings.Contains(out, "main.go") {
		t.Errorf("expected main.go in diff stat, got: %q", out)
	}
}

func TestDiffSameRef(t *testing.T) {
	repo := newTestRepo(t)

	out, err := Diff(context.Background(), repo, "HEAD", "HEAD", 120)
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}
	if strings.Contains(out, "diff --git") {
		t.Errorf("expected empty diff, got: %q", out)
	}
}
