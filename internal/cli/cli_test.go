package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/cryskram/relith/internal/db"
)

// isolatedEnv points every config lookup at fresh temp directories so tests
// never touch (or read from) the developer's real relith data/config.
func isolatedEnv(t *testing.T) {
	t.Helper()
	t.Setenv("RELITH_CORE_DATA_DIR", t.TempDir())
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
}

var ansiRe = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func stripANSI(s string) string {
	return ansiRe.ReplaceAllString(s, "")
}

// runCLI executes the root command with the given args, capturing stdout.
// Commands print via fmt to os.Stdout, so the test swaps os.Stdout for a pipe.
func runCLI(t *testing.T, args ...string) (string, error) {
	t.Helper()

	rootCmd.SetArgs(args)
	rootCmd.SilenceErrors = true
	rootCmd.SilenceUsage = true

	orig := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w
	defer func() {
		os.Stdout = orig
	}()

	runErr := rootCmd.Execute()
	_ = w.Close()
	out, _ := io.ReadAll(r)
	_ = r.Close()

	return string(out), runErr
}

// writeFixtureRepo creates files (keyed by relative path) under a fresh temp dir.
func writeFixtureRepo(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, content := range files {
		p := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", filepath.Dir(p), err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", p, err)
		}
	}
	return dir
}

func writeGitConfig(t *testing.T, repoDir, content string) {
	t.Helper()
	p := filepath.Join(repoDir, ".git", "config")
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatalf("mkdir .git: %v", err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatalf("write git config: %v", err)
	}
}

func TestVersionCommand(t *testing.T) {
	isolatedEnv(t)
	out, err := runCLI(t, "version")
	if err != nil {
		t.Fatalf("version: %v", err)
	}
	if got := strings.TrimSpace(out); got != Version {
		t.Errorf("version = %q, want %q", got, Version)
	}
}

