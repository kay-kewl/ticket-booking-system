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

func (s *Storage) CreateBooking(ctx context.Context, userID, eventID int64, seatIDs []int64) (int64, error) {
	const op = "storage.CreateBooking"

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", op, err)
	}
	defer tx.Rollback(ctx)

	var bookingID int64
	err = tx.QueryRow(ctx,
					  "INSERT INTO bookings(user_id, event_id) VALUES($1, $2) RETURNING id",
					  userID,
					  eventID,
	).Scan(&bookingID)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", op, err)
	}

	for _, seatID := range seatIDs {
		_, err = tx.Exec(ctx,
						 "INSERT INTO booking_seats(booking_id, seat_id) VALUES($1, $2)",
						 bookingID,
						 seatID,
		)
		if err != nil {
			return 0, fmt.Errorf("%s: %w", op, err)
		}
	}

	return bookingID, tx.Commit(ctx)
}