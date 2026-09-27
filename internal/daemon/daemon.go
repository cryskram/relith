package daemon

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/cryskram/relith/internal/api"
	"github.com/cryskram/relith/internal/app"
	"github.com/cryskram/relith/internal/db"
	"github.com/cryskram/relith/internal/indexer"
	"github.com/cryskram/relith/internal/watcher"
)

type Daemon struct {
	app      *app.App
	apiSrv   *api.Server
	watchers []*watcher.Watcher
}

func New(a *app.App) *Daemon {
	return &Daemon{
		app: a,
	}
}

func (d *Daemon) Run(ctx context.Context) error {
	if err := d.initDataDir(); err != nil {
		return err
	}

	if err := d.openDB(); err != nil {
		return err
	}
	defer d.closeDB()

	if ctx == nil {
		ctx = context.Background()
	}

	if err := db.Migrate(ctx, d.app.DB); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}

	d.apiSrv = api.New(d.app.DB, d.app.Logger, d.app.Config)
	if err := d.apiSrv.Start(); err != nil {
		return fmt.Errorf("api server: %w", err)
	}
	defer d.stopAPI(ctx)

	ctx, cancel := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer cancel()

	if err := d.startWatchers(ctx); err != nil {
		slog.Warn("watcher startup failed", "err", err)
	}

	<-ctx.Done()

	d.stopWatchers()

	if errors.Is(ctx.Err(), context.Canceled) {
		return nil
	}
	return ctx.Err()
}

func (d *Daemon) startWatchers(ctx context.Context) error {
	cfg := d.app.Config
	if cfg == nil || !cfg.Watcher.Enabled {
		return nil
	}

	idx := indexer.New(d.app.DB, d.app.Logger, cfg.Indexer)

	repos, err := db.New(d.app.DB).ListRepos(ctx)
	if err != nil {
		return fmt.Errorf("list repos: %w", err)
	}

	slog.Info("starting file watcher", "repos", len(repos))
	for _, repo := range repos {
		w := watcher.New(repo.Path, repo.ID, idx, d.app.Logger, cfg.Watcher)
		if err := w.Start(ctx); err != nil {
			slog.Error("start watcher", "err", err, "repo", repo.Path)
			continue
		}
		d.watchers = append(d.watchers, w)
	}
	return nil
}

func (d *Daemon) stopWatchers() {
	for _, w := range d.watchers {
		w.Stop()
	}
	d.watchers = nil
}

func (d *Daemon) stopAPI(ctx context.Context) {
	if d.apiSrv != nil {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := d.apiSrv.Stop(shutdownCtx); err != nil {
			if !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
				slog.Error("stop api server", "err", err)
			}
		}
	}
}

func (d *Daemon) initDataDir() error {
	dir := d.app.Config.Core.DataDir
	if dir == "" {
		return fmt.Errorf("data dir not configured")
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create data dir %s: %w", dir, err)
	}
	return nil
}

func (d *Daemon) openDB() error {
	if d.app.DB != nil {
		return nil
	}
	path := filepath.Join(d.app.Config.Core.DataDir, "relith.db")
	database, err := db.Open(path)
	if err != nil {
		return fmt.Errorf("open db: %w", err)
	}
	d.app.DB = database
	return nil
}

func (d *Daemon) closeDB() {
	if d.app.DB != nil {
		if err := d.app.DB.Close(); err != nil {
			slog.Error("close database", "err", err)
		}
	}
}
