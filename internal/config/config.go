package config

import (
	"os"
	"fmt"
)

type Config struct {
	APIPort			string
	AuthGRPCPort	string
	BookingGRPCPort	string
	EventGRPCPort	string
	PostgresURL		string
}

func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}

	return defaultValue
}

func Load() (*Config, error) {
	postgresURL := getEnv("DATABASE_URL", "")
	if postgresURL == "" {
		postgresURL = fmt.Sprintf("postgres://%s:%s@postgres:%s/%s?sslmode=disable",
								  getEnv("POSTGRES_USER", "user"),
							   	  getEnv("POSTGRES_PASSWORD", "password"),
								  getEnv("POSTGRES_PORT", "5432"),
								  getEnv("POSTGRES_DB", "booking_db"),
		)
	}
	

	cfg := &Config{
		APIPort:			getEnv("API_PORT", "8080"),
		AuthGRPCPort:		getEnv("AUTH_GRPC_PORT", "50051"),
		BookingGRPCPort:	getEnv("BOOKING_GRPC_PORT", "50053"),
		EventGRPCPort:		getEnv("EVENT_GRPC_PORT", "50052"),
		PostgresURL:		postgresURL,
	}

	return cfg, nil
}