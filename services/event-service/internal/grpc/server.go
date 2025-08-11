package grpc

import (
	"context"

	"google.golang.org/grpc"

	eventv1 "github.com/kay-kewl/ticket-booking-system/gen/go/event"
)

type Events interface {
	ListEvents(ctx context.Context) ([]*eventv1.Event, error)
}

type serverAPI struct {
	eventv1.UnimplementedEventServiceServer
	events Events
}

func Register(gRPCServer *grpc.Server, events Events) {
	eventv1.RegisterEventServiceServer(gRPCServer, &serverAPI{events: events})
}

func (s *serverAPI) ListEvents(ctx context.Context, req *eventv1.ListEventsRequest) (*eventv1.ListEventsResponse, error) {
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