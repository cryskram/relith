package db

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

func Open(path string) (*sql.DB, error) {
	database, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("sql open: %w", err)
	}

	if _, err := database.Exec("PRAGMA journal_mode=WAL"); err != nil {
		database.Close()
		return nil, fmt.Errorf("enable WAL: %w", err)
	}
	if _, err := database.Exec("PRAGMA busy_timeout=5000"); err != nil {
		database.Close()
		return nil, fmt.Errorf("set busy timeout: %w", err)
	}
	if _, err := database.Exec("PRAGMA foreign_keys=ON"); err != nil {
		database.Close()
		return nil, fmt.Errorf("enable foreign keys: %w", err)
	}

	if err := database.Ping(); err != nil {
		database.Close()
		return nil, fmt.Errorf("db ping: %w", err)
	}

	return database, nil
}
