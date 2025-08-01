# ---------- build stage ----------
FROM golang:1.24-alpine AS builder
WORKDIR /src

RUN apk add --no-cache ca-certificates curl \
  && curl -L https://github.com/golang-migrate/migrate/releases/download/v4.17.1/migrate.linux-amd64.tar.gz \
     | tar -xz && mv migrate /usr/bin/migrate

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -trimpath -o /build/server ./services/auth-service/cmd

# ---------- runtime stage ----------
FROM alpine:latest
WORKDIR /app

RUN apk add --no-cache ca-certificates postgresql-client

COPY --from=builder /usr/bin/migrate    /usr/local/bin/migrate
COPY --from=builder /build/server       /app/server
COPY services/auth-service/migrations   /app/migrations
COPY entrypoint.sh                      /usr/local/bin/entrypoint.sh
RUN chmod +x /usr/local/bin/entrypoint.sh /app/server

ENTRYPOINT ["entrypoint.sh"]