package database

import (
	"log/slog"
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

func NewConnection(ctx context.Context, destination string, logger *slog.Logger) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, destination)
	if err != nil {
		return nil, err
	}

	// check that connection is established, close the pool if not
	if err := pool.Ping(ctx); error != nil { 
		pool.Close()
		return nil, err
	}

	logger.Info("Database connection pool established successfully")

	return pool, nil
}