FROM golang:1.24-alpine AS builder

ARG SERVICE
ARG PORT

WORKDIR /app

RUN apk --no-cache add curl && \
    curl -L https://github.com/golang-migrate/migrate/releases/download/v4.17.1/migrate.linux-amd64.tar.gz | tar xvz && \
    mv migrate /usr/bin/migrate

COPY go.mod go.sum ./
# COPY services/ ./services/
# COPY pkg/ ./pkg/

RUN go mod download

COPY internal ./internal
COPY gen ./gen

COPY services/${SERVICE} ./services/${SERVICE}

# RUN mkdir -p ./services/${SERVICE}/migrations

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -trimpath -o /app/server ./services/${SERVICE}/cmd/main.go

FROM alpine:latest

ARG SERVICE
ARG PORT

WORKDIR /app

COPY --from=builder /usr/bin/migrate /usr/bin/migrate
COPY --from=builder /app/server /app/server

# COPY --from=builder /app/services/${SERVICE}/migrations /app/migrations

RUN chmod +x /app/server

EXPOSE ${PORT}

ENTRYPOINT ["/app/server"]