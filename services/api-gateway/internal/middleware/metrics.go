package middleware

import (
	"net/http"

	"github.com/kay-kewl/ticket-booking-system/internal/metrics"
)

func Metrics(next http.Handler) http.Handler {
	return http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			metrics.HTTPRequestsTotal.WithLabelValues(r.Method, r.URL.Path).Inc()
			next.ServeHTTP(w, r)
		},
	)
}