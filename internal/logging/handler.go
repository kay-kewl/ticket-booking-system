package logging

import (
	"context"
	"log/slog"

	"github.com/kay-kewl/ticket-booking-system/internal/requestid"
)

type RequestIDHandler struct {
	slog.Handler
}

func (h *RequestIDHandler) Handle(ctx context.Context, r slog.Record) error {
	if requestID, ok := requestid.Get(ctx); ok {
		r.AddAttrs(slog.String("request_id", requestID))
	}
	return h.Handler.Handle(ctx, r)
}