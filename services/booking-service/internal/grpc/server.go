package grpc

import (
	"context"
	"errors"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	bookingv1 "github.com/kay-kewl/ticket-booking-system/gen/go/booking"
	"github.com/kay-kewl/ticket-booking-system/services/booking-service/internal/service"
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
		if errors.Is(service.ErrSeatNotAvailable, err) {
			return nil, status.Error(codes.FailedPrecondition, "seat has already been reserved")
		}
		return nil, status.Error(codes.Internal, "failed to create booking")
	}

	return &bookingv1.CreateBookingResponse{BookingId: bookingID}, nil
}