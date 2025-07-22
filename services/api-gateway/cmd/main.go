package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	authv1 "github.com/kay-kewl/ticket-booking-system/gen/go/auth"
	"github.com/kay-kewl/ticket-booking-system/internal/config"
	"github.com/kay-kewl/ticket-booking-system/internal/logging"
	"github.com/kay-kewl/ticket-booking-system/services/api-gateway/internal/handler"
)

func main() {
	logger := logging.New()

	logger.Info("API Gateway is starting up...")

	cfg, err := config.Load()
	if err != nil {
		logger.Error("Failed to load configuration", "error", err)
		os.Exit(1)
	}

	authServiceConn, err := grpc.NewClient("auth-service:50051", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		logger.Error("Failed to connect to auth-service", "error", err)
		os.Exit(1)
	}
	defer authServiceConn.Close()

	authClient := authv1.NewAuthClient(authServiceConn)

	// dbPool, err := database.NewConnection(context.Background(), cfg.PostgresURL, logger)
	// if err != nil {
	// 	logger.Error("Failed to connect to database", "error", err)
	// 	os.Exit(1)
	// }
	// defer dbPool.Close()

	logger.Info("gRPC connection to auth-service established")

	h := handler.New(authClient, logger)

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/register", h.Register)
	mux.HandleFunc("/api/v1/login", h.Login)
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

	srv := &http.Server{
		Addr: 		":" + cfg.APIPort,
		Handler: 	mux,
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

	ctx, cancel := context.WithTimeout(context.Background(), 5 * time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.Error("Server forced to shutdown", "error", err)
	}

	logger.Info("Server waiting")
}