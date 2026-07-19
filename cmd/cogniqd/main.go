package main

import (
	"context"
	"log"

	"github.com/cryskram/cogniq/internal/config"
	"github.com/cryskram/cogniq/internal/daemon"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}

	d := daemon.New(cfg)

	if err := d.Run(context.Background()); err != nil {
		log.Fatal(err)
	}
}