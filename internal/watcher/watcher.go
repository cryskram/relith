package watcher

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
	"log/slog"

	"github.com/cryskram/relith/internal/config"
	"github.com/cryskram/relith/internal/indexer"
)

type Watcher struct {
	fsWatcher *fsnotify.Watcher
	debouncer *Debouncer
	indexer   *indexer.Indexer
	repoPath  string
	repoID    int64
	logger    *slog.Logger
	cfg       config.WatcherConfig

	cancel context.CancelFunc
	done   chan struct{}
}

func New(repoPath string, repoID int64, idx *indexer.Indexer, logger *slog.Logger, cfg config.WatcherConfig) *Watcher {
	return &Watcher{
		repoPath: repoPath,
		repoID:   repoID,
		indexer:  idx,
		logger:   logger.With("component", "watcher", "repo", repoPath),
		cfg:      cfg,
		done:     make(chan struct{}),
	}
}

func (w *Watcher) Start(ctx context.Context) error {
	fw, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("fsnotify: %w", err)
	}
	w.fsWatcher = fw

	if err := w.addTree(w.repoPath); err != nil {
		fw.Close()
		return fmt.Errorf("add tree: %w", err)
	}

	debounceInterval := w.cfg.Debounce
	if debounceInterval <= 0 {
		debounceInterval = time.Second
	}

	w.debouncer = NewDebouncer(debounceInterval, w.handleChanges)

	ctx, cancel := context.WithCancel(ctx)
	w.cancel = cancel

	go w.loop(ctx)

	w.logger.Info("watcher started", "debounce", debounceInterval)
	return nil
}

func (w *Watcher) Stop() {
	if w.cancel != nil {
		w.cancel()
	}
	if w.debouncer != nil {
		w.debouncer.Stop()
	}
	if w.fsWatcher != nil {
		w.fsWatcher.Close()
	}
	<-w.done
	w.logger.Info("watcher stopped")
}

func (w *Watcher) loop(ctx context.Context) {
	defer close(w.done)

	for {
		select {
		case event, ok := <-w.fsWatcher.Events:
			if !ok {
				return
			}
			w.handleEvent(event)

		case err, ok := <-w.fsWatcher.Errors:
			if !ok {
				return
			}
			w.logger.Error("watcher error", "err", err)

		case <-ctx.Done():
			return
		}
	}
}

func (w *Watcher) handleEvent(event fsnotify.Event) {
	if event.Has(fsnotify.Chmod) {
		return
	}

	rel, err := filepath.Rel(w.repoPath, event.Name)
	if err != nil {
		return
	}
	if w.shouldSkip(rel) {
		return
	}

	if event.Has(fsnotify.Create) {
		info, err := os.Stat(event.Name)
		if err == nil && info.IsDir() {
			w.addTree(event.Name)
		}
	}

	w.debouncer.Add(rel)
}

func (w *Watcher) handleChanges(paths []string) {
	ctx := context.Background()

	for _, relPath := range paths {
		fullPath := filepath.Join(w.repoPath, relPath)

		info, err := os.Stat(fullPath)
		if os.IsNotExist(err) {
			w.logger.Debug("file removed", "path", relPath)
			if err := w.indexer.DeleteFile(ctx, w.repoID, relPath); err != nil {
				w.logger.Error("delete failed", "err", err, "path", relPath)
			}
			continue
		}
		if err != nil {
			w.logger.Error("stat failed", "err", err, "path", relPath)
			continue
		}
		if info.IsDir() {
			continue
		}

		w.logger.Debug("file changed", "path", relPath, "size", info.Size())
		if err := w.indexer.IndexFile(ctx, w.repoID, relPath, fullPath); err != nil {
			w.logger.Error("index failed", "err", err, "path", relPath)
		}
	}
}

func (w *Watcher) addTree(root string) error {
	return filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			rel, err := filepath.Rel(w.repoPath, path)
			if err != nil {
				return nil
			}
			if !strings.HasPrefix(rel, ".git") && rel != "." && w.shouldSkip(rel) {
				return filepath.SkipDir
			}
			if err := w.fsWatcher.Add(path); err != nil {
				w.logger.Warn("watch add dir failed", "err", err, "path", path)
			}
		}
		return nil
	})
}

func (w *Watcher) shouldSkip(relPath string) bool {
	parts := strings.Split(relPath, string(filepath.Separator))
	for _, part := range parts {
		if part == ".git" || strings.HasPrefix(part, ".") {
			return true
		}
		if part == "node_modules" || part == "vendor" || part == "__pycache__" {
			return true
		}
	}
	if w.indexer == nil {
		return false
	}
	info, err := os.Stat(filepath.Join(w.repoPath, relPath))
	if err != nil {
		return false
	}
	if info.IsDir() {
		return false
	}
	if info.Size() == 0 {
		return true
	}
	return false
}
