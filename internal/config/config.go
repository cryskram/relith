package config

import (
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	Core    CoreConfig
	Daemon  DaemonConfig
	MCP     MCPConfig
	Indexer IndexerConfig
	Watcher WatcherConfig
	Search  SearchConfig
	Log     LogConfig
}

type CoreConfig struct {
	DataDir string
}

type DaemonConfig struct {
	Socket  string
	TCPHost string
	TCPPort int
}

type MCPConfig struct {
	Enabled   bool
	Transport string
	TCPPort   int
}

type IndexerConfig struct {
	Concurrency int
	MaxFileSize int64
	MaxCommits  int
}

type WatcherConfig struct {
	Enabled  bool
	Debounce time.Duration
}

type SearchConfig struct {
	MaxResults   int
	PathBoosting bool
}

type LogConfig struct {
	Level  string
	Format string
	Output string
}

func Load() (*Config, error) {
	v := viper.New()

	v.SetDefault("core.data_dir", "~/.local/share/cogniq")
	v.SetDefault("daemon.socket", "~/.local/share/cogniq/cogniq.sock")
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

	v.SetEnvPrefix("COGNIQ")
	v.AutomaticEnv()

	var cfg Config

	if err := v.Unmarshal(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}