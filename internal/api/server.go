package api

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/cryskram/relith/internal/config"
	"github.com/cryskram/relith/internal/db"
	"github.com/cryskram/relith/internal/indexer"
	"github.com/cryskram/relith/internal/reasoning"
	"github.com/cryskram/relith/internal/search"
)

type Server struct {
	http   *http.Server
	listen net.Listener
	logger *slog.Logger
	cfg    config.DaemonConfig
}

func New(database *sql.DB, logger *slog.Logger, cfg *config.Config) *Server {
	searcher := search.New(database, logger, cfg.Search)
	h := &handlers{
		queries:  db.New(database),
		indexer:  indexer.New(database, logger, cfg.Indexer),
		searcher: searcher,
		reasoner: reasoning.New(database, logger, searcher),
	}

	mux := http.NewServeMux()
	mux.Handle("GET /", dashboardHandler())
	mux.HandleFunc("GET /v1/health", h.health)
	mux.HandleFunc("GET /v1/stats", h.stats)
	mux.HandleFunc("GET /v1/reason", h.reason)
	mux.HandleFunc("GET /v1/graph", h.graph)
	mux.HandleFunc("GET /v1/repos", h.listRepos)
	mux.HandleFunc("POST /v1/repos", h.createRepo)
	mux.HandleFunc("GET /v1/repos/{id}", h.getRepo)
	mux.HandleFunc("DELETE /v1/repos/{id}", h.deleteRepo)
	mux.HandleFunc("POST /v1/repos/{id}/index", h.indexRepo)
	mux.HandleFunc("GET /v1/search", h.search)
	mux.HandleFunc("GET /v1/content", h.content)

	httpSrv := &http.Server{
		Handler:     mux,
		ReadTimeout: 10 * time.Second,
		IdleTimeout: 60 * time.Second,
	}

	discard := slog.New(slog.DiscardHandler)
	return &Server{
		http:   httpSrv,
		logger: discard,
		cfg:    cfg.Daemon,
	}
}

func (s *Server) Start() error {
	socketPath := s.cfg.Socket

	if val, ok := os.LookupEnv("RELITH_DAEMON_SOCKET"); ok && val == "" {
		socketPath = ""
	}

	if socketPath != "" {
		os.Remove(socketPath)
		listener, err := net.Listen("unix", socketPath)
		if err != nil {
			return fmt.Errorf("unix listen: %w", err)
		}
		if err := os.Chmod(socketPath, 0660); err != nil {
			listener.Close()
			return fmt.Errorf("socket chmod: %w", err)
		}
		s.listen = listener
	} else {
		addr := fmt.Sprintf("%s:%d", s.cfg.TCPHost, s.cfg.TCPPort)
		listener, err := net.Listen("tcp", addr)
		if err != nil {
			return fmt.Errorf("tcp listen: %w", err)
		}
		s.listen = listener
	}

	go func() {
		if err := s.http.Serve(s.listen); err != nil && err != http.ErrServerClosed {
			s.logger.Error("server error", "err", err)
		}
	}()

	return nil
}

func (s *Server) Stop(ctx context.Context) error {
	shutdownCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := s.http.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown: %w", err)
	}

	if s.cfg.Socket != "" {
		os.Remove(s.cfg.Socket)
	}

	return nil
}
