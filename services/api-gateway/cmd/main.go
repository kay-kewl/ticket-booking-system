package main

import (
	"context"
	"log"
	"os"

	"github.com/kay-kewl/ticket-booking-system/internal/config"
	"github.com/kay-kewl/ticket-booking-system/internal/database"
	"github.com/kay-kewl/ticket-booking-system/internal/logging"
)

func main() {
	logger := logging.New()

	logger.Info("API Gateway is starting up...")

	cfg, error := config.Load()
	if error != nil {
		log.Fatalf("failed to load configuration: %v", error)
	}

	logger.Info("Configuration loaded, logger initialized")

	dbPool, error := database.NewConnection(context.Background(), cfg.PostgresURL, logger)
	if error != nil {
		logger.Error("Failed to connect to database", "error", error)
		os.Exit(1)
		// return
	}

	defer dbPool.Close()

	logger.Info("Application initialized successfully. Ready to start server.")
}