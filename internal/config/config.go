package config

import (
	"os"
	"fmt"
)

type Config struct {
	APIPort			string
	AuthGRPCPort	string
	BookingGRPCPort	string
	DatabaseSchema	string
	EventGRPCPort	string
	PostgresURL		string
	RabbitMQURL		string
	JWTSecret		string
}

func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}

	return defaultValue
}

func Load() (*Config, error) {
	schema := getEnv("DATABASE_SCHEMA", "public")
	postgresURL := getEnv("DATABASE_URL", "")
	if postgresURL == "" {
		postgresURL = fmt.Sprintf("postgres://%s:%s@postgres:%s/%s?sslmode=disable&search_path=%s",
								  getEnv("POSTGRES_USER", "user"),
							   	  getEnv("POSTGRES_PASSWORD", "password"),
								  getEnv("POSTGRES_PORT", "5432"),
								  getEnv("POSTGRES_DB", "booking_db"),
								  schema,
		)
	}
	

	cfg := &Config{
		APIPort:			getEnv("API_PORT", "8080"),
		AuthGRPCPort:		getEnv("AUTH_GRPC_PORT", "50051"),
		BookingGRPCPort:	getEnv("BOOKING_GRPC_PORT", "50053"),
		DatabaseSchema:		schema,
		EventGRPCPort:		getEnv("EVENT_GRPC_PORT", "50052"),
		PostgresURL:		postgresURL,
		RabbitMQURL:		getEnv("RABBITMQ_URL", "amqp://guest:guest@rabbitmq:5672/"),
		JWTSecret:			getEnv("JWT_SECRET", "my-secret"),
	}

	return cfg, nil
}