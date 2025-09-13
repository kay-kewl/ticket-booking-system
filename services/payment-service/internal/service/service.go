package service

import (
    "bytes"
    "crypto/hmac"
    "crypto/sha256"
    "encoding/hex"
    "encoding/json"
    "fmt"
    "io"
    "log/slog"
    "math/rand/v2"
    "net/http"
    "time"
)

type PaymentService struct {
    logger              *slog.Logger
    webhookTargetURL    string
    webhookSecret       []byte
}

func New(logger *slog.Logger, targetURL, secret string) *PaymentService {
    return &PaymentService{
        logger:             logger,
        webhookTargetURL:   targetURL,
        webhookSecret:      []byte(secret),
    }
}

type CreatePaymentRequest struct {
    BookingID   int64   `json:"booking_id"`
    Amount      float64 `json:"amount"`
}

type CreatePaymentResponse struct {
    BookingID   int64   `json:"booking_id"`
    Status      string  `json:"status"`
}

func (s *PaymentService) CreatePaymentHandler(w http.ResponseWriter, r *http.Request) {
    var req CreatePaymentRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "Invalid request body", http.StatusBadRequest)
        return
    }

    s.logger.Info("Payment creation request received", "booking_id", req.BookingID)

    go s.simulatePaymentAndSendWebhook(req.BookingID)

    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusAccepted)
    json.NewEncoder(w).Encode(CreatePaymentResponse{
        BookingID:  req.BookingID,
        Status:     "PENDING",
    })
}

func (s *PaymentService) simulatePaymentAndSendWebhook(bookingID int64) {
    time.Sleep(3 * time.Second)

    status := "CONFIRMED"
    if rand.IntN(10) >= 9 {
        status = "FAILED"
    }

    payload := map[string]interface{}{
        "booking_id":   bookingID,
        "status":       status,
        "timestamp":    time.Now().UTC().Format(time.RFC3339),
    }
    body, err := json.Marshal(payload)
    if err != nil {
        s.logger.Error("Failed to marshal webhook payload", "error", err)
        return
    }

    signature := s.calculateSignature(body)

    maxRetries := 3
    for i := 0; i < maxRetries; i++ {
        err = s.sendWebhook(body, signature)
        if err == nil {
            s.logger.Info("Webhook sent successfully", "booking_id", bookingID, "status", status)
            return
        }

        s.logger.Warn("Failed to send webhook, retrying...", "attempt", i + 1, "error", err)
        time.Sleep(2 * time.Second)
    }

    s.logger.Error("Failed to send webhook after all retries", "booking_id", bookingID)
}

func (s *PaymentService) sendWebhook(body []byte, signature string) error {
    req, err := http.NewRequest("POST", s.webhookTargetURL, bytes.NewBuffer(body))
    if err != nil {
        return err
    }
    req.Header.Set("Content-Type", "application/json")
    req.Header.Set("X-Webhook-Signature", signature)

    client := &http.Client{Timeout: 1 * time.Minute}
    resp, err := client.Do(req)
    if err != nil {
        return err
    }
    defer resp.Body.Close()

    if resp.StatusCode < 200 || resp.StatusCode >= 300 {
        respBody, _ := io.ReadAll(resp.Body)
        return fmt.Errorf("webhook target returned status %d: %s", resp.StatusCode, string(respBody))
    }

    return nil
}

func (s *PaymentService) calculateSignature(body []byte) string {
    mac := hmac.New(sha256.New, s.webhookSecret)
    mac.Write(body)
    return hex.EncodeToString(mac.Sum(nil))
}
