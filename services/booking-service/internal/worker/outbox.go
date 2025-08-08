package worker

import (
	"context"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	amqp "github.com/rabbitmq/amqp091-go"
)

type ChannelProvider interface {
	GetChannel() (*amqp.Channel, error)
}

type OutboxWorker struct {
	db			*pgxpool.Pool
	provider 	ChannelProvider
	logger		*slog.Logger
	ticker		*time.Ticker
}

func NewOutboxWorker(db *pgxpool.Pool, provider ChannelProvider, logger *slog.Logger, interval time.Duration) *OutboxWorker {
	return &OutboxWorker{
		db:			db,
		provider:	provider,
		logger:		logger,
		ticker: 	time.NewTicker(interval),
	}
}

func (w *OutboxWorker) Start(ctx context.Context) {
	w.logger.Info("Starting Outbox Worker")
	for {
		select {
		case <-ctx.Done():
			w.logger.Info("Stopping Outbox Worker")
			w.ticker.Stop()
			return
		case <-w.ticker.C:
			w.processOutboxMessages(ctx)
		}
	}
}

func (w *OutboxWorker) processOutboxMessages(ctx context.Context) {
	const op = "worker.processOutboxMessages"
	log := w.logger.With(slog.String("op", op))

	ch, err := w.provider.GetChannel()
	if err != nil {
		log.Error("Failed to get RabbitMQ channel, skipping iteration", "error", err)
		return
	}
	defer ch.Close()

	tx, err := w.db.Begin(ctx)
	if err != nil {
		log.Error("Failed to begin transaction", "error", err)
		return
	}
	defer tx.Rollback(ctx)

	rows, err := tx.Query(
		ctx, `
		SELECT id, exchange, routing_key, payload FROM booking.outbox_messages
		WHERE processed_at IS NULL
		ORDER BY created_at
		LIMIT 10
		FOR UPDATE SKIP LOCKED 
		`,
	)
	if err != nil {
		log.Error("Failed to query outbox messages", "error", err)
		return
	}
	defer rows.Close()

	var messageIDs []int64
	for rows.Next() {
		var (
			id 			int64
			exchange	string
			routingKey	string
			payload		[]byte
		)

		if err := rows.Scan(&id, &exchange, &routingKey, &payload); err != nil {
			log.Error("Failed to scan outbox message", "error", err)
			continue
		}

		err = ch.PublishWithContext(
			ctx,
			exchange,
			routingKey,
			false,
			false,
			amqp.Publishing{
				ContentType: 	"application/json",
				Body:			payload,
			},
		)
		if err != nil {
			log.Error("Failed to publish message to RabbitMQ", "id", id, "error", err)
			continue
		}

		log.Info("Successfully published message", "id", id)
		messageIDs = append(messageIDs, id)
	}

	if len(messageIDs) == 0 {
		return
	}

	_, err = tx.Exec(
		ctx,
		"UPDATE booking.outbox_messages SET processed_at = NOW() WHERE id = ANY($1)",
		messageIDs,
	)
	if err != nil {
		log.Error("Failed to update outbox messages", "error", err)
		return
	}

	if err := tx.Commit(ctx); err != nil {
		log.Error("Failed to commit transaction", "error", err)
	}
}