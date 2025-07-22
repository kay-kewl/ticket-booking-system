package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	"github.com/kay-kewl/ticket-booking-system/internal/config"
	"github.com/kay-kewl/ticket-booking-system/internal/database"
	"github.com/kay-kewl/ticket-booking-system/internal/logging"
	grpcserver "github.com/kay-kewl/ticket-booking-system/services/event-service/internal/grpc"
	"github.com/kay-kewl/ticket-booking-system/services/event-service/internal/service"
	"github.com/kay-kewl/ticket-booking-system/services/event-service/internal/storage"
)

func main() {
	logger := logging.New()

	logger.Info("Event Service is starting up...")

	cfg, err := config.Load()
	if err != nil {
		logger.Error("Failed to load configuration", "error", err)
		os.Exit(1)
	}

	dbPool, err := database.NewConnection(context.Background(), cfg.PostgresURL, logger)
	if err != nil {
		logger.Error("Failed to connect to database", "error", err)
		os.Exit(1)
	}

	defer dbPool.Close()

	eventStorage := storage.New(dbPool)
	eventService := service.New(eventStorage)

	l, err := net.Listen("tcp", fmt.Sprintf(":%s", cfg.EventGRPCPort))
	if err != nil {
		logger.Error("Failed to listen port", "error", err)
		os.Exit(1)
	}

	logger.Info("Event Service ready. gRPC server listening", "address", l.Addr().String())

	grpcSrv := grpc.NewServer()

	grpcserver.Register(grpcSrv, eventService)

	reflection.Register(grpcSrv)

	go func() {
		if err := grpcSrv.Serve(l); err != nil {
			logger.Error("Failed to serve gRPC", "error", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	logger.Info("Shutting down gRPC server...")

	grpcSrv.GracefulStop()
	logger.Info("gRPC server stopped")
}