package main

import (
	"log"

	"github.com/cryskram/relith/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		log.Fatal(err)
	}
}