#!/bin/sh
set -eu

DB_SCHEMA="${DATABASE_SCHEMA:-public}"

echo "Waiting for Postgres at postgres:${POSTGRES_PORT}..."
until nc -z "postgres" "$POSTGRES_PORT"; do
  sleep 1
done
echo "Postgres is up"

echo "Ensuring schema ${DB_SCHEMA}"
psql "postgres://${POSTGRES_USER}:${POSTGRES_PASSWORD}@postgres:${POSTGRES_PORT}/${POSTGRES_DB}?sslmode=disable" \
     -c "CREATE SCHEMA IF NOT EXISTS ${DB_SCHEMA};"

if find /app/migrations -maxdepth 1 -name '*.sql' | read _; then
  echo "Running migrations..."
  migrate -path /app/migrations \
          -database "postgres://${POSTGRES_USER}:${POSTGRES_PASSWORD}@postgres:${POSTGRES_PORT}/${POSTGRES_DB}?sslmode=disable&search_path=${DB_SCHEMA}" \
          up
else
  echo "No migrations to run"
fi

echo "Starting server..."
exec /app/server