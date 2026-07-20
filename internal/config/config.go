package config

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/viper"
)

var (
	ErrNoDataDir       = errors.New("data directory not configured")
	ErrInvalidLogLevel = errors.New("invalid log level (valid: debug, info, warn, error, fatal)")
	ErrInvalidLogFmt   = errors.New("invalid log format (valid: console, json)")
	ErrInvalidPort     = errors.New("port must be between 1 and 65535")
)

type Config struct {
	Core    CoreConfig    `mapstructure:"core"`
	Daemon  DaemonConfig  `mapstructure:"daemon"`
	MCP     MCPConfig     `mapstructure:"mcp"`
	Indexer IndexerConfig `mapstructure:"indexer"`
	Watcher WatcherConfig `mapstructure:"watcher"`
	Search  SearchConfig  `mapstructure:"search"`
	Log     LogConfig     `mapstructure:"log"`
}

type CoreConfig struct {
	DataDir string `mapstructure:"data_dir"`
}

type DaemonConfig struct {
	Socket  string `mapstructure:"socket"`
	TCPHost string `mapstructure:"tcp_host"`
	TCPPort int    `mapstructure:"tcp_port"`
}

type MCPConfig struct {
	Enabled   bool   `mapstructure:"enabled"`
	Transport string `mapstructure:"transport"`
	TCPPort   int    `mapstructure:"tcp_port"`
}

type IndexerConfig struct {
	Concurrency int   `mapstructure:"concurrency"`
	MaxFileSize int64 `mapstructure:"max_file_size"`
	MaxCommits  int   `mapstructure:"max_commits"`
}

type WatcherConfig struct {
	Enabled  bool          `mapstructure:"enabled"`
	Debounce time.Duration `mapstructure:"debounce"`
}

type SearchConfig struct {
	MaxResults   int  `mapstructure:"max_results"`
	PathBoosting bool `mapstructure:"path_boosting"`
}

type LogConfig struct {
	Level  string `mapstructure:"level"`
	Format string `mapstructure:"format"`
	Output string `mapstructure:"output"`
}

func setDefaults(v *viper.Viper) error {
	dataDir, err := DefaultDataDir()
	if err != nil {
		return fmt.Errorf("default data dir: %w", err)
	}
	v.SetDefault("core.data_dir", dataDir)

	socketPath, err := DefaultSocketPath()
	if err != nil {
		return fmt.Errorf("default socket path: %w", err)
	}
	v.SetDefault("daemon.socket", socketPath)
	v.SetDefault("daemon.tcp_host", "127.0.0.1")
	v.SetDefault("daemon.tcp_port", 9876)

	v.SetDefault("mcp.enabled", true)
	v.SetDefault("mcp.transport", "stdio")
	v.SetDefault("mcp.tcp_port", 9877)

	v.SetDefault("indexer.concurrency", 4)
	v.SetDefault("indexer.max_file_size", 10*1024*1024)
	v.SetDefault("indexer.max_commits", 10000)

	v.SetDefault("watcher.enabled", true)
	v.SetDefault("watcher.debounce", time.Second)

	v.SetDefault("search.max_results", 100)
	v.SetDefault("search.path_boosting", true)

	v.SetDefault("log.level", "info")
	v.SetDefault("log.format", "console")
	v.SetDefault("log.output", "stderr")

	return nil
}

func validate(cfg *Config) error {
	if cfg.Core.DataDir == "" {
		return ErrNoDataDir
	}
	switch cfg.Log.Level {
	case "debug", "info", "warn", "error", "fatal":
	default:
		return ErrInvalidLogLevel
	}
	switch cfg.Log.Format {
	case "console", "json":
	default:
		return ErrInvalidLogFmt
	}
	if cfg.Daemon.TCPPort < 1 || cfg.Daemon.TCPPort > 65535 {
		return fmt.Errorf("daemon tcp_port: %w", ErrInvalidPort)
	}
	if cfg.MCP.TCPPort < 1 || cfg.MCP.TCPPort > 65535 {
		return fmt.Errorf("mcp tcp_port: %w", ErrInvalidPort)
	}
	if cfg.Indexer.Concurrency < 1 {
		cfg.Indexer.Concurrency = 1
	}
	if cfg.Search.MaxResults < 1 {
		cfg.Search.MaxResults = 1
	}
	return nil
}

func Load() (*Config, error) {
	v := viper.New()

	if err := setDefaults(v); err != nil {
		return nil, err
	}

	configDir, err := DefaultConfigDir()
	if err == nil {
		v.AddConfigPath(configDir)
	}
	v.AddConfigPath(".")
	v.SetConfigName("relith")
	v.SetConfigType("yaml")

	_ = v.ReadInConfig()

	v.SetEnvPrefix("RELITH")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, err
	}

	if err := validate(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func ConfigFilePath() (string, error) {
	configDir, err := DefaultConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(configDir, "relith.yaml"), nil
}