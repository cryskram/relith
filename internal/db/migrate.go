package db

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/pressly/goose/v3"

	"github.com/cryskram/relith/sql/migrations"
)

func Migrate(ctx context.Context, db *sql.DB) error {
	provider, err := goose.NewProvider(
		goose.DialectSQLite3,
		db,
		migrations.Content,
		goose.WithDisableGlobalRegistry(true),
	)
	if err != nil {
		return fmt.Errorf("goose provider: %w", err)
	}

	if _, err := provider.Up(ctx); err != nil {
		return fmt.Errorf("migration up: %w", err)
	}

	return nil
}