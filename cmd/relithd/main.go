package main

import (
	"context"
	"log"

	"github.com/cryskram/relith/internal/app"
	"github.com/cryskram/relith/internal/config"
	"github.com/cryskram/relith/internal/daemon"
	"github.com/cryskram/relith/internal/logger"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}

	zl := logger.New(cfg.Log)
	application := &app.App{Config: cfg, Logger: zl}

	d := daemon.New(application)
	if err := d.Run(context.Background()); err != nil {
		zl.Fatal().Err(err).Msg("daemon exited with error")
	}
}
