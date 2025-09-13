package integration_tests

import (
	"context"
    "errors"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	bookingservice "github.com/kay-kewl/ticket-booking-system/services/booking-service/internal/service"
	bookingstorage "github.com/kay-kewl/ticket-booking-system/services/booking-service/internal/storage"
)

type simulatorPaymentGateway struct {
    simulator func() bool
}

func NewSimulatorPaymentGateway(sim func() bool) bookingservice.PaymentGateway {
    return &simulatorPaymentGateway{simulator: sim}
}

func (g *simulatorPaymentGateway) InitiatePayment(ctx context.Context, bookingID int64, amount float64) error {
    if g.simulator() {
        return nil
    }

    return errors.New("payment failed by simulator")
}

func TestBookingService_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	pgContainer, err := postgres.RunContainer(
		ctx,
		testcontainers.WithImage("postgres:16-alpine"),
		postgres.WithDatabase("test-db"),
		postgres.WithUsername("user"),
		postgres.WithPassword("password"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
			WithOccurrence(2).
			WithStartupTimeout(5*time.Second),
		),
	)
	require.NoError(t, err, "Failed to start postgres container")

	defer func() {
		require.NoError(t, pgContainer.Terminate(context.Background()), "Failed to terminate postgres container")
	}()

	connStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err, "Failed to get connection string")

	pool, err := pgxpool.New(ctx, connStr)
	require.NoError(t, err, "Failed to create db pool")
	defer pool.Close()

	applyMigrations(t, pool)

	storage := bookingstorage.New(pool)

	t.Run("Happy Path - Successful Booking", func(t *testing.T) {
        successGateway := NewSimulatorPaymentGateway(func() bool { return true })
        service := bookingservice.New(storage, successGateway)

		userID := int64(1)
		eventID := int64(1)
		seatIDs := []int64{1, 2}
		seedTestData(t, pool, userID, eventID, seatIDs)

		bookingID, err := service.CreateBooking(ctx, userID, eventID, seatIDs)

		require.NoError(t, err, "CreateBooking should not return an error on happy path")
		require.NotZero(t, bookingID, "Booking ID should not be zero")

		var bookingStatus string
		err = pool.QueryRow(
            ctx, 
			"SELECT status FROM booking.bookings WHERE id = $1",
			bookingID,
		).Scan(&bookingStatus)
		require.NoError(t, err, "Should be able to query booking status")
		require.Equal(t, "PENDING", bookingStatus, "Booking status should be PENDING")

		var seatStatus string
		err = pool.QueryRow(
			ctx,
			"SELECT status FROM event.seats WHERE id = $1", 
			seatIDs[0],
		).Scan(&seatStatus)
		require.NoError(t, err, "Should be able to query seat status")
		require.Equal(t, "RESERVED", seatStatus, "Seat status should be RESERVED")
	})

	t.Run("Failed Path - Payment Fails and Booking is Cancelled", func(t *testing.T) {
		failureGateway := NewSimulatorPaymentGateway(func() bool { return false })
		service := bookingservice.New(storage, failureGateway)

		userID := int64(2)
		eventID := int64(2)
		seatIDs := []int64{11}
		seedTestData(t, pool, userID, eventID, seatIDs)

		_, err := service.CreateBooking(ctx, userID, eventID, seatIDs)

		require.Error(t, err, "CreateBooking should return an error on payment failure")
		require.ErrorIs(t, err, bookingservice.ErrPaymentFailed, "Error should be of type ErrPaymentFailed")

		var bookingStatus string
		err = pool.QueryRow(
			ctx,
			"SELECT status FROM booking.bookings WHERE user_id = $1 ORDER BY id DESC LIMIT 1", 
			userID,
		).Scan(&bookingStatus)
		require.NoError(t, err, "Should be able to query booking status")
		require.Equal(t, "CANCELLED", bookingStatus, "Booking status should be CANCELLED after compensation")

		var seatStatus string
		err = pool.QueryRow(
			ctx,
			"SELECT status FROM event.seats WHERE id = $1",
			seatIDs[0],
		).Scan(&seatStatus)
		require.NoError(t, err, "Should be able to query seat status")
		require.Equal(t, "AVAILABLE", seatStatus, "Seat status should be AVAILABLE after compensation")
	})
}

func applyMigrations(t *testing.T, pool *pgxpool.Pool) {
	migrations := []string{
		`CREATE SCHEMA IF NOT EXISTS auth;`,
		`CREATE SCHEMA IF NOT EXISTS event;`,
		`CREATE SCHEMA IF NOT EXISTS booking;`,
		`CREATE TABLE IF NOT EXISTS auth.users (id BIGSERIAL PRIMARY KEY);`,
		`CREATE TABLE IF NOT EXISTS event.events (id BIGSERIAL PRIMARY KEY);`,
		`CREATE TYPE booking_status AS ENUM ('PENDING', 'CONFIRMED', 'CANCELLED', 'EXPIRED');`,
		`CREATE TABLE IF NOT EXISTS booking.bookings (id BIGSERIAL PRIMARY KEY, user_id BIGINT REFERENCES auth.users(id), event_id BIGINT REFERENCES event.events(id), status booking_status);`,
		`CREATE TYPE seat_status AS ENUM ('AVAILABLE', 'BOOKED', 'RESERVED');`,
		`CREATE TABLE IF NOT EXISTS event.seats (id BIGSERIAL PRIMARY KEY, event_id BIGINT REFERENCES event.events(id), status seat_status);`,
		`CREATE TABLE IF NOT EXISTS booking.booking_seats (booking_id BIGINT REFERENCES booking.bookings(id), seat_id BIGINT REFERENCES event.seats(id));`,
		`CREATE TABLE IF NOT EXISTS booking.outbox_messages (id BIGSERIAL PRIMARY KEY, exchange TEXT NOT NULL, routing_key TEXT NOT NULL, payload JSONB NOT NULL, created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), processed_at TIMESTAMPTZ);`,
	}
	for _, migration := range migrations {
		_, err := pool.Exec(context.Background(), migration)
		require.NoError(t, err, "Failed to apply migration: " + migration)
	}
}

func seedTestData(t *testing.T, pool *pgxpool.Pool, userID, eventID int64, seatIDs []int64) {
	_, err := pool.Exec(
		context.Background(),
		"INSERT INTO auth.users (id) VALUES ($1) ON CONFLICT DO NOTHING;",
		userID,
	)
	require.NoError(t, err)

	_, err = pool.Exec(
		context.Background(),
		"INSERT INTO event.events (id) VALUES ($1) ON CONFLICT DO NOTHING;",
		eventID,
	)
	require.NoError(t, err)

	for _, seatID := range seatIDs {
		_, err = pool.Exec(
			context.Background(),
			"INSERT INTO event.seats (id, event_id, status) VALUES ($1, $2, 'AVAILABLE') ON CONFLICT DO NOTHING;",
			seatID,
			eventID,
		)
		require.NoError(t, err)
	}
}

func init() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, nil)))
}
