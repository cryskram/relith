package logger

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cryskram/relith/internal/config"
)

func TestNewStderr(t *testing.T) {
	cfg := config.LogConfig{Level: "info", Format: "console", Output: "stderr"}
	zl := New(cfg)
	if zl.GetLevel() != -1 {
		t.Errorf("expected default level -1, got %d", zl.GetLevel())
	}
	zl.Info().Msg("test message")
}

func TestNewStdout(t *testing.T) {
	cfg := config.LogConfig{Level: "debug", Format: "json", Output: "stdout"}
	zl := New(cfg)
	zl.Debug().Msg("debug message")
}

func TestNewFileOutput(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "test.log")

	cfg := config.LogConfig{Level: "info", Format: "json", Output: logFile}
	zl := New(cfg)
	zl.Info().Msg("hello")

	data, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("read log file: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("expected non-empty log file")
	}
}

func TestNewInvalidLevelFallsBack(t *testing.T) {
	cfg := config.LogConfig{Level: "bogus", Format: "console", Output: "stderr"}
	New(cfg)
}

func TestNewInvalidFileFallback(t *testing.T) {
	cfg := config.LogConfig{Level: "info", Format: "console", Output: "/nonexistent/dir/log.log"}
	New(cfg)
}

func TestNewJSONFormat(t *testing.T) {
	var buf strings.Builder
	cfg := config.LogConfig{Level: "info", Format: "json", Output: "stdout"}
	zl := New(cfg)
	zl.Info().Str("key", "value").Msg("test")

	_ = buf
}

func TestNewConsoleFormat(t *testing.T) {
	cfg := config.LogConfig{Level: "info", Format: "console", Output: "stderr"}
	zl := New(cfg)
	zl.Info().Msg("console test")
}
