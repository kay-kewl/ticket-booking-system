# ---------- build stage ----------
FROM golang:1.24-alpine AS builder
WORKDIR /src

RUN apk add --no-cache ca-certificates curl

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -trimpath -o /build/server ./services/api-gateway/cmd

# ---------- runtime stage ----------
FROM alpine:latest
WORKDIR /app
RUN apk add --no-cache ca-certificates postgresql-client

COPY --from=builder /build/server /app/server
RUN mkdir /app/migrations
COPY entrypoint.sh  /usr/local/bin/entrypoint.sh
RUN chmod +x /usr/local/bin/entrypoint.sh /app/server

ENTRYPOINT ["entrypoint.sh"]