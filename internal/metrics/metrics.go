package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var HTTPRequestsTotal = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Name: "http_requests_total",
		Help: "Total number of HTTP requests",
	},
	[]string{"method", "path"},
)

var BookingsTotal = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Name: "bookings_total",
		Help: "Total number of created bookings by status",
	},
	[]string{"status"},
)