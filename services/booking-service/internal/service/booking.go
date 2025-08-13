package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/kay-kewl/ticket-booking-system/services/booking-service/internal/storage"
)

var ErrSeatNotAvailable = errors.New("seat is not available")
var ErrPaymentFailed = errors.New("payment failed")

type BookingCreator interface {
	CreateBooking(ctx context.Context, userID, eventID int64, seatIDs []int64) (int64, error)
	ConfirmBooking(ctx context.Context, bookingID int64) error
	CancelBooking(ctx context.Context, bookingID int64) error
	ExpireBooking(ctx context.Context, bookingID int64) error
}

type Booking struct {
	bookingCreator BookingCreator
}

func New(bookingCreator BookingCreator) *Booking {
	return &Booking{
		bookingCreator: bookingCreator,
	}
}

func (b *Booking) CreateBooking(ctx context.Context, userID, eventID int64, seatIDs []int64) (int64, error) {
	const op = "service.CreateBooking"

	// TODO: validate seats
	bookingID, err := b.bookingCreator.CreateBooking(ctx, userID, eventID, seatIDs)
	if err != nil {
		if errors.Is(err, storage.ErrSeatNotAvailable) {
			return 0, fmt.Errorf("%s: %w", op, ErrSeatNotAvailable)
		}
		return 0, fmt.Errorf("%s: %w", op, err)
	}

	paymentSuccessful := false

	if paymentSuccessful {
		err = b.bookingCreator.ConfirmBooking(ctx, bookingID)
		if err != nil {
			slog.Error("CRITICAL: payment was successful but failed to confirm booking", "bookingID", bookingID, "error", err)
			return 0, fmt.Errorf("%s: critical error - failed to confirm booking after payment: %w", op, err)
		}
		return bookingID, nil
	} 

	err = b.bookingCreator.CancelBooking(ctx, bookingID)
	if err != nil {
		slog.Error("Payment failed and failed to automatically cancel booking", "bookingID", bookingID, "error", err)
	}
	return 0, ErrPaymentFailed
}

func (b *Booking) CancelBooking(ctx context.Context, bookingID int64) error {
	const op = "service.CancelBooking"

	err := b.bookingCreator.CancelBooking(ctx, bookingID)
	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}

	return nil
}

func (b *Booking) ExpireBooking(ctx context.Context, bookingID int64) error {
	const op = "service.ExpireBooking"

	err := b.bookingCreator.ExpireBooking(ctx, bookingID)
	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}

	return nil
}
