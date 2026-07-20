package cli

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	"github.com/cryskram/relith/internal/config"
	"github.com/cryskram/relith/internal/db"
	"github.com/cryskram/relith/internal/logger"
	"github.com/rs/zerolog"
)

type cliApp struct {
	cfg    *config.Config
	logger zerolog.Logger
	db     *sql.DB
}

func openDB() (*cliApp, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("config: %w", err)
	}

	zl := logger.New(cfg.Log)

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

	return &cliApp{cfg: cfg, logger: zl, db: database}, nil
}

func (a *cliApp) close() {
	if a.db != nil {
		a.db.Close()
	}
}
