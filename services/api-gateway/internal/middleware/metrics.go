package middleware

import (
	"net/http"
	"strconv"
	"time"

	"github.com/kay-kewl/ticket-booking-system/internal/metrics"
)

type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func Metrics(next http.Handler) http.Handler {
	return http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rw := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

			next.ServeHTTP(rw, r)

			duration := time.Since(start).Seconds()
			statusCodeStr := strconv.Itoa(rw.statusCode)

            path := r.URL.Path

			metrics.HTTPRequestsTotal.WithLabelValues(r.Method, path, statusCodeStr).Inc()
			metrics.HTTPRequestDuration.WithLabelValues(r.Method, path).Observe(duration)
		},
	)
}
