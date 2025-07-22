package grpc

import (
	"context"

	"google.golang.org/grpc"

	bookingv1 "github.com/kay-kewl/ticket-booking-system/gen/go/booking"
)

type Booking interface {
	CreateBooking(ctx context.Context, userID, eventID int64, seatIDs []int64) (int64, error)
}

type serverAPI struct {
	bookingv1.UnimplementedBookingServiceServer
	booking Booking
}

func Register(gRPCServer *grpc.Server, booking Booking) {
	bookingv1.RegisterBookingServiceServer(gRPCServer, &serverAPI{booking: booking})
}

func (s *serverAPI) CreateBooking(ctx context.Context, req *bookingv1.CreateBookingRequest) (*bookingv1.CreateBookingResponse, error) {
	bookingID, err := s.booking.CreateBooking(ctx, req.GetUserId(), req.GetEventId(), req.GetSeatIds())
	if err != nil {
		return nil, err
	}

	return &bookingv1.CreateBookingResponse{BookingId: bookingID}, nil
}