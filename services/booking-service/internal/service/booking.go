package service

import (
    "bytes"
	"context"
    "encoding/json"
	"errors"
	"fmt"
    "io"
	"log/slog"
    "net/http"
    "time"

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

type PaymentGateway interface {
    InitiatePayment(ctx context.Context, bookingID int64, amount float64) error
}

type Booking struct {
	bookingCreator 		BookingCreator
    paymentGateway      PaymentGateway
}

func New(bookingCreator BookingCreator, gateway PaymentGateway) *Booking {
	return &Booking{
		bookingCreator: 	bookingCreator,
        paymentGateway:     gateway,
	}
}

type httpPaymentGateway struct {
    client  *http.Client
    url     string
}

func NewHTTPPaymentGateway(url string) PaymentGateway {
    return &httpPaymentGateway{
        client: &http.Client{Timeout: 1 * time.Minute},
        url:    url,
    }
}

func (g *httpPaymentGateway) InitiatePayment(ctx context.Context, bookingID int64, amount float64) error {
    payload := map[string]any{
        "booking_id":   bookingID,
        "amount":       amount,
    }

    body, err := json.Marshal(payload)
    if err != nil {
        return fmt.Errorf("failed to marshal payment payload: %w", err)
    }

    req, err := http.NewRequestWithContext(ctx, http.MethodPost, g.url, bytes.NewReader(body))
    if err != nil {
        return fmt.Errorf("failed to build payment request: %w", err)
    }
    req.Header.Set("Content-Type", "application/json")

    resp, err := g.client.Do(req)
    if err != nil {
        return fmt.Errorf("payment request failed: %w", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode < 200 || resp.StatusCode >= 300 {
        respBody, _ := io.ReadAll(resp.Body)
        return fmt.Errorf("payment service returned status %d: %s", resp.StatusCode, string(respBody))
    }

    return err
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
    
    err = b.paymentGateway.InitiatePayment(ctx, bookingID, 1500.0)
    if err != nil {
        slog.Error("failed to initiate payment, compensating booking", "booking_id", bookingID, "error", err)
        compensationCtx, cancel := context.WithTimeout(context.Background(), 1 * time.Minute)
        defer cancel()
        if compensationErr := b.bookingCreator.CancelBooking(compensationCtx, bookingID); compensationErr != nil {
            slog.Error("critical: failed to compensate booking", "booking_id", bookingID, "error", compensationErr)
        }
        return 0, ErrPaymentFailed
    }

    slog.Info("Booking created and payment initiated successfully", "booking_id", bookingID)
    return bookingID, nil
}

func (b *Booking) ConfirmBooking(ctx context.Context, bookingID int64) error {
    const op = "service.ConfirmBooking"

    err := b.bookingCreator.ConfirmBooking(ctx, bookingID)
    if err != nil {
        return fmt.Errorf("%s: %w", op, err)
    }

    return nil
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
