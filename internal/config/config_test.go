package config_test

import (
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/vlantonov/GoGrpcKafkaMicroservice/internal/config"
)

func TestLoad_Defaults(t *testing.T) {
	t.Setenv("GRPC_PORT", "")
	t.Setenv("KAFKA_BROKERS", "")
	t.Setenv("KAFKA_TOPIC", "")
	t.Setenv("KAFKA_GROUP_ID", "")
	t.Setenv("LOG_LEVEL", "")
	t.Setenv("HEALTH_PORT", "")

	c := config.Load()

	assert.Equal(t, "50051", c.GRPCPort)
	assert.Equal(t, []string{"localhost:9092"}, c.KafkaBrokers)
	assert.Equal(t, "orders", c.KafkaTopic)
	assert.Equal(t, "order-processor", c.KafkaGroupID)
	assert.Equal(t, "8080", c.HealthPort)
	assert.Equal(t, slog.LevelInfo, c.LogLevel)
}

func TestLoad_EnvOverrides(t *testing.T) {
	t.Setenv("GRPC_PORT", "50052")
	t.Setenv("KAFKA_BROKERS", "broker1:9092,broker2:9092")
	t.Setenv("KAFKA_TOPIC", "my-topic")
	t.Setenv("KAFKA_GROUP_ID", "my-group")
	t.Setenv("LOG_LEVEL", "debug")
	t.Setenv("HEALTH_PORT", "9090")

	c := config.Load()

	assert.Equal(t, "50052", c.GRPCPort)
	assert.Equal(t, []string{"broker1:9092", "broker2:9092"}, c.KafkaBrokers)
	assert.Equal(t, "my-topic", c.KafkaTopic)
	assert.Equal(t, "my-group", c.KafkaGroupID)
	assert.Equal(t, "9090", c.HealthPort)
	assert.Equal(t, slog.LevelDebug, c.LogLevel)
}

func TestLoad_SingleBroker(t *testing.T) {
	t.Setenv("KAFKA_BROKERS", "kafka:9092")

	c := config.Load()

	assert.Equal(t, []string{"kafka:9092"}, c.KafkaBrokers)
}
