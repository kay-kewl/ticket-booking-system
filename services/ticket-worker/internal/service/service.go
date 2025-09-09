package service

import (
    "context"
    "encoding/json"
    "fmt"
    "log/slog"
    "os"
    "time"

    "github.com/jung-kurt/gofpdf"
    "github.com/kay-kewl/ticket-booking-system/internal/rabbitmq"
    amqp "github.com/rabbitmq/amqp091-go"
)

type TicketService struct {
    outputPath  string
    logger      *slog.Logger
}

func New(outputPath string, logger *slog.Logger) *TicketService {
    return &TicketService{outputPath, logger}
}

func (s *TicketService) StartConsumer(ctx context.Context, rabbitManager *rabbitmq.ConnectionManager) {
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

    msgs, err := ch.Consume("ticket_queue", "", false, false, false, false, nil)
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
                s.logger.Warn("Message channel closed by RabbitMQ. Attempting to reconnect...")
                // TODO: цикл для повторного вызова StartConsumer
                return
            }

            if err := s.processMessage(msg); err != nil {
                s.logger.Error("Failed to process message, sending to DLQ", "error", err)
                msg.Nack(false, false)
            } else {
                msg.Ack(false)
            }
        }
    }
}

func (s *TicketService) processMessage(msg amqp.Delivery) error {
    var message struct {
        BookingID int64 `json:"booking_id"`
    }

    if err := json.Unmarshal(msg.Body, &message); err != nil {
        return fmt.Errorf("failed to unmarshal message: %w", err)
    }

    // TODO: grpc call for booking's details
    // bookingDetails, err := s.bookingClient.GetBookingDetails(context.Background(), message.BookingID)

    if err := os.MkdirAll(s.outputPath, os.ModePerm); err != nil {
        return fmt.Errorf("failed to create output directory: %w", err)
    }

    pdf := gofpdf.New("P", "mm", "A4", "")
    pdf.AddPage()
    pdf.SetFont("Arial", "B", 16)
    pdf.Cell(40, 10, "Your Ticket")
    pdf.Ln(20)
    pdf.SetFont("Arial", "", 12)
    pdf.Cell(40, 10, fmt.Sprintf("Booking ID: %d", message.BookingID))
    pdf.Ln(10)
    pdf.Cell(40, 10, fmt.Sprintf("Issued: %s", time.Now().Format("2000-01-02 03:04:05")))

    filename := fmt.Sprintf("%s/ticket_%d.pdf", s.outputPath, message.BookingID)
    if err := pdf.OutputFileAndClose(filename); err != nil {
        return fmt.Errorf("failed to save PDF file: %w", err)
    }

    s.logger.Info("Ticket generated successfully", "booking_id", message.BookingID, "file", filename)
    return nil
}

func (s *TicketService) setupTopology(ch *amqp.Channel) error {
    _, err := ch.QueueDeclare("ticket_dlq", true, false, false, false, nil)
    if err != nil {
        return err
    }

    args := amqp.Table{
        "x-dead-letter-exchange":       "",
        "x-dead-letter-routing-key":    "ticket_dlq",
    }

    _, err = ch.QueueDeclare("ticket_queue", true, false, false, false, args)
    if err != nil {
        return err
    }
    
    err = ch.QueueBind("ticket_queue", "booking.confirmed", "bookings_exchange", false, nil)
    return err
}
