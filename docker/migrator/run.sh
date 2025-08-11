#!/bin/sh
set -eu

until pg_isready -h postgres -p "$POSTGRES_PORT" -U "$POSTGRES_USER"; do
	echo "Waiting for Postgres..."
	sleep 5
done

psql "$DATABASE_URL" -c "CREATE SCHEMA IF NOT EXISTS auth;"
psql "$DATABASE_URL" -c "CREATE SCHEMA IF NOT EXISTS booking;"
psql "$DATABASE_URL" -c "CREATE SCHEMA IF NOT EXISTS event;"

echo "Running auth migrations..."
migrate -path /migrations/auth -database "${DATABASE_URL}?search_path=auth" up

echo "Running event migrations..."
migrate -path /migrations/event -database "${DATABASE_URL}?search_path=event" up

echo "Running booking migrations..."
migrate -path /migrations/booking -database "${DATABASE_URL}?search_path=booking" up

echo "All migrations applied successfully"