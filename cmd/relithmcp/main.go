package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/cryskram/relith/internal/config"
	"github.com/cryskram/relith/internal/db"
	"github.com/cryskram/relith/internal/mcp"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "relithmcp: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}

	slogLogger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	if err := os.MkdirAll(cfg.Core.DataDir, 0755); err != nil {
		return fmt.Errorf("create data directory %s: %w", cfg.Core.DataDir, err)
	}

	dbPath := filepath.Join(cfg.Core.DataDir, "relith.db")
	database, err := db.Open(dbPath)
	if err != nil {
		return fmt.Errorf("open database %s: %w", dbPath, err)
	}
	defer database.Close()

	server := mcp.NewServer(database, slogLogger)

	ctx := context.Background()
	if err := server.Run(ctx); err != nil {
		return fmt.Errorf("mcp server: %w", err)
	}
	return nil
}
