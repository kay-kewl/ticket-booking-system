package storage

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Storage struct {
	db *pgxpool.Pool
}

func New(db *pgxpool.Pool) *Storage {
	return &Storage{db: db}
}

func (s *Storage) SaveUser(ctx context.Context, email string, passHash []byte) (int64, error) {
	const op = "storage.SaveUser"

	query := "INSERT INTO users(email, password_hash) VALUES($1, $2) RETURNING id"

	var id int64
	err := s.db.QueryRow(ctx, query, email, passHash).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", op, err)
	}

	return id, nil
}

func (s *Storage) User(ctx context.Context, email string) (int64, []byte, error) {
	const op = "storage.User"

	query := "SELECT id, password_hash FROM users WHERE email = $1"

	var id int64
	var passHash []byte

	err := s.db.QueryRow(ctx, query, email).Scan(&id, &passHash)
	if err != nil {
		// TODO: pgx.ErrNoRows
		return 0, nil, fmt.Errorf("%s: %w", op, err)
	}

	return id, passHash, nil
}