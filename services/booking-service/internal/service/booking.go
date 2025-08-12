package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/kay-kewl/ticket-booking-system/services/booking-service/internal/storage"
)

var ErrSeatNotAvailable = errors.New("seat is not available")

type BookingCreator interface {
	CreateBooking(ctx context.Context, userID, eventID int64, seatIDs []int64) (int64, error)
	CancelExpiredBooking(ctx context.Context, bookingID int64) error
}

type Booking struct {
	bookingCreator	BookingCreator
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

	return bookingID, nil 
}

func (b *Booking) CancelExpiredBooking(ctx context.Context, bookingID int64) error {
	const op = "service.CancelExpiredBooking"

	if err := b.bookingCreator.CancelExpiredBooking(ctx, bookingID); err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}

	return nil 
}