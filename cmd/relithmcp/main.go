package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/cryskram/relith/internal/config"
	"github.com/cryskram/relith/internal/db"
	"github.com/cryskram/relith/internal/logger"
	"github.com/cryskram/relith/internal/mcp"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "config: %v\n", err)
		os.Exit(1)
	}

	log := logger.New(cfg.Log)

	if err := os.MkdirAll(cfg.Core.DataDir, 0755); err != nil {
		log.Fatal().Err(err).Str("dir", cfg.Core.DataDir).Msg("create data directory")
	}

	dbPath := filepath.Join(cfg.Core.DataDir, "relith.db")
	database, err := db.Open(dbPath)
	if err != nil {
		log.Fatal().Err(err).Str("path", dbPath).Msg("open database")
	}
	defer database.Close()

	server := mcp.NewServer(database, log)

	ctx := context.Background()
	if err := server.Run(ctx); err != nil {
		log.Fatal().Err(err).Msg("mcp server error")
	}
}
