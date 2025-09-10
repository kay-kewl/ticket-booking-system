package service

import (
    "context"
    "encoding/json"
    "fmt"
    "log/slog"

    "github.com/kay-kewl/ticket-booking-system/internal/rabbitmq"
    amqp "github.com/kay-kewl/rabbitmq/amqp091-go"
)

type NotificationService struct {
    logger *slog.Logger
}

func New(logger *slog.Logger) *NotificationService {
    return &NotificationService{logger}
}

func (s *NotificationService) StartConsumer(ctx context.Context, rabbitManager *rabbitmq.ConnectionManager) {
    s.logger.Info("Waiting for RabbitMQ connection...")
    rabbitManager.WaitUntilReady()
    s.logger.Info("RabbitMQ connection is ready. Starting consumer...")

    ch, err := rabbitManager.GetChannel()
    if err != nil {
        s.logger.Error("Failed to get channel", "error", err)
        return
    }
    defer ch.Close()

    if err := s.setupTopology(ch); err != nil {
        s.logger.Error("Failed to setup RabbitMQ topology", "error", err)
        return
    }

    msgs, err := ch.Consume("notification_queue", "", false, false, false, false, nil)
    if err != nil {
        s.logger.Error("Failed to start consuming messages", "error", err)
        return
    }

    s.logger.Info("Consumer started. Waiting for messages...")
    for {
        select {
        case <-ctx.Done():
            s.logger.Info("Context cancelled, stopping consumer...")
            return
        case msg, ok := <-msgs:
            if !ok {
                s.logger.Warn("Message channel closed. Exiting consumer loop.")
                return
            }

            s.processMessage(msg)
            msg.Ack(false)
        }
    }
}

func (s *NotificationService) processMessage(msg amqp.Delivery) {
    var message struct {
        BookingID int64 `json:"booking_id"`
    }

    if err := json.Unmarshal(msg.Body, &message); err != nil {
        s.logger.Error("Failed to unmarshal message, discarding", "error", err)
        return
    }

    var notificationType string
    switch msg.RoutingKey {
    case "booking.confirmed":
        notificationType = "Booking Confirmed"
    case "booking.cancelled":
        notificationType = "Booking Cancelled"
    case "booking.expired":
        notificationType = "Booking Expired"
    default:
        notificationType = "Unknown Event"
    }

    s.logger.Info("Simulating sending notification", "type", notificationType, "booking_id", message.BookingID)
}

func (s *NotificationService) setupTopology(ch *amqp.Channel) error {
    _, err := ch.QueueDeclare("notification_dlq", true, false, false, false, nil)
    if err != nil {
        return fmt.Errorf("failed to declare dlq: %w", err)
    }

    args := amqp.Table{
        "x-dead-letter-exchange":       "",
        "x-dead-letter-routing-key":    "notification_dlq",
    }
    q, err := ch.QueueDeclare("notification_queue", true, false, false, false, args)
    if err != nil {
        return fmt.Errorf("failed to declare queue: %w", err)
    }

    s.logger.Info("Binding queue to routing keys...")
    eventsToBind := []string{
        "booking.confirmed",
        "booking.cancelled",
        "booking.expired",
    }

    for _, eventKey := range eventsToBind {
        err = ch.QueueBind(q.Name, eventKey, "bookings_exchange", false, nil)
        if err != nil {
            return fmt.Errorf("failed to bind queue to key %s: %w", eventKey, err)
        }
    }

    return nil
}
