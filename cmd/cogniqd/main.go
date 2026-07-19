package main

import (
	"context"
	"log"

	"github.com/cryskram/cogniq/internal/daemon"
)

func main() {
	d := daemon.New()

	if err := d.Run(context.Background()); err != nil {
		log.Fatal(err)
	}
}