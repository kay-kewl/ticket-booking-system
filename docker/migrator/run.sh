#!/bin/sh
set -eu

append_param() {
	case "$1" in
		*\?*) 	echo "$1&$2" ;;
		*) 		echo "$1?$2" ;;
	esac
}

until pg_isready -h postgres -p 5432 -U "$POSTGRES_USER"; do
	echo "Waiting for Postgres..."
	sleep 5
done

psql "$DATABASE_URL" -c "CREATE SCHEMA IF NOT EXISTS auth;"
psql "$DATABASE_URL" -c "CREATE SCHEMA IF NOT EXISTS booking;"
psql "$DATABASE_URL" -c "CREATE SCHEMA IF NOT EXISTS event;"

AUTH_URL=$(append_param "$DATABASE_URL" "search_path=auth")
BOOKING_URL=$(append_param "$DATABASE_URL" "search_path=booking")
EVENT_URL=$(append_param "$DATABASE_URL" "search_path=event")

echo "Running auth migrations..."
migrate -path /migrations/auth -database "$AUTH_URL" up

echo "Running event migrations..."
migrate -path /migrations/event -database "$EVENT_URL" up

echo "Running booking migrations..."
migrate -path /migrations/booking -database "$BOOKING_URL" up

echo "All migrations applied successfully"