package main

import (
    "context"
    "log/slog"
    "os"
    "os/signal"
    "sync"
    "syscall"

    "github.com/kay-kewl/ticket-booking-system/internal/config"
    "github.com/kay-kewl/ticket-booking-system/internal/logging"
    "github.com/kay-kewl/ticket-booking-system/internal/rabbitmq"
    "github.com/kay-kewl/ticket-booking-system/services/ticket-worker/internal/service"
)

func main() {
    logger := logging.New()
    logger.Info("Starting ticket worker...")

    cfg, err := config.Load()
    if err != nil {
        logger.Error("Failed to load configuration", "error", err)
        os.Exit(1)
    }

    outputPath := os.Getenv("TICKET_OUTPUT_PATH")
    if outputPath == "" {
        outputPath = "/tickets"
    }

    rabbitManager := rabbitmq.NewConnectionManager(cfg.RabbitMQURL, logger)
    defer rabbitManager.Close()

    ticketService := service.New(outputPath, logger)

    ctx, cancel := context.WithCancel(context.Background())
    var wg sync.WaitGroup

    wg.Add(1)
    go func() {
        defer wg.Done()
        ticketService.StartConsumer(ctx, rabbitManager)
    }()

    quit := make(chan os.Signal, 1)
    signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
    <-quit

    logger.Info("Shutting down ticket worker...")
    cancel()
    wg.Wait()
    logger.Info("Ticket worker stopped")
}
