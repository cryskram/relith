package daemon

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/cryskram/relith/internal/app"
	"github.com/cryskram/relith/internal/config"
	"github.com/cryskram/relith/internal/testutil"
)

func newDaemon(t *testing.T) (*Daemon, *app.App, string) {
	t.Helper()

	dir := t.TempDir()
	cfg := &config.Config{Core: config.CoreConfig{DataDir: dir}}
	a := &app.App{Config: cfg, Logger: testutil.DiscardLogger()}
	return New(a), a, dir
}

func TestInitDataDir(t *testing.T) {
	d, _, dir := newDaemon(t)

	if err := d.initDataDir(); err != nil {
		t.Fatalf("initDataDir: %v", err)
	}
	if _, err := os.Stat(dir); err != nil {
		t.Errorf("data dir should exist after init: %v", err)
	}
}

func TestInitDataDirMissingConfig(t *testing.T) {
	a := &app.App{Config: &config.Config{}, Logger: testutil.DiscardLogger()}
	d := New(a)

	if err := d.initDataDir(); err == nil {
		t.Error("expected error when data dir is empty")
	}
}

func TestOpenCloseDB(t *testing.T) {
	d, a, dir := newDaemon(t)

	if err := d.initDataDir(); err != nil {
		t.Fatalf("initDataDir: %v", err)
	}
	if err := d.openDB(); err != nil {
		t.Fatalf("openDB: %v", err)
	}
	if a.DB == nil {
		t.Fatal("expected app.DB to be set after openDB")
	}

	if _, err := os.Stat(filepath.Join(dir, "relith.db")); err != nil {
		t.Errorf("expected relith.db to be created: %v", err)
	}

	d.closeDB()
	if a.DB == nil {
		t.Fatal("closeDB should not nil out app.DB")
	}
}

func TestOpenDBSkipIfAlreadySet(t *testing.T) {
	d, a, _ := newDaemon(t)

	fdb := testutil.NewDB(t)
	a.DB = fdb.SQL

	if err := d.openDB(); err != nil {
		t.Fatalf("openDB: %v", err)
	}
	if a.DB != fdb.SQL {
		t.Error("openDB should keep the existing connection")
	}

	d.closeDB()
}
