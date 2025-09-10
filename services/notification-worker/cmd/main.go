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
    "github.com/kay-kewl/ticket-booking-system/services/notification-worker/internal/service"
)

func main() {
    logger := logging.New()
    logger.Info("Starting notification worker...")

    cfg, err := config.Load()
    if err != nil {
        logger.Error("Failed to load configuration", "error", err)
        os.Exit(1)
    }

    rabbitManager := rabbitmq.NewConnectionManager(cfg.RabbitMQURL, logger)
    defer rabbitManager.Close()

    notificationService := service.New(logger)

    ctx, cancel := context.WithCancel(context.Background())
    var wg sync.WaitGroup

    wg.Add(1)
    go func() {
        defer wg.Done()
        notificationService.StartConsumer(ctx, rabbitManager)
    }()

    quit := make(chan os.Signal, 1)
    signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
    <-quit

    logger.Info("Shutting down notification worker...")
    cancel()
    wg.Wait()
    logger.Info("Notification worker stopped")
}
