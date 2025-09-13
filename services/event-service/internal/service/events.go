package service

import (
	"context"

	eventv1 "github.com/kay-kewl/ticket-booking-system/gen/go/event"
)

type EventProvider interface {
	ListEvents(ctx context.Context, pageNumber, pageSize int32) ([]*eventv1.Event, int64, error)
    GetEvent(ctx context.Context, eventID int64) (*eventv1.Event, error)
}

type Events struct {
	eventProvider EventProvider
}

func New(eventProvider EventProvider) *Events {
	return &Events{eventProvider: eventProvider}
}

func (e *Events) ListEvents(ctx context.Context, pageNumber, pageSize int32) ([]*eventv1.Event, int64, error) {
	return e.eventProvider.ListEvents(ctx, pageNumber, pageSize)
}

func (e *Events) GetEvent(ctx context.Context, eventID int64) (*eventv1.Event, error) {
    return e.eventProvider.GetEvent(ctx, eventID)
}
