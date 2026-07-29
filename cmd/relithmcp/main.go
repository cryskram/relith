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
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "config: %v\n", err)
		os.Exit(1)
	}

	slogLogger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	if err := os.MkdirAll(cfg.Core.DataDir, 0755); err != nil {
		slogLogger.Error("create data directory", "err", err, "dir", cfg.Core.DataDir)
		os.Exit(1)
	}

	dbPath := filepath.Join(cfg.Core.DataDir, "relith.db")
	database, err := db.Open(dbPath)
	if err != nil {
		slogLogger.Error("open database", "err", err, "path", dbPath)
		os.Exit(1)
	}
	defer database.Close()

	server := mcp.NewServer(database, slogLogger)

	ctx := context.Background()
	if err := server.Run(ctx); err != nil {
		slogLogger.Error("mcp server error", "err", err)
		os.Exit(1)
	}
}
