package main

import (
	"context"
	"log"
	"log/slog"
	"os"

	"github.com/cryskram/relith/internal/app"
	"github.com/cryskram/relith/internal/config"
	"github.com/cryskram/relith/internal/daemon"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}

	slogLogger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	application := &app.App{Config: cfg, Logger: slogLogger}

	d := daemon.New(application)
	if err := d.Run(context.Background()); err != nil {
		slogLogger.Error("daemon exited with error", "err", err)
		os.Exit(1)
	}
}
