package database

import (
	"context"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres" // Driver
	_ "github.com/golang-migrate/migrate/v4/source/file"       // Source
	"log/slog"
)

func RunMigrations(ctx context.Context, dbURL string, logger *slog.Logger) error {
	m, err := migrate.New(
		"file:///app/services/api-gateway/migrations",
		dbURL,
	)

	if err != nil {
		return err
	}

	defer m.Close()

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return err
	}

	logger.Info("Migrations applied successfully")
	return nil
}