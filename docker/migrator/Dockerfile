FROM alpine:latest

RUN apk add --no-cache ca-certificates curl postgresql-client \
  && curl -L https://github.com/golang-migrate/migrate/releases/download/v4.17.1/migrate.linux-amd64.tar.gz \
     | tar -xz && mv migrate.linux-amd64 /usr/bin/migrate

COPY services/auth-service/migrations /migrations/auth
COPY services/booking-service/migrations /migrations/booking
COPY services/event-service/migrations /migrations/event

COPY docker/migrator/run.sh /run.sh
RUN chmod +x /run.sh

ENTRYPOINT ["/run.sh"]