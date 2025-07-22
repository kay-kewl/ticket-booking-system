package service

import (
	"context"

	eventv1 "github.com/kay-kewl/ticket-booking-system/gen/go/event"
)

type EventProvider interface {
	ListEvents(ctx context.Context) ([]*eventv1.Event, error)
}

type Events struct {
	eventProvider EventProvider
}

func New(eventProvider EventProvider) *Events {
	return &Events{eventProvider: eventProvider}
}

func (e *Events) ListEvents(ctx context.Context) ([]*eventv1.Event, error) {
	return e.eventProvider.ListEvents(ctx)
}