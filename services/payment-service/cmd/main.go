package main

import (
	"log/slog"
	"net/http"
	"os"

	"github.com/kay-kewl/ticket-booking-system/services/payment-service/internal/service"
)

func main() {
    logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

    webhookSecret := os.Getenv("PAYMENT_WEBHOOK_SECRET")
    if webhookSecret == "" {
        logger.Error("PAYMENT_WEBHOOK_SECRET is not set")
        os.Exit(1)
    }

    webhookTargetURL := os.Getenv("WEBHOOK_TARGET_URL")
    if webhookTargetURL == "" {
        logger.Error("WEBHOOK_TARGET_URL is not set")
        os.Exit(1)
    }

    paymentService := service.New(
        logger,
        webhookTargetURL,
        webhookSecret,
    )

    mux := http.NewServeMux()
    mux.HandleFunc("POST /v1/payments", paymentService.CreatePaymentHandler)

    port := "8081"
    logger.Info("Starting payment-service", "port", port)

    if err := http.ListenAndServe(":"+port, mux); err != nil {
        logger.Error("Server failed to start", "error", err)
    }
}

