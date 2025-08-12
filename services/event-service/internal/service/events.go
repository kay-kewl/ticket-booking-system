package service

import (
	"context"

	eventv1 "github.com/kay-kewl/ticket-booking-system/gen/go/event"
)

type EventProvider interface {
	ListEvents(ctx context.Context, pageNumber, pageSize int32) ([]*eventv1.Event, int64, error)
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