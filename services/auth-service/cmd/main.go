package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"net"
	"syscall"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	"github.com/kay-kewl/ticket-booking-system/internal/config"
	"github.com/kay-kewl/ticket-booking-system/internal/database"
	"github.com/kay-kewl/ticket-booking-system/internal/logging"
	"github.com/kay-kewl/ticket-booking-system/services/auth-service/internal/service"
	"github.com/kay-kewl/ticket-booking-system/services/auth-service/internal/storage"
	grpcserver "github.com/kay-kewl/ticket-booking-system/services/auth-service/internal/grpc"
)

func main() {
	logger := logging.New()

	logger.Info("Auth Service is starting up...")

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

	authStorage := storage.New(dbPool)
	authService := service.New(authStorage, authStorage, 1 * time.Hour)

	l, err := net.Listen("tcp", fmt.Sprintf(":%s", cfg.GRPCPort))
	if err != nil {
		logger.Error("Failed to listen port", "error", err)
		os.Exit(1)
	}

	logger.Info("Auth Service ready. gRPC server listening", "address", l.Addr().String())

	grpcSrv := grpc.NewServer()

	grpcserver.Register(grpcSrv, authService)

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