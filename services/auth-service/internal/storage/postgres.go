package storage

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrUserExists = errors.New("user with this email already exists")
var ErrUserNotFound = errors.New("user with this email is not found")

type Storage struct {
	db *pgxpool.Pool
}

func New(db *pgxpool.Pool) *Storage {
	return &Storage{db: db}
}

func (s *Storage) SaveUser(ctx context.Context, email string, passHash []byte) (int64, error) {
	const op = "storage.SaveUser"

	query := "INSERT INTO auth.users(email, password_hash) VALUES($1, $2) RETURNING id"

	var id int64
	err := s.db.QueryRow(ctx, query, email, passHash).Scan(&id)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return 0, fmt.Errorf("%s: %w", op, ErrUserExists)
		}
		return 0, fmt.Errorf("%s: %w", op, err)
	}

	return id, nil
}

func (s *Storage) User(ctx context.Context, email string) (int64, []byte, error) {
	const op = "storage.User"

	query := "SELECT id, password_hash FROM auth.users WHERE email = $1"

	var id int64
	var passHash []byte

	err := s.db.QueryRow(ctx, query, email).Scan(&id, &passHash)
	if err != nil {
		// TODO: pgx.ErrNoRows
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, nil, fmt.Errorf("%s: %w", op, ErrUserNotFound)
		}
		return 0, nil, fmt.Errorf("%s: %w", op, err)
	}

	return id, passHash, nil
}

func (s *Storage) UserDetails(ctx context.Context, userID int64) (email string, err error) {
    const op = "storage.UserDetails"

    query := "SELECT email FROM auth.users WHERE id = $1"

    err = s.db.QueryRow(ctx, query, userID).Scan(&email)
    if err != nil {
        if errors.Is(err, pgx.ErrNoRows) {
            return "", fmt.Errorf("%s: %w", op, ErrUserNotFound)
        }
        return "", fmt.Errorf("%s: %w", op, err)
    }

    return email, nil
}
