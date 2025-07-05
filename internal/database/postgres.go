package database

import (
	"log/slog"
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

func NewConnection(ctx context.Context, destination string, logger *slog.Logger) (*pgxpool.Pool, error) {
	pool, error := pgxpool.New(ctx, destination)
	if error != nil {
		return nil, error
	}

	// check that connection is established, close the pool if not
	if error := pool.Ping(ctx); error != nil { 
		pool.Close()
		return nil, error
	}

	logger.Info("Database connection pool established successfully")

	return pool, nil
}