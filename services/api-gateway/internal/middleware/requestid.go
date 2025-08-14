package middleware

import (
	"context"
	"net/http"

	"github.com/google/uuid"
	"github.com/kay-kewl/ticket-booking-system/internal/requestid"
)

func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			requestID := r.Header.Get("X-Request-ID")
			if requestID == "" {
				requestID = uuid.NewString()
			}
			w.Header().Set("X-Request-ID", requestID)

			ctx := context.WithValue(r.Context(), requestid.Key, requestID)
			next.ServeHTTP(w, r.WithContext(ctx))
		},
	)
}