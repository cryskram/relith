package logger

import (
	"io"
	"os"
	"time"

	"github.com/rs/zerolog"

	"github.com/cryskram/relith/internal/config"
)

func New(cfg config.LogConfig) zerolog.Logger {
	var output io.Writer = os.Stderr

	if cfg.Output == "stdout" {
		output = os.Stdout
	} else if cfg.Output != "" && cfg.Output != "stderr" {
		f, err := os.OpenFile(cfg.Output, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
		if err == nil {
			output = f
		}
	}

	zerolog.TimeFieldFormat = time.RFC3339Nano

	level, err := zerolog.ParseLevel(cfg.Level)
	if err != nil {
		level = zerolog.InfoLevel
	}

	zerolog.SetGlobalLevel(level)

	if cfg.Format == "json" {
		return zerolog.New(output).With().Timestamp().Logger()
	}

	return zerolog.New(zerolog.ConsoleWriter{Out: output, TimeFormat: "15:04:05"}).With().Timestamp().Logger()
}