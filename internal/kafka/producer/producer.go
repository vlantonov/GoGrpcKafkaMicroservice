package producer

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/IBM/sarama"
	"github.com/vlantonov/GoGrpcKafkaMicroservice/internal/domain"
)

// SaramaProducer publishes order events to Kafka using a synchronous producer.
type SaramaProducer struct {
	producer sarama.SyncProducer
	topic    string
}

// NewSaramaProducer creates a SaramaProducer connected to the given brokers.
func NewSaramaProducer(brokers []string, topic string) (*SaramaProducer, error) {
	cfg := sarama.NewConfig()
	cfg.Producer.RequiredAcks = sarama.WaitForAll
	cfg.Producer.Retry.Max = 5
	cfg.Producer.Return.Successes = true

	p, err := sarama.NewSyncProducer(brokers, cfg)
	if err != nil {
		return nil, fmt.Errorf("create sync producer: %w", err)
	}
	return &SaramaProducer{producer: p, topic: topic}, nil
}

// Publish JSON-encodes event and sends it to Kafka with the order ID as the key.
func (sp *SaramaProducer) Publish(_ context.Context, event *domain.OrderEvent) error {
	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}
	msg := &sarama.ProducerMessage{
		Topic: sp.topic,
		Key:   sarama.StringEncoder(event.OrderID),
		Value: sarama.ByteEncoder(payload),
	}
	_, _, err = sp.producer.SendMessage(msg)
	if err != nil {
		return fmt.Errorf("send kafka message: %w", err)
	}
	return nil
}

// NewSaramaProducerFromSync wraps an existing sarama.SyncProducer (used in tests).
func NewSaramaProducerFromSync(p sarama.SyncProducer, topic string) *SaramaProducer {
	return &SaramaProducer{producer: p, topic: topic}
}

// Close flushes and closes the underlying Sarama producer.
func (sp *SaramaProducer) Close() error {
	return sp.producer.Close()
}
