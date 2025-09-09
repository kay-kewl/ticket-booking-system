package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	_ "google.golang.org/grpc/health"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

	authv1 "github.com/kay-kewl/ticket-booking-system/gen/go/auth"
	bookingv1 "github.com/kay-kewl/ticket-booking-system/gen/go/booking"
	eventv1 "github.com/kay-kewl/ticket-booking-system/gen/go/event"
	"github.com/kay-kewl/ticket-booking-system/internal/config"
	"github.com/kay-kewl/ticket-booking-system/internal/grpc/interceptors"
	"github.com/kay-kewl/ticket-booking-system/internal/logging"
	"github.com/kay-kewl/ticket-booking-system/internal/telemetry"
	"github.com/kay-kewl/ticket-booking-system/services/api-gateway/internal/handler"
	apimiddleware "github.com/kay-kewl/ticket-booking-system/services/api-gateway/internal/middleware"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	logger := logging.New()

	logger.Info("API Gateway is starting up...")

	shutdown, err := telemetry.InitTracerProvider(context.Background(), "api-gateway", "jaeger:4317")
	if err != nil {
		logger.Error("Failed to initialize tracer provider", "error", err)
		os.Exit(1)
	}
	defer shutdown()

	tracingOpt := grpc.WithStatsHandler(otelgrpc.NewClientHandler())
	interceptorOpt := grpc.WithChainUnaryInterceptor(
		interceptors.ClientRequestIDInterceptor(),
	)

	cfg, err := config.Load()
	if err != nil {
		logger.Error("Failed to load configuration", "error", err)
		os.Exit(1)
	}

	// ctx, cancel := context.WithTimeout(context.Background(), 5 * time.Second)
	// defer cancel()

	retryPolicy := `{
		"methodConfig": [{
			"name": [{}],
			"retryPolicy": {
				"maxAttempts": 5,
				"initialBackoff": "0.1s",
				"maxBackoff": "5s",
				"backoffMultiplier": 2.0,
				"retryableStatusCodes": [ "UNAVAILABLE" ]
			}
		}]
	}`

	authServiceAddr := fmt.Sprintf("auth-service:%s", cfg.AuthGRPCPort)
	authServiceConn, err := grpc.NewClient(
		authServiceAddr,
		grpc.WithDefaultServiceConfig(retryPolicy),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		tracingOpt,
		interceptorOpt,
	)
	if err != nil {
		logger.Error("Failed to dial auth-service", "error", err)
		os.Exit(1)
	}
	defer authServiceConn.Close()
	authClient := authv1.NewAuthClient(authServiceConn)
	logger.Info("gRPC connection to auth-service established")

	eventServiceAddr := fmt.Sprintf("event-service:%s", cfg.EventGRPCPort)
	eventServiceConn, err := grpc.NewClient(
		eventServiceAddr,
		grpc.WithDefaultServiceConfig(retryPolicy),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		tracingOpt,
		interceptorOpt,
	)
	if err != nil {
		logger.Error("Failed to dial event-service", "error", err)
		os.Exit(1)
	}
	defer eventServiceConn.Close()
	eventClient := eventv1.NewEventServiceClient(eventServiceConn)
	logger.Info("gRPC connection to event-service established")

	bookingServiceAddr := fmt.Sprintf("booking-service:%s", cfg.BookingGRPCPort)
	bookingServiceConn, err := grpc.NewClient(
		bookingServiceAddr,
		grpc.WithDefaultServiceConfig(retryPolicy),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		tracingOpt,
		interceptorOpt,
	)
	if err != nil {
		logger.Error("Failed to dial booking-service", slog.String("addr", bookingServiceAddr), "error", err)
		os.Exit(1)
	}
	defer bookingServiceConn.Close()
	bookingClient := bookingv1.NewBookingServiceClient(bookingServiceConn)
	logger.Info("gRPC connection to booking-service established")

	go func() {
		metricsMux := http.NewServeMux()
		metricsMux.Handle("/metrics", promhttp.Handler())
		logger.Info("Starting metrics server on port 9100")
		if err := http.ListenAndServe(":9100", metricsMux); err != nil {
			logger.Error("Metrics server failed", "error", err)
		}
	}()

	h := handler.New(authClient, bookingClient, eventClient, cfg.PaymentWebhookSecret, logger)

	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/register", h.Register)
	mux.HandleFunc("POST /api/v1/login", h.Login)
	mux.HandleFunc("GET /api/v1/events", h.ListEvents)
	mux.HandleFunc("POST /api/v1/bookings", h.CreateBooking)
	mux.HandleFunc("POST /api/v1/payments/webhook", h.PaymentWebhook)
	// mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
	// 	if err := dbPool.Ping(r.Context()); err != nil {
	// 		logger.Error("Database ping failed", "error", err)
	// 		w.WriteHeader(http.StatusServiceUnavailable)
	// 		w.Write([]byte("Database down"))
	// 		return
	// 	}
	// 	w.WriteHeader(http.StatusOK)
	// 	w.Write([]byte("OK"))
	// })

	var handlerWithMiddleware http.Handler = mux
	handlerWithMiddleware = otelhttp.NewHandler(handlerWithMiddleware, "http.server")
	// handlerWithMiddleware = apimiddleware.Idempotency(handlerWithMiddleware)
	handlerWithMiddleware = apimiddleware.Metrics(handlerWithMiddleware)
	handlerWithMiddleware = apimiddleware.RequestID(handlerWithMiddleware)

	srv := &http.Server{
		Addr:         ":" + cfg.APIPort,
		Handler:      handlerWithMiddleware,
		IdleTimeout:  300 * time.Second,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	go func() {
		logger.Info("Starting HTTP server on port", "port", cfg.APIPort)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("Server failed", "error", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	logger.Info("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.Error("Server forced to shutdown", "error", err)
	}

	logger.Info("Server waiting")
}
