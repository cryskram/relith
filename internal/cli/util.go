package cli

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/cryskram/relith/internal/config"
	"github.com/cryskram/relith/internal/db"
)

type cliApp struct {
	cfg    *config.Config
	logger *slog.Logger
	db     *sql.DB
}

func openDB() (*cliApp, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("config: %w", err)
	}

	if err := os.MkdirAll(cfg.Core.DataDir, 0755); err != nil {
		return nil, fmt.Errorf("create data dir: %w", err)
	}

	dbPath := filepath.Join(cfg.Core.DataDir, "relith.db")
	database, err := db.Open(dbPath)
	if err != nil {
		return nil, fmt.Errorf("db: %w", err)
	}

	if err := db.Migrate(context.Background(), database); err != nil {
		database.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}

	discard := slog.New(slog.DiscardHandler)
	return &cliApp{cfg: cfg, logger: discard, db: database}, nil
}

func (a *cliApp) close() {
	if a.db != nil {
		a.db.Close()
	}
}
