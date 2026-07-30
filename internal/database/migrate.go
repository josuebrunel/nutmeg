package database

import (
	"context"
	"database/sql"
	"embed"
	"fmt"

	"github.com/pressly/goose/v3"
)

// Migrate runs database migrations using the given embedded filesystem.
func Migrate(db *sql.DB, fsys embed.FS) error {
	provider, err := goose.NewProvider(
		goose.DialectPostgres, db, fsys,
		goose.WithDisableGlobalRegistry(true),
	)
	if err != nil {
		return fmt.Errorf("migration provider: %w", err)
	}

	if _, err := provider.Up(context.Background()); err != nil {
		return fmt.Errorf("migration up: %w", err)
	}

	return nil
}

// MigrateDown reverts the most recently applied migration.
func MigrateDown(db *sql.DB, fsys embed.FS) error {
	provider, err := goose.NewProvider(
		goose.DialectPostgres, db, fsys,
		goose.WithDisableGlobalRegistry(true),
	)
	if err != nil {
		return fmt.Errorf("migration provider: %w", err)
	}

	if _, err := provider.Down(context.Background()); err != nil {
		return fmt.Errorf("migration down: %w", err)
	}

	return nil
}
