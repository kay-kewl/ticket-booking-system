package main

import (
	"context"
	"os"
	"os/signal"
	"net/http"
	"syscall"
	"time"

	"github.com/kay-kewl/ticket-booking-system/internal/config"
	"github.com/kay-kewl/ticket-booking-system/internal/database"
	"github.com/kay-kewl/ticket-booking-system/internal/logging"
)

func main() {
	logger := logging.New()

	logger.Info("API Gateway is starting up...")

	cfg, error := config.Load()
	if error != nil {
		logger.Error("failed to load configuration", "error", error)
		os.Exit(-1)
	}

	logger.Info("Configuration loaded, logger initialized")

	dbPool, error := database.NewConnection(context.Background(), cfg.PostgresURL, logger)
	if error != nil {
		logger.Error("Failed to connect to database", "error", error)
		os.Exit(1)
	}

	defer dbPool.Close()

	logger.Info("Application initialized successfully. Ready to start server.")

	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

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

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.Error("Server forced to shutdown", "error", err)
	}

	logger.Info("Server waiting")
}