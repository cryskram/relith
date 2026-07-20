package db

import (
	"database/sql"
	"fmt"
	"strings"

	_ "modernc.org/sqlite"
)

func Open(path string) (*sql.DB, error) {
	database, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("sql open: %w", err)
	}

	if path != ":memory:" {
		var journalMode string
		if err := database.QueryRow("PRAGMA journal_mode=WAL").Scan(&journalMode); err != nil {
			database.Close()
			return nil, fmt.Errorf("enable WAL: %w", err)
		}
		if !strings.EqualFold(journalMode, "wal") {
			database.Close()
			return nil, fmt.Errorf("WAL mode not enabled (got %q)", journalMode)
		}
	}

	if _, err := database.Exec("PRAGMA busy_timeout=5000"); err != nil {
		database.Close()
		return nil, fmt.Errorf("set busy timeout: %w", err)
	}
	if _, err := database.Exec("PRAGMA foreign_keys=ON"); err != nil {
		database.Close()
		return nil, fmt.Errorf("enable foreign keys: %w", err)
	}

	database.SetMaxOpenConns(1)
	database.SetMaxIdleConns(1)
	database.SetConnMaxLifetime(0)

	if err := database.Ping(); err != nil {
		database.Close()
		return nil, fmt.Errorf("db ping: %w", err)
	}

	return database, nil
}
