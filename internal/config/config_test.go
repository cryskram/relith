package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDefaults(t *testing.T) {
	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Core.DataDir == "" {
		t.Error("expected non-empty DataDir")
	}
	if cfg.Daemon.TCPPort != 9876 {
		t.Errorf("expected TCPPort 9876, got %d", cfg.Daemon.TCPPort)
	}
	if cfg.MCP.TCPPort != 9877 {
		t.Errorf("expected MCP TCPPort 9877, got %d", cfg.MCP.TCPPort)
	}
	if cfg.Indexer.Concurrency != 4 {
		t.Errorf("expected concurrency 4, got %d", cfg.Indexer.Concurrency)
	}
	if cfg.Log.Level != "info" {
		t.Errorf("expected log level 'info', got %q", cfg.Log.Level)
	}
}

func TestLoadEnvOverride(t *testing.T) {
	os.Setenv("RELITH_CORE_DATA_DIR", "/tmp/relith-test")
	os.Setenv("RELITH_DAEMON_TCP_PORT", "9999")
	os.Setenv("RELITH_LOG_LEVEL", "debug")
	t.Cleanup(func() {
		os.Unsetenv("RELITH_CORE_DATA_DIR")
		os.Unsetenv("RELITH_DAEMON_TCP_PORT")
		os.Unsetenv("RELITH_LOG_LEVEL")
	})

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Core.DataDir != "/tmp/relith-test" {
		t.Errorf("expected DataDir '/tmp/relith-test', got %q", cfg.Core.DataDir)
	}
	if cfg.Daemon.TCPPort != 9999 {
		t.Errorf("expected TCPPort 9999, got %d", cfg.Daemon.TCPPort)
	}
	if cfg.Log.Level != "debug" {
		t.Errorf("expected log level 'debug', got %q", cfg.Log.Level)
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr bool
	}{
		{
			name: "valid defaults",
			cfg: Config{
				Core:    CoreConfig{DataDir: "/tmp/data"},
				Log:     LogConfig{Level: "info", Format: "console"},
				Daemon:  DaemonConfig{TCPPort: 9876},
				MCP:     MCPConfig{TCPPort: 9877},
				Indexer: IndexerConfig{Concurrency: 4},
				Search:  SearchConfig{MaxResults: 100},
			},
		},
		{
			name: "empty data dir",
			cfg: Config{
				Core:   CoreConfig{DataDir: ""},
				Log:    LogConfig{Level: "info", Format: "console"},
				Daemon: DaemonConfig{TCPPort: 9876},
				MCP:    MCPConfig{TCPPort: 9877},
			},
			wantErr: true,
		},
		{
			name: "invalid log level",
			cfg: Config{
				Core:   CoreConfig{DataDir: "/tmp/data"},
				Log:    LogConfig{Level: "trace", Format: "console"},
				Daemon: DaemonConfig{TCPPort: 9876},
				MCP:    MCPConfig{TCPPort: 9877},
			},
			wantErr: true,
		},
		{
			name: "invalid daemon port",
			cfg: Config{
				Core:   CoreConfig{DataDir: "/tmp/data"},
				Log:    LogConfig{Level: "info", Format: "console"},
				Daemon: DaemonConfig{TCPPort: 0},
				MCP:    MCPConfig{TCPPort: 9877},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validate(&tt.cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("validate() error = %v, wantErr = %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateClamps(t *testing.T) {
	cfg := Config{
		Core:    CoreConfig{DataDir: "/tmp/data"},
		Log:     LogConfig{Level: "info", Format: "console"},
		Daemon:  DaemonConfig{TCPPort: 9876},
		MCP:     MCPConfig{TCPPort: 9877},
		Indexer: IndexerConfig{Concurrency: 0},
		Search:  SearchConfig{MaxResults: 0},
	}
	if err := validate(&cfg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Indexer.Concurrency != 1 {
		t.Errorf("expected concurrency clamped to 1, got %d", cfg.Indexer.Concurrency)
	}
	if cfg.Search.MaxResults != 1 {
		t.Errorf("expected MaxResults clamped to 1, got %d", cfg.Search.MaxResults)
	}
}

func TestConfigFilePath(t *testing.T) {
	path, err := ConfigFilePath()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if path == "" {
		t.Fatal("expected non-empty path")
	}
	if filepath.Ext(path) != ".yaml" {
		t.Fatalf("expected .yaml extension, got %q", path)
	}
}
