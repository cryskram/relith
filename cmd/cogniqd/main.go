package main

import (
	"context"
	"log"

	"github.com/cryskram/cogniq/internal/app"
	"github.com/cryskram/cogniq/internal/config"
	"github.com/cryskram/cogniq/internal/daemon"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}

	application := &app.App{
		Config: cfg,
	}

	d := daemon.New(application)

	if err := d.Run(context.Background()); err != nil {
		log.Fatal(err)
	}
}