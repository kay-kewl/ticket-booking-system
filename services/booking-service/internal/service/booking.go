package service

import (
	"context"
)

type BookingCreator interface {
	CreateBooking(ctx context.Context, userID, eventID int64, seatIDs []int64) (int64, error)
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
	const op = "Booking.CreateBooking"

	// TODO: validate seats

	return b.bookingCreator.CreateBooking(ctx, userID, eventID, seatIDs)
}