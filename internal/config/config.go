package config

import (
	"os"
	"fmt"
)

type Config struct {
	APIPort			string
	PostgresURL		string
}

func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}

	return defaultValue
}

func Load() (*Config, error) {
	postgresURL := fmt.Sprintf("postgres://%s:%s@postgres:%s/%s",
								getEnv("POSTGRES_USER", "user"),
								getEnv("POSTGRES_PASSWORD", "password"),
								getEnv("POSTGRES_PORT", "5432"),
								getEnv("POSTGRES_DB", "booking_db"),
	)

	cfg := &Config{
		APIPort:		getEnv("API_PORT", "8080"),
		PostgresURL:	postgresURL,
	}

	return cfg, nil
}