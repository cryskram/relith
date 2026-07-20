package db

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestOpenInMemory(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		t.Fatalf("ping failed: %v", err)
	}
}

func TestOpenWALMode(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.db")
	db, err := Open(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer db.Close()

	var journalMode string
	if err := db.QueryRow("PRAGMA journal_mode").Scan(&journalMode); err != nil {
		t.Fatalf("read journal_mode: %v", err)
	}
	if !strings.EqualFold(journalMode, "wal") {
		t.Errorf("expected journal_mode=wal, got %q", journalMode)
	}
}

func TestOpenForeignKeys(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer db.Close()

	var fk string
	if err := db.QueryRow("PRAGMA foreign_keys").Scan(&fk); err != nil {
		t.Fatalf("read foreign_keys: %v", err)
	}
	if fk != "1" {
		t.Errorf("expected foreign_keys=1, got %q", fk)
	}
}

func TestOpenFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.db")

	db, err := Open(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	db.Close()
}

func TestOpenInvalidPath(t *testing.T) {
	_, err := Open("/nonexistent/dir/relith.db")
	if err == nil {
		t.Fatal("expected error for invalid path")
	}
}

func TestOpenCreatesFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "new.db")

	db, err := Open(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	db.Close()

	files, err := filepath.Glob(filepath.Join(dir, "new.db*"))
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	if len(files) == 0 {
		t.Fatal("expected db files to be created")
	}
}
