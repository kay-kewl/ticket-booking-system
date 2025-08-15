package storage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/kay-kewl/ticket-booking-system/internal/metrics"
)

var ErrSeatNotAvailable = errors.New("seat is not available or does not exist")
var ErrBookingCannotBeChanged = errors.New("booking is not in a state that can be changed")

type Storage struct {
	db *pgxpool.Pool
}

type OutboxMessage struct {
	Exchange   string
	RoutingKey string
	Payload    []byte
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

	rows, err := tx.Query(
		ctx,
		"SELECT id FROM event.seats WHERE id = ANY($1) AND event_id = $2 AND status = 'AVAILABLE' ORDER BY id FOR UPDATE",
		seatIDs,
		eventID,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			return 0, fmt.Errorf("%s: %w", op, ErrSeatNotAvailable)
		}
		return 0, fmt.Errorf("%s: failed to lock seats: %w", op, err)
	}

	var lockedSeatIDs []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return 0, fmt.Errorf("%s: failed to scan locked seat id: %w", op, err)
		}
		lockedSeatIDs = append(lockedSeatIDs, id)
	}
	rows.Close()

	if len(lockedSeatIDs) != len(seatIDs) {
		return 0, ErrSeatNotAvailable
	}

	var bookingID int64
	err = tx.QueryRow(
		ctx,
		"INSERT INTO booking.bookings(user_id, event_id, status) VALUES($1, $2, 'PENDING') RETURNING id",
		userID,
		eventID,
	).Scan(&bookingID)
	if err != nil {
		return 0, fmt.Errorf("%s: failed to create booking: %w", op, err)
	}

	for _, seatID := range lockedSeatIDs {
		_, err = tx.Exec(
			ctx,
			"INSERT INTO booking.booking_seats(booking_id, seat_id) VALUES($1, $2)",
			bookingID,
			seatID,
		)
		if err != nil {
			return 0, fmt.Errorf("%s: failed to link seat to booking: %w", op, err)
		}
	}

	_, err = tx.Exec(ctx, "UPDATE event.seats SET status = 'RESERVED' WHERE id = ANY($1)", lockedSeatIDs)
	if err != nil {
		return 0, fmt.Errorf("%s: failed to update seat status: %w", op, err)
	}

	payload, err := json.Marshal(map[string]int64{"booking_id": bookingID})
	if err != nil {
		return 0, fmt.Errorf("%s: failed to marshal outbox message: %w", op, err)
	}

	_, err = tx.Exec(
		ctx,
		"INSERT INTO booking.outbox_messages (exchange, routing_key, payload) VALUES ($1, $2, $3::jsonb)",
		"bookings_exchange",
		"booking.created",
		payload,
	)
	if err != nil {
		return 0, fmt.Errorf("%s: failed to save outbox message: %w", op, err)
	}

	return bookingID, tx.Commit(ctx)
}

func (s *Storage) ConfirmBooking(ctx context.Context, bookingID int64) error {
	const op = "storage.ConfirmBooking"

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("%s: failed to begin transaction: %w", op, err)
	}
	defer tx.Rollback(ctx)

	tag, err := tx.Exec(
		ctx,
		"UPDATE booking.bookings SET status = 'CONFIRMED' WHERE id = $1 AND status = 'PENDING'",
		bookingID,
	)
	if err != nil {
		return fmt.Errorf("%s: failed to update booking status: %w", op, err)
	}
	if tag.RowsAffected() == 0 {
		return ErrBookingCannotBeChanged
	}

	metrics.BookingsTotal.WithLabelValues("confirmed").Inc()

	_, err = tx.Exec(
		ctx,
		"UPDATE event.seats SET status = 'BOOKED' WHERE id IN (SELECT seat_id FROM booking.booking_seats WHERE booking_id = $1)",
		bookingID,
	)
	if err != nil {
		return fmt.Errorf("%s: failed to update seat status to BOOKED: %w", op, err)
	}

	payload, err := json.Marshal(map[string]int64{"booking_id": bookingID})
	if err != nil {
		return fmt.Errorf("%s: failed to marshal outbox payload: %w", op, err)
	}
	_, err = tx.Exec(
		ctx,
		"INSERT INTO booking.outbox_messages (exchange, routing_key, payload) VALUES ($1, $2, $3::jsonb)",
		"bookings_exchange",
		"booking.confirmed",
		payload,
	)
	if err != nil {
		return fmt.Errorf("%s: failed to save outbox message: %w", op, err)
	}

	return tx.Commit(ctx)
}

func (s *Storage) CancelBooking(ctx context.Context, bookingID int64) error {
	const op = "storage.CancelBooking"

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("%s: failed to begin transaction: %w", op, err)
	}
	defer tx.Rollback(ctx)

	err = s.updateBookingStatus(ctx, tx, bookingID, "CANCELLED", "booking.cancelled")
	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}

	return tx.Commit(ctx)
}

func (s *Storage) ExpireBooking(ctx context.Context, bookingID int64) error {
	const op = "storage.ExpireBooking"

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("%s: failed to begin transaction: %w", op, err)
	}
	defer tx.Rollback(ctx)

	err = s.updateBookingStatus(ctx, tx, bookingID, "EXPIRED", "booking.expired")
	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}

	return tx.Commit(ctx)
}

func (s *Storage) updateBookingStatus(ctx context.Context, tx pgx.Tx, bookingID int64, newStatus, routingKey string) error {
	const op = "storage.internal.updateBookingStatus"

	tag, err := tx.Exec(
		ctx,
		"UPDATE booking.bookings SET status = $1 WHERE id = $2 AND status = 'PENDING'",
		newStatus,
		bookingID,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "22P02" {
			return fmt.Errorf("%s: invalid status value %s: %w", op, newStatus, err)
		}
		return fmt.Errorf("%s: failed to update booking status: %w", op, err)
	}

	if tag.RowsAffected() == 0 {
		return nil
	}

	metrics.BookingsTotal.WithLabelValues(newStatus).Inc()

	_, err = tx.Exec(
		ctx,
		"UPDATE event.seats SET status = 'AVAILABLE' WHERE id IN (SELECT seat_id FROM booking.booking_seats WHERE booking_id = $1)",
		bookingID,
	)
	if err != nil {
		return fmt.Errorf("%s: failed to release seats: %w", op, err)
	}

	payload, err := json.Marshal(map[string]interface{}{"booking_id": bookingID, "reason": newStatus})
	if err != nil {
		return fmt.Errorf("%s: failed to marshal outbox payload: %w", op, err)
	}

	_, err = tx.Exec(
		ctx,
		"INSERT INTO booking.outbox_messages (exchange, routing_key, payload) VALUES ($1, $2, $3::jsonb)",
		"bookings_exchange",
		routingKey,
		payload,
	)
	if err != nil {
		return fmt.Errorf("%s: failed to save outbox message: %w", op, err)
	}

	return nil
}
