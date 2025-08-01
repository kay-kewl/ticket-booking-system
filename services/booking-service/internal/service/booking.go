package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	amqp "github.com/rabbitmq/amqp091-go"

	"github.com/kay-kewl/ticket-booking-system/services/booking-service/internal/storage"
)

var ErrSeatNotAvailable = errors.New("seat is not available")

type BookingCreator interface {
	CreateBooking(ctx context.Context, userID, eventID int64, seatIDs []int64) (int64, error)
}

type Booking struct {
	bookingCreator	BookingCreator
	amqpChannel		*amqp.Channel
}

func New(bookingCreator BookingCreator, amqpChannel *amqp.Channel) *Booking {
	return &Booking{
		bookingCreator: bookingCreator,
		amqpChannel:	amqpChannel,
	}
}

func (b *Booking) CreateBooking(ctx context.Context, userID, eventID int64, seatIDs []int64) (int64, error) {
	const op = "Booking.CreateBooking"

	// TODO: validate seats
	bookingID, err := b.bookingCreator.CreateBooking(ctx, userID, eventID, seatIDs)
	if err != nil {
		if errors.Is(err, storage.ErrSeatNotAvailable) {
			return 0, fmt.Errorf("%s: %w", op, ErrSeatNotAvailable)
		}
		return 0, fmt.Errorf("%s: %w", op, err)
	}

	msgBody, _ := json.Marshal(map[string]int64{"booking_id": bookingID})

	err = b.amqpChannel.PublishWithContext(
		ctx,
		"bookings_exchange",
		"booking.created",
		false,
		false,
		amqp.Publishing{
			ContentType:	"application/json",
			Body:			msgBody,
		},
	)

	if err != nil {
		// TODO: critical error: booking created but failed to publish expiration message
	}

	return bookingID, nil 
}