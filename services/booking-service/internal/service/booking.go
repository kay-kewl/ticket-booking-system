package service

import (
    "bytes"
	"context"
    "encoding/json"
	"errors"
	"fmt"
	"log/slog"
    "net/http"

	"github.com/kay-kewl/ticket-booking-system/services/booking-service/internal/storage"
)

var ErrSeatNotAvailable = errors.New("seat is not available")
var ErrPaymentFailed = errors.New("failed to initiate payment")

type BookingCreator interface {
	CreateBooking(ctx context.Context, userID, eventID int64, seatIDs []int64) (int64, error)
	ConfirmBooking(ctx context.Context, bookingID int64) error
	CancelBooking(ctx context.Context, bookingID int64) error
	ExpireBooking(ctx context.Context, bookingID int64) error
}

type PaymentSimulator func() bool

type Booking struct {
	bookingCreator 		BookingCreator
	paymentServiceURL   string
}

func New(bookingCreator BookingCreator, paymentServiceURL string) *Booking {
	return &Booking{
		bookingCreator: 	bookingCreator,
		paymentServiceURL:	paymentServiceURL,
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

    paymentReqPayload := map[string]interface{}{
        "booking_id":   bookingID,
        "amount":       1500.0,
    }

    body, err := json.Marshal(paymentReqPayload)
    if err != nil {
        b.bookingCreator.CancelBooking(context.Background(), bookingID)
        return 0, fmt.Errorf("failed to create payment request payload: %w", err)
    }

    resp, err := http.Post(b.paymentServiceURL, "application/json", bytes.NewBuffer(body))
    if err != nil || resp.StatusCode != http.StatusAccepted {
        slog.Error("failed to initiate payment", "booking_id", bookingID, "error", err, "status_code", resp.StatusCode)
        b.bookingCreator.CancelBooking(context.Background(), bookingID)
        return 0, ErrPaymentFailed
    }

    slog.Info("Booking created and payment initiated successfully", "booking_id", bookingID)
    return bookingID, nil
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
