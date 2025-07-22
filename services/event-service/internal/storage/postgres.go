package storage

import (
	"context"
	"fmt"

	eventv1 "github.com/kay-kewl/ticket-booking-system/gen/go/event"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Storage struct {
	db *pgxpool.Pool
}

func New(db *pgxpool.Pool) *Storage {
	return &Storage{db: db}
}

func (s *Storage) ListEvents(ctx context.Context) ([]*eventv1.Event, error) {
	const op = "storage.ListEvents"

	rows, err := s.db.Query(ctx, "SELECT id, title, description FROM events ORDER BY created_at DESC")
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}
	defer rows.Close()

	var events []*eventv1.Event
	for rows.Next() {
		var event eventv1.Event
		if err := rows.Scan(&event.Id, &event.Title, &event.Description); err != nil {
			return nil, fmt.Errorf("%s: %w", op, err)
		}
		events = append(events, &event)
	}

	return events, nil
}