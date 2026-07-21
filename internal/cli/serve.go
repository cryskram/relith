package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/cryskram/relith/internal/app"
	"github.com/cryskram/relith/internal/config"
	"github.com/cryskram/relith/internal/daemon"
	"github.com/cryskram/relith/internal/logger"
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

		zl := logger.New(cfg.Log)
		application := &app.App{Config: cfg, Logger: zl}

		d := daemon.New(application)

		addr := fmt.Sprintf("%s:%d", cfg.Daemon.TCPHost, cfg.Daemon.TCPPort)
		zl.Info().Str("addr", addr).Msg("dashboard at http://" + addr)

		if err := d.Run(cmd.Context()); err != nil {
			zl.Fatal().Err(err).Msg("daemon exited with error")
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(serveCmd)
}
