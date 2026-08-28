package config

import (
	"log/slog"
	"os"
	"strings"
)

// Config holds all runtime configuration read from environment variables.
type Config struct {
	GRPCPort     string
	KafkaBrokers []string
	KafkaTopic   string
	KafkaGroupID string
	LogLevel     slog.Level
	HealthPort   string
}

// Load reads environment variables and returns a Config with defaults applied.
func Load() Config {
	c := Config{
		GRPCPort:     getEnv("GRPC_PORT", "50051"),
		KafkaTopic:   getEnv("KAFKA_TOPIC", "orders"),
		KafkaGroupID: getEnv("KAFKA_GROUP_ID", "order-processor"),
		HealthPort:   getEnv("HEALTH_PORT", "8080"),
	}

	brokers := getEnv("KAFKA_BROKERS", "localhost:9092")
	c.KafkaBrokers = strings.Split(brokers, ",")

	levelStr := getEnv("LOG_LEVEL", "info")
	switch strings.ToLower(levelStr) {
	case "debug":
		c.LogLevel = slog.LevelDebug
	default:
		c.LogLevel = slog.LevelInfo
	}

	return c
}

func getEnv(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}
