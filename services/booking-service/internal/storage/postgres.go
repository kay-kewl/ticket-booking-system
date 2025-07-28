package storage

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/pgconn"
)

var ErrSeatNotAvailable = errors.New("seat is not available or does not exist")

type Storage struct {
	db *pgxpool.Pool
}

func New(db *pgxpool.Pool) *Storage {
	return &Storage{db: db}
}

func (s *Storage) CreateBooking(ctx context.Context, userID, eventID int64, seatIDs []int64) (int64, error) {
	const op = "storage.CreateBooking"

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", op, err)
	}
	defer tx.Rollback(ctx)

	rows, err := tx.Query(ctx,
						  "SELECT id FROM seats WHERE id = ANY($1) AND status = 'AVAILABLE' FOR UPDATE",
						  seatIDs)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, pgErr) {
			return 0, fmt.Errorf("%s: %w", op, ErrSeatNotAvailable)
		}
		return 0, fmt.Errorf("%s: failed to lock seats: %w", op, err)
	}

	var lockedSeatIDs []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return 0, fmt.Errorf("%s: failed to scan locked seat id: %w", op, err)
		}
		lockedSeatIDs = append(lockedSeatIDs, id)
	}
	rows.Close()

	if len(lockedSeatIDs) != len(seatIDs) {
		return 0, ErrSeatNotAvailable
	}

	var bookingID int64
	err = tx.QueryRow(ctx,
					  "INSERT INTO bookings(user_id, event_id, status) VALUES($1, $2, 'PENDING') RETURNING id",
					  userID,
					  eventID,
					  ).Scan(&bookingID)
	if err != nil {
		return 0, fmt.Errorf("%s: failed to create booking: %w", op, err)
	}

	for _, seatID := range lockedSeatIDs {
		_, err = tx.Exec(ctx,
						 "INSERT INTO booking_seats(booking_id, seat_id) VALUES($1, $2)",
						 bookingID,
						 seatID)
		if err != nil {
			return 0, fmt.Errorf("%s: failed to link seat to booking: %w", op, err)
		}
	}

	_, err = tx.Exec(ctx, "UPDATE seats SET status = 'RESERVED' WHERE id = ANY($1)", lockedSeatIDs)
	if err != nil {
		return 0, fmt.Errorf("%s: failed to update seat status: %w", op, err)
	}

	return bookingID, tx.Commit(ctx)
}