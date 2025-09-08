package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"

	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"

	amqp "github.com/rabbitmq/amqp091-go"

	"github.com/kay-kewl/ticket-booking-system/internal/config"
	"github.com/kay-kewl/ticket-booking-system/internal/database"
	"github.com/kay-kewl/ticket-booking-system/internal/grpc/interceptors"
	"github.com/kay-kewl/ticket-booking-system/internal/logging"
	"github.com/kay-kewl/ticket-booking-system/internal/rabbitmq"
	"github.com/kay-kewl/ticket-booking-system/internal/telemetry"
	grpcserver "github.com/kay-kewl/ticket-booking-system/services/booking-service/internal/grpc"
	"github.com/kay-kewl/ticket-booking-system/services/booking-service/internal/service"
	"github.com/kay-kewl/ticket-booking-system/services/booking-service/internal/storage"
	"github.com/kay-kewl/ticket-booking-system/services/booking-service/internal/worker"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	logger := logging.New()

	logger.Info("Booking Service is starting up...")

	shutdown, err := telemetry.InitTracerProvider(context.Background(), "booking-service", "jaeger:4317")
	if err != nil {
		logger.Error("Failed to initialize tracer provider", "error", err)
		os.Exit(1)
	}
	defer shutdown()

	go func() {
		metricsMux := http.NewServeMux()
		metricsMux.Handle("/metrics", promhttp.Handler())

		port := "9103"
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

	rabbitmqManager := rabbitmq.NewConnectionManager(cfg.RabbitMQURL, logger)
	defer rabbitmqManager.Close()

	logger.Info("Waiting for RabbitMQ connection...")
	rabbitmqManager.WaitUntilReady()
	logger.Info("RabbitMQ connection is ready")

	setupCh, err := rabbitmqManager.GetChannel()
	if err != nil {
		logger.Error("Failed to get channel for topology setup", "error", err)
		os.Exit(1)
	}
	if err := setupRabbitMQTopology(setupCh); err != nil {
		logger.Error("Failed to setup RabbitMQ topology", "error", err)
		os.Exit(1)
	}
	setupCh.Close()
	logger.Info("RabbitMQ topology setup successfully")

	dbPool, err := database.NewConnection(context.Background(), cfg.PostgresURL, logger)
	if err != nil {
		logger.Error("Failed to connect to database", "error", err)
		os.Exit(1)
	}

	defer dbPool.Close()

	bookingStorage := storage.New(dbPool)
	realPaymentSimulator := func() bool {
		return rand.IntN(10) < 9
	}
	bookingService := service.New(bookingStorage, realPaymentSimulator)

	l, err := net.Listen("tcp", fmt.Sprintf(":%s", cfg.BookingGRPCPort))
	if err != nil {
		logger.Error("Failed to listen port", "error", err)
		os.Exit(1)
	}

	workerCtx, workerCancel := context.WithCancel(context.Background())
	defer workerCancel()

	outboxWorker := worker.NewOutboxWorker(dbPool, rabbitmqManager, logger, 10*time.Second)
	go outboxWorker.Start(workerCtx)

	go runExpirationWorker(workerCtx, rabbitmqManager, bookingService, logger)

	logger.Info("Booking Service ready. gRPC server listening", "address", l.Addr().String())

	healthSrv := health.NewServer()
	grpcSrv := grpc.NewServer(
		grpc.ChainUnaryInterceptor(
			otelgrpc.UnaryServerInterceptor(),
			interceptors.ServerRequestIDInterceptor(),
		),
	)
	grpcserver.Register(grpcSrv, bookingService)
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
		"x-message-ttl":          900000,
		"x-dead-letter-exchange": "bookings_dlx",
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

func runExpirationWorker(ctx context.Context, provider worker.ChannelProvider, bs *service.Booking, logger *slog.Logger) {
	logger.Info("Starting expiration worker")
	for {
		select {
		case <-ctx.Done():
			logger.Info("Expiration worker stopping...")
			return
		default:
		}
		ch, err := provider.GetChannel()
		if err != nil {
			logger.Error("Expiration worker: failed to get channel, retrying...", "error", err)
			time.Sleep(10 * time.Second)
			continue
		}

		msgs, err := ch.Consume(
			"bookings_expired_queue",
			"",
			false,
			false,
			false,
			false,
			nil,
		)
		if err != nil {
			logger.Error("Expiration worker: failed to start consumer, retrying...", "error", err)
			ch.Close()
			time.Sleep(10 * time.Second)
			continue
		}

		logger.Info("Expiration worker started. Waiting for messages...")

	processLoop:
		for {
			select {
			case <-ctx.Done():
				logger.Info("Expiration worker stopping...")
				ch.Close()
				return
			case d, ok := <-msgs:
				if !ok {
					logger.Warn("Expiration worker: message channel closed. Reconnecting...")
					ch.Close()
					break processLoop
				}

				logger.Info("Received an expired booking message", "body", string(d.Body))

				var msgBody map[string]int64
				if err := json.Unmarshal(d.Body, &msgBody); err != nil {
					logger.Error("Failed to unmarshal expiration message, discarding", "error", err)
					_ = d.Nack(false, false)
					continue
				}

				bookingID, ok := msgBody["booking_id"]
				if !ok {
					logger.Error("Invalid message format, discarding", "body", string(d.Body))
					_ = d.Nack(false, false)
					continue
				}

				opCtx, opCancel := context.WithTimeout(context.Background(), 1*time.Minute)
				if err := bs.CancelBooking(opCtx, bookingID); err != nil {
					logger.Error("Failed to process expired booking, retrying", "booking_id", bookingID, "error", err)
					_ = d.Nack(false, true)
					opCancel()
					continue
				}
				opCancel()

				logger.Info("Successfully cancelled expired booking", "booking_id", bookingID)
				if err := d.Ack(false); err != nil {
					logger.Error("Failed to acknowledge message", "error", err)
				}
			}
		}
	}
}
