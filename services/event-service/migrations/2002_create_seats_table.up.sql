CREATE TYPE seat_status AS ENUM ('AVAILABLE', 'BOOKED', 'RESERVED');

CREATE TABLE IF NOT EXISTS seats (
    id BIGSERIAL PRIMARY KEY,
    
    event_id BIGINT NOT NULL REFERENCES events(id) ON DELETE CASCADE,

    seat_number VARCHAR(10) NOT NULL, -- например, "A12"
    row_number INT,
    sector VARCHAR(50),
    
    status seat_status NOT NULL DEFAULT 'AVAILABLE'
);

CREATE INDEX idx_seats_on_event_and_status ON seats (event_id, status);
CREATE UNIQUE INDEX idx_unique_seat_on_event ON seats (event_id, seat_number);
