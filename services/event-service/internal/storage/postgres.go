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

func (s *Storage) ListEvents(ctx context.Context, pageNumber, pageSize int32) ([]*eventv1.Event, int64, error) {
	const op = "storage.ListEvents"

	var totalCount int64
	if err := s.db.QueryRow(ctx, "SELECT COUNT(*) FROM event.events").Scan(&totalCount); err != nil {
        return nil, 0, fmt.Errorf("%s: failed to count events: %w", op, err)
	}

	offset := (pageNumber - 1) * pageSize

	rows, err := s.db.Query(ctx, "SELECT id, title, description FROM event.events ORDER BY created_at DESC LIMIT $1 OFFSET $2", pageSize, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("%s: %w", op, err)
	}
	defer rows.Close()

	var events []*eventv1.Event
	for rows.Next() {
		var event eventv1.Event
		if err := rows.Scan(&event.Id, &event.Title, &event.Description); err != nil {
			return nil, 0, fmt.Errorf("%s: %w", op, err)
		}
		events = append(events, &event)
	}

	return events, totalCount, nil
}

func (s *Storage) GetEvent(ctx context.Context, eventID int64) (*eventv1.Event, error) {
    const op = "storage.GetEvent"

    var event eventv1.Event
    query := "SELECT id, title, description FROM event.events WHERE id = $1"
    err := s.db.QueryRow(ctx, query, eventID).Scan(&event.Id, &event.Title, &event.Description)
    if err != nil {
        // TODO: pgx.ErrNoRows -> "not found"
        return nil, fmt.Errorf("%s: %w", op, err)
    }

    return &event, nil
}
