CREATE TABLE booking.outbox_messages (
	id BIGSERIAL PRIMARY KEY,
	exchange TEXT NOT NULL,
	routing_key TEXT NOT NULL,
	payload JSONB NOT NULL,
	created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	processed_at TIMESTAMPTZ
);

CREATE INDEX idx_outbox_unprocessed ON booking.outbox_messages (processed_at) WHERE processed_at IS NULL;