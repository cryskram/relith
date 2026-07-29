package cli

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/cryskram/relith/internal/app"
	"github.com/cryskram/relith/internal/config"
	"github.com/cryskram/relith/internal/daemon"
	"github.com/cryskram/relith/internal/tui"
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the daemon (REST API + dashboard + file watcher)",
	Long: `Starts the Relith daemon which serves the REST API, dashboard web UI,
and file watcher for automatic re-indexing.

Listens on TCP (127.0.0.1:9876 by default). Set daemon.socket
in config or RELITH_DAEMON_SOCKET to use a Unix socket instead.

The dashboard is available at http://localhost:9876/ in a browser.

Examples:
  relith serve
  RELITH_DAEMON_TCP_PORT=9877 relith serve
  RELITH_DAEMON_SOCKET=/tmp/relith.sock relith serve`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("config: %w", err)
		}

		slogLogger := slog.New(slog.DiscardHandler)
		application := &app.App{Config: cfg, Logger: slogLogger}

		d := daemon.New(application)

		addr := fmt.Sprintf("%s:%d", cfg.Daemon.TCPHost, cfg.Daemon.TCPPort)

		if term.IsTerminal(int(os.Stdout.Fd())) {
			return serveTUI(application, d, addr)
		}

		fmt.Printf("Starting server at http://%s\n", addr)
		if err := d.Run(cmd.Context()); err != nil {
			return fmt.Errorf("daemon exited with error: %w", err)
		}
		return nil
	},
}

func serveTUI(application *app.App, d *daemon.Daemon, addr string) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	m := tui.NewServerModel(addr)

	errCh := make(chan error, 1)
	go func() {
		if err := d.Run(ctx); err != nil {
			errCh <- err
		}
	}()

	p := tea.NewProgram(m)
	if _, err := p.Run(); err != nil {
		return err
	}

	cancel()

	select {
	case err := <-errCh:
		if err != nil && !errors.Is(err, context.Canceled) {
			return err
		}
	default:
	}

	return nil
}

func init() {
	rootCmd.AddCommand(serveCmd)
}
