package daemon

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"syscall"
)

type Daemon struct{}

func New() *Daemon {
	return &Daemon{}
}

func (d *Daemon) Run(ctx context.Context) error {
	ctx, cancel := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer cancel()

	<-ctx.Done()

	if errors.Is(ctx.Err(), context.Canceled) {
		return nil
	}

	return ctx.Err()
}