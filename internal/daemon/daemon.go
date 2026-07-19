package daemon

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"syscall"

	"github.com/cryskram/cogniq/internal/app"
)

type Daemon struct {
	app *app.App
}

func New(app *app.App) *Daemon {
	return &Daemon{
		app: app,
	}
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