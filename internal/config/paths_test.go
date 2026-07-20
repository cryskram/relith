package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfigDir(t *testing.T) {
	dir, err := DefaultConfigDir()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dir == "" {
		t.Fatal("expected non-empty config dir")
	}
	if !filepath.IsAbs(dir) {
		t.Fatalf("expected absolute path, got %q", dir)
	}
	if filepath.Base(dir) != "relith" {
		t.Fatalf("expected dir to end with 'relith', got %q", dir)
	}
}

func TestDefaultDataDir(t *testing.T) {
	dir, err := DefaultDataDir()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dir == "" {
		t.Fatal("expected non-empty data dir")
	}
	if !filepath.IsAbs(dir) {
		t.Fatalf("expected absolute path, got %q", dir)
	}
}

func TestDefaultSocketPath(t *testing.T) {
	path, err := DefaultSocketPath()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if path == "" {
		t.Fatal("expected non-empty socket path")
	}
	if !filepath.IsAbs(path) {
		t.Fatalf("expected absolute path, got %q", path)
	}
	if filepath.Ext(path) != ".sock" {
		t.Fatalf("expected .sock extension, got %q", path)
	}
}

func TestDefaultDataDir_XDGOverride(t *testing.T) {
	orig := os.Getenv("XDG_DATA_HOME")
	t.Cleanup(func() { os.Setenv("XDG_DATA_HOME", orig) })

	os.Setenv("XDG_DATA_HOME", "/custom/xdg/data")
	dir, err := DefaultDataDir()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := filepath.Join("/custom/xdg/data", "relith")
	if dir != expected {
		t.Fatalf("expected %q, got %q", expected, dir)
	}
}