func TestExtractRemoteURL(t *testing.T) {
	tests := []struct {
		name   string
		config string
		want   string
	}{
		{name: "origin url", config: "[remote \"origin\"]\n\turl = https://github.com/foo/bar.git", want: "https://github.com/foo/bar.git"},
		{name: "quoted url", config: "[remote \"origin\"]\n\turl = \"git@github.com:foo/bar.git\"", want: "git@github.com:foo/bar.git"},
		{name: "other remote ignored", config: "[remote \"upstream\"]\n\turl = https://example.com/up.git\n[remote \"origin\"]\n\turl = https://github.com/foo/bar.git", want: "https://github.com/foo/bar.git"},
		{name: "no origin", config: "[remote \"upstream\"]\n\turl = https://example.com/up.git", want: ""},
		{name: "empty", config: "", want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := extractRemoteURL(tt.config); got != tt.want {
				t.Errorf("extractRemoteURL() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestTruncateForDisplay(t *testing.T) {
	tests := []struct {
		in   string
		n    int
		want string
	}{
		{in: "short", n: 10, want: "short"},
		{in: "abcdefghijklmno", n: 5, want: "abcde..."},
		{in: "", n: 5, want: ""},
	}
	for _, tt := range tests {
		if got := truncateForDisplay(tt.in, tt.n); got != tt.want {
			t.Errorf("truncateForDisplay(%q, %d) = %q, want %q", tt.in, tt.n, got, tt.want)
		}
	}
}

func TestToInt64(t *testing.T) {
	tests := []struct {
		in   interface{}
		want int64
	}{
		{in: int64(42), want: 42},
		{in: float64(3.9), want: 3},
		{in: int(7), want: 7},
		{in: "nope", want: 0},
		{in: nil, want: 0},
	}
	for _, tt := range tests {
		if got := toInt64(tt.in); got != tt.want {
			t.Errorf("toInt64(%v) = %d, want %d", tt.in, got, tt.want)
		}
	}
}

func TestRepoAddCommand(t *testing.T) {
	isolatedEnv(t)

	t.Run("missing path", func(t *testing.T) {
		missing := filepath.Join(t.TempDir(), "does-not-exist")
		_, err := runCLI(t, "repo", "add", missing)
		if err == nil || !strings.Contains(err.Error(), "stat path") {
			t.Fatalf("expected stat path error, got %v", err)
		}
	})

	t.Run("not a directory", func(t *testing.T) {
		f := filepath.Join(t.TempDir(), "file.txt")
		if err := os.WriteFile(f, []byte("x"), 0o644); err != nil {
			t.Fatalf("write: %v", err)
		}
		_, err := runCLI(t, "repo", "add", f)
		if err == nil || !strings.Contains(err.Error(), "not a directory") {
			t.Fatalf("expected not a directory error, got %v", err)
		}
	})

	t.Run("success", func(t *testing.T) {
		repoDir := writeFixtureRepo(t, map[string]string{"main.go": "package main\n"})
		out, err := runCLI(t, "repo", "add", repoDir)
		if err != nil {
			t.Fatalf("repo add: %v", err)
		}
		if !strings.Contains(out, "Added repository: id=1") {
			t.Errorf("output missing Added repository line: %q", out)
		}
	})

	t.Run("duplicate", func(t *testing.T) {
		repoDir := writeFixtureRepo(t, map[string]string{"main.go": "package main\n"})
		if _, err := runCLI(t, "repo", "add", repoDir); err != nil {
			t.Fatalf("first add: %v", err)
		}
		_, err := runCLI(t, "repo", "add", repoDir)
		if err == nil || !strings.Contains(err.Error(), "already exists") {
			t.Fatalf("expected already exists error, got %v", err)
		}
	})

	t.Run("git remote detected", func(t *testing.T) {
		const want = "https://github.com/cryskram/relith.git"
		repoDir := writeFixtureRepo(t, map[string]string{"main.go": "package main\n"})
		writeGitConfig(t, repoDir, fmt.Sprintf("[remote \"origin\"]\n\turl = %s\n", want))

		if _, err := runCLI(t, "repo", "add", repoDir); err != nil {
			t.Fatalf("repo add: %v", err)
		}

		app, err := openDB()
		if err != nil {
			t.Fatalf("openDB: %v", err)
		}
		defer app.close()

		absPath, _ := filepath.Abs(repoDir)
		repo, err := db.New(app.db).GetRepoByPath(context.Background(), absPath)
		if err != nil {
			t.Fatalf("get repo: %v", err)
		}
		if !repo.RemoteUrl.Valid || repo.RemoteUrl.String != want {
			t.Errorf("remote url = %v, want %s", repo.RemoteUrl, want)
		}
	})
}

func TestRepoListCommand(t *testing.T) {
	isolatedEnv(t)

	t.Run("empty", func(t *testing.T) {
		out, err := runCLI(t, "repo", "list")
		if err != nil {
			t.Fatalf("repo list: %v", err)
		}
		if !strings.Contains(out, "No repositories indexed.") {
			t.Errorf("expected empty message, got %q", out)
		}
	})

	t.Run("populated", func(t *testing.T) {
		repoDir := writeFixtureRepo(t, map[string]string{"main.go": "package main\n"})
		if _, err := runCLI(t, "repo", "add", repoDir); err != nil {
			t.Fatalf("repo add: %v", err)
		}
		out, err := runCLI(t, "repo", "list")
		if err != nil {
			t.Fatalf("repo list: %v", err)
		}
		clean := stripANSI(out)
		if !strings.Contains(clean, "(1)") || !strings.Contains(clean, filepath.Base(repoDir)) {
			t.Errorf("list output missing repo, got %q", clean)
		}
	})
}

func TestRepoRemoveCommand(t *testing.T) {
	isolatedEnv(t)

	t.Run("by id", func(t *testing.T) {
		repoDir := writeFixtureRepo(t, map[string]string{"main.go": "package main\n"})
		if _, err := runCLI(t, "repo", "add", repoDir); err != nil {
			t.Fatalf("repo add: %v", err)
		}
		out, err := runCLI(t, "repo", "remove", "1")
		if err != nil {
			t.Fatalf("repo remove: %v", err)
		}
		if !strings.Contains(out, "Removed repository: id=1") {
			t.Errorf("expected Removed repository line, got %q", out)
		}

		out, err = runCLI(t, "repo", "list")
		if err != nil {
			t.Fatalf("repo list: %v", err)
		}
		if !strings.Contains(out, "No repositories indexed.") {
			t.Errorf("repo should be gone, got %q", out)
		}
	})

	t.Run("by name", func(t *testing.T) {
		repoDir := writeFixtureRepo(t, map[string]string{"main.go": "package main\n"})
		name := filepath.Base(repoDir)
		if _, err := runCLI(t, "repo", "add", repoDir); err != nil {
			t.Fatalf("repo add: %v", err)
		}
		out, err := runCLI(t, "repo", "remove", name)
		if err != nil {
			t.Fatalf("repo remove: %v", err)
		}
		if !strings.Contains(out, "Removed repository") {
			t.Errorf("expected Removed repository line, got %q", out)
		}
	})

	t.Run("not found", func(t *testing.T) {
		_, err := runCLI(t, "repo", "remove", "ghost")
		if err == nil || !strings.Contains(err.Error(), "repository not found") {
			t.Fatalf("expected repository not found error, got %v", err)
		}
	})
}

func TestIndexCommand(t *testing.T) {
	isolatedEnv(t)

	t.Run("repo not found", func(t *testing.T) {
		repoDir := writeFixtureRepo(t, map[string]string{"main.go": "package main\n"})
		_, err := runCLI(t, "index", repoDir)
		if err == nil || !strings.Contains(err.Error(), "repository not found") {
			t.Fatalf("expected repository not found error, got %v", err)
		}
	})

	t.Run("no repositories to index", func(t *testing.T) {
		out, err := runCLI(t, "index")
		if err != nil {
			t.Fatalf("index: %v", err)
		}
		if !strings.Contains(out, "No repositories to index.") {
			t.Errorf("expected empty message, got %q", out)
		}
	})

	t.Run("indexes a repository", func(t *testing.T) {
		repoDir := writeFixtureRepo(t, map[string]string{
			"main.go": "package main\n\nfunc RunServer() {}\n",
			"app.go":  "package main\n\nfunc helper() int { return 1 }\n",
		})
		if _, err := runCLI(t, "repo", "add", repoDir); err != nil {
			t.Fatalf("repo add: %v", err)
		}
		out, err := runCLI(t, "index", repoDir)
		if err != nil {
			t.Fatalf("index: %v", err)
		}
		if !strings.Contains(out, "Indexing:") || !strings.Contains(out, "Indexed: 2 files") {
			t.Errorf("unexpected index output: %q", out)
		}
	})
}

func TestSearchCommand(t *testing.T) {
	isolatedEnv(t)
	repoDir := writeFixtureRepo(t, map[string]string{
		"auth.go":  "package main\n\nfunc AuthToken() string { return \"tok\" }\n",
		"auth2.go": "package main\n\nvar AuthToken = \"x\"\n",
	})
	if _, err := runCLI(t, "repo", "add", repoDir); err != nil {
		t.Fatalf("repo add: %v", err)
	}
	if _, err := runCLI(t, "index", repoDir); err != nil {
		t.Fatalf("index: %v", err)
	}

	t.Run("no results", func(t *testing.T) {
		out, err := runCLI(t, "search", "zzqqxx")
		if err != nil {
			t.Fatalf("search: %v", err)
		}
		if !strings.Contains(out, "No results found.") {
			t.Errorf("expected no results, got %q", out)
		}
	})

	t.Run("found", func(t *testing.T) {
		out, err := runCLI(t, "search", "AuthToken")
		if err != nil {
			t.Fatalf("search: %v", err)
		}
		if !strings.Contains(out, "2 result(s) for \"AuthToken\"") {
			t.Errorf("expected 2 results, got %q", out)
		}
		if !strings.Contains(out, "auth.go") || !strings.Contains(out, "auth2.go") {
			t.Errorf("expected both files in results, got %q", out)
		}
	})

	t.Run("limit flag", func(t *testing.T) {
		out, err := runCLI(t, "search", "--limit", "1", "AuthToken")
		if err != nil {
			t.Fatalf("search: %v", err)
		}
		if !strings.Contains(out, "1 result(s) for \"AuthToken\"") {
			t.Errorf("expected 1 result with limit, got %q", out)
		}
	})
}

func TestStatusCommand(t *testing.T) {
	isolatedEnv(t)

	t.Run("empty", func(t *testing.T) {
		out, err := runCLI(t, "status")
		if err != nil {
			t.Fatalf("status: %v", err)
		}
		if !strings.Contains(stripANSI(out), "No repositories configured.") {
			t.Errorf("expected empty message, got %q", out)
		}
	})

	t.Run("populated", func(t *testing.T) {
		repoDir := writeFixtureRepo(t, map[string]string{"main.go": "package main\n"})
		if _, err := runCLI(t, "repo", "add", repoDir); err != nil {
			t.Fatalf("repo add: %v", err)
		}
		out, err := runCLI(t, "status")
		if err != nil {
			t.Fatalf("status: %v", err)
		}
		clean := stripANSI(out)
		if !strings.Contains(clean, filepath.Base(repoDir)) || !strings.Contains(clean, "Totals:") {
			t.Errorf("status output incomplete, got %q", clean)
		}
	})
}

func TestDBVacuumCommand(t *testing.T) {
	isolatedEnv(t)

	t.Run("database not found", func(t *testing.T) {
		_, err := runCLI(t, "db", "vacuum")
		if err == nil || !strings.Contains(err.Error(), "database not found") {
			t.Fatalf("expected database not found error, got %v", err)
		}
	})

	t.Run("no free pages", func(t *testing.T) {
		repoDir := writeFixtureRepo(t, map[string]string{"main.go": "package main\n"})
		if _, err := runCLI(t, "repo", "add", repoDir); err != nil {
			t.Fatalf("repo add: %v", err)
		}
		out, err := runCLI(t, "db", "vacuum")
		if err != nil {
			t.Fatalf("db vacuum: %v", err)
		}
		if !strings.Contains(out, "no free pages") && !strings.Contains(out, "Done.") {
			t.Errorf("unexpected vacuum output: %q", out)
		}
	})
}

func TestServeSmoke(t *testing.T) {
	isolatedEnv(t)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skipf("cannot allocate port: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()

	t.Setenv("RELITH_DAEMON_TCP_HOST", "127.0.0.1")
	t.Setenv("RELITH_DAEMON_TCP_PORT", strconv.Itoa(port))
	t.Setenv("RELITH_WATCHER_ENABLED", "false")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	rootCmd.SetArgs([]string{"serve"})
	rootCmd.SetContext(ctx)
	rootCmd.SilenceErrors = true

	done := make(chan error, 1)
	go func() {
		done <- rootCmd.Execute()
	}()

	deadline := time.Now().Add(15 * time.Second)
	up := false
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 200*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			up = true
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if !up {
		cancel()
		t.Fatalf("serve did not start listening on %d", port)
	}

	cancel()

	select {
	case err := <-done:
		if err != nil && !errors.Is(err, context.Canceled) && !strings.Contains(err.Error(), "context canceled") {
			t.Fatalf("serve exited with error: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("serve did not exit after cancel")
	}
}
