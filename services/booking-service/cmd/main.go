package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	amqp "github.com/rabbitmq/amqp091-go"

	"github.com/kay-kewl/ticket-booking-system/internal/config"
	"github.com/kay-kewl/ticket-booking-system/internal/database"
	"github.com/kay-kewl/ticket-booking-system/internal/logging"
	"github.com/kay-kewl/ticket-booking-system/services/booking-service/internal/service"
	"github.com/kay-kewl/ticket-booking-system/services/booking-service/internal/storage"
	"github.com/kay-kewl/ticket-booking-system/services/booking-service/internal/worker"
	grpcserver "github.com/kay-kewl/ticket-booking-system/services/booking-service/internal/grpc"
)

func main() {
	logger := logging.New()

	logger.Info("Booking Service is starting up...")

	cfg, err := config.Load()
	if err != nil {
		logger.Error("Failed to load configuration", "error", err)
		os.Exit(1)
	}

	conn, err := amqp.Dial(cfg.RabbitMQURL)
	if err != nil {
		logger.Error("Failed to connect to RabbitMQ", "error", err)
		os.Exit(1)
	}
	defer conn.Close()
	logger.Info("Successfully connected to RabbitMQ")

	ch, err := conn.Channel()
	if err != nil {
		logger.Error("Failed to open a channel", "error", err)
		os.Exit(1)
	}
	defer ch.Close()

	err = setupRabbitMQTopology(ch)
	if err != nil {
		logger.Error("Failed to setup RabbitMQ topology", "error", err)
		os.Exit(1)
	}
	logger.Info("RabbitMQ topology setup successfully")

	dbPool, err := database.NewConnection(context.Background(), cfg.PostgresURL, logger)
	if err != nil {
		logger.Error("Failed to connect to database", "error", err)
		os.Exit(1)
	}

	defer dbPool.Close()

	bookingStorage := storage.New(dbPool)
	bookingService := service.New(bookingStorage)

	l, err := net.Listen("tcp", fmt.Sprintf(":%s", cfg.BookingGRPCPort))
	if err != nil {
		logger.Error("Failed to listen port", "error", err)
		os.Exit(1)
	}

	outboxWorker := worker.NewOutboxWorker(dbPool, ch, logger, 10 * time.Second)
	workerCtx, workerCancel := context.WithCancel(context.Background())
	defer workerCancel()
	go outboxWorker.Start(workerCtx)

	logger.Info("Booking Service ready. gRPC server listening", "address", l.Addr().String())

	grpcSrv := grpc.NewServer()

	grpcserver.Register(grpcSrv, bookingService)

	reflection.Register(grpcSrv)

	go func() {
		if err := grpcSrv.Serve(l); err != nil {
			logger.Error("Failed to serve gRPC", "error", err)
		}
	}()

	go func() {
		workerChannel, err := conn.Channel()
		if err != nil {
			logger.Error("Failed to open channel for worker", "error", err)
			return
		}
		defer workerChannel.Close()

		msgs, err := workerChannel.Consume(
			"bookings_expired_queue",
			"",
			false,
			false,
			false,
			false,
			nil,
		)
		if err != nil {
			logger.Error("Failed to start consumer", "error", err)
			return
		}

		logger.Info("Expiration worker started. Waiting for messages...")

		for d := range msgs {
			logger.Info("Received an expired booking message", "body", string(d.Body))

			// TODO: parse json, get booking_id, cancel reservation

			d.Ack(false)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	logger.Info("Shutting down gRPC server...")

	grpcSrv.GracefulStop()
	logger.Info("gRPC server stopped")
}

func setupRabbitMQTopology(ch *amqp.Channel) error {
	err := ch.ExchangeDeclare("bookings_exchange", "topic", true, false, false, false, nil)
	if err != nil {
		return fmt.Errorf("failed to declare exchange: %w", err)
	}

	err = ch.ExchangeDeclare("bookings_dlx", "fanout", true, false, false, false, nil)
	if err != nil {
		return fmt.Errorf("failed to declare dead-letter exchange: %w", err)
	}

	_, err = ch.QueueDeclare("bookings_expired_queue", true, false, false, false, nil)
	if err != nil {
		return fmt.Errorf("failed to declare worker queue: %w", err)
	}

	err = ch.QueueBind("bookings_expired_queue", "", "bookings_dlx", false, nil)
	if err != nil {
		return fmt.Errorf("failed to bind worker queue: %w", err)
	}

	_, err = ch.QueueDeclare("bookings_delay_15m", true, false, false, false, amqp.Table{
		"x-message-ttl":			90000,
		"x-dead-letter-exchange":	"bookings_dlx",
	})
	if err != nil {
		return fmt.Errorf("failed to declare delay queue: %w", err)
	}

	err = ch.QueueBind("bookings_delay_15m", "booking.created", "bookings_exchange", false, nil)
	if err != nil {
		return fmt.Errorf("failed to bind delay queue: %w", err)
	}

	return nil
}