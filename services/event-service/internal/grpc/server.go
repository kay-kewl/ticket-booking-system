package grpc

import (
	"context"
	"log/slog"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	eventv1 "github.com/kay-kewl/ticket-booking-system/gen/go/event"
)

type Events interface {
	ListEvents(ctx context.Context, pageNumber, pageSize int32) ([]*eventv1.Event, int64, error)
}

type serverAPI struct {
	eventv1.UnimplementedEventServiceServer
	events Events
	log	   *slog.Logger
}

func Register(gRPCServer *grpc.Server, events Events, log *slog.Logger) {
	eventv1.RegisterEventServiceServer(gRPCServer, &serverAPI{events: events, log: log})
}

func (s *serverAPI) ListEvents(ctx context.Context, req *eventv1.ListEventsRequest) (*eventv1.ListEventsResponse, error) {
	s.log.InfoContext(ctx, "ListEvents request received in event-service")
	pageNumber := req.GetPageNumber()
	if pageNumber < 1 {
		pageNumber = 1
	}

	pageSize := req.GetPageSize()
	if pageSize < 1 {
		pageSize = 10
	}

	events, totalCount, err := s.events.ListEvents(ctx, pageNumber, pageSize)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to list events")
	}

	return &eventv1.ListEventsResponse{Events: events, TotalCount: totalCount}, nil
}
