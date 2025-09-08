package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"net"
	"net/http"
	"syscall"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"

	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"

	"github.com/kay-kewl/ticket-booking-system/internal/config"
	"github.com/kay-kewl/ticket-booking-system/internal/database"
	"github.com/kay-kewl/ticket-booking-system/internal/grpc/interceptors"
	"github.com/kay-kewl/ticket-booking-system/internal/logging"
	"github.com/kay-kewl/ticket-booking-system/internal/telemetry"
	grpcserver "github.com/kay-kewl/ticket-booking-system/services/auth-service/internal/grpc"
	"github.com/kay-kewl/ticket-booking-system/services/auth-service/internal/service"
	"github.com/kay-kewl/ticket-booking-system/services/auth-service/internal/storage"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	logger := logging.New()

	logger.Info("Auth Service is starting up...")

	shutdown, err := telemetry.InitTracerProvider(context.Background(), "auth-service", "jaeger:4317")
	if err != nil {
		logger.Error("Failed to initialize tracer provider", "error", err)
		os.Exit(1)
	}
	defer shutdown()

	go func() {
		metricsMux := http.NewServeMux()
		metricsMux.Handle("/metrics", promhttp.Handler())
		port := "9101"
		logger.Info("Starting metrics server", "port", port)
		if err := http.ListenAndServe(":"+port, metricsMux); err != nil {
			logger.Error("Metrics server failed to start", "error", err)
		}
	}()

	cfg, err := config.Load()
	if err != nil {
		logger.Error("Failed to load configuration", "error", err)
		os.Exit(1)
	}
	if cfg.JWTSecret == "" {
		logger.Error("JWT_SECRET environmental variable is not set")
		os.Exit(1)
	}

	dbPool, err := database.NewConnection(context.Background(), cfg.PostgresURL, logger)
	if err != nil {
		logger.Error("Failed to connect to database", "error", err)
		os.Exit(1)
	}

	defer dbPool.Close()

	authStorage := storage.New(dbPool)
	authService := service.New(cfg.JWTSecret, 1*time.Hour, authStorage, authStorage)

	l, err := net.Listen("tcp", fmt.Sprintf(":%s", cfg.AuthGRPCPort))
	if err != nil {
		logger.Error("Failed to listen port", "error", err)
		os.Exit(1)
	}

	logger.Info("Auth Service ready. gRPC server listening", "address", l.Addr().String())

	healthSrv := health.NewServer()

	grpcSrv := grpc.NewServer(
		grpc.ChainUnaryInterceptor(
			otelgrpc.UnaryServerInterceptor(),
			interceptors.ServerRequestIDInterceptor(),
		),
	)

	grpcserver.Register(grpcSrv, authService)

	grpc_health_v1.RegisterHealthServer(grpcSrv, healthSrv)

	reflection.Register(grpcSrv)

	healthSrv.SetServingStatus("", grpc_health_v1.HealthCheckResponse_SERVING)

	go func() {
		if err := grpcSrv.Serve(l); err != nil {
			logger.Error("Failed to serve gRPC", "error", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	logger.Info("Shutting down gRPC server...")

	healthSrv.SetServingStatus("", grpc_health_v1.HealthCheckResponse_NOT_SERVING)

	grpcSrv.GracefulStop()
	logger.Info("gRPC server stopped")
}
