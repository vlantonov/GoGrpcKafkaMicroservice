package consumer

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/IBM/sarama"
)

// SaramaConsumer subscribes to a Kafka topic using a consumer group and logs each message.
type SaramaConsumer struct {
	group  sarama.ConsumerGroup
	topic  string
	logger *slog.Logger
}

// NewSaramaConsumer creates a SaramaConsumer connected to the given brokers.
func NewSaramaConsumer(brokers []string, groupID, topic string, logger *slog.Logger) (*SaramaConsumer, error) {
	cfg := sarama.NewConfig()
	cfg.Consumer.Offsets.Initial = sarama.OffsetOldest
	cfg.Consumer.Group.Rebalance.GroupStrategies = []sarama.BalanceStrategy{sarama.NewBalanceStrategyRoundRobin()}

	group, err := sarama.NewConsumerGroup(brokers, groupID, cfg)
	if err != nil {
		return nil, fmt.Errorf("create consumer group: %w", err)
	}
	return newFromGroup(group, topic, logger), nil
}

// NewSaramaConsumerFromGroup wraps an existing sarama.ConsumerGroup (used in tests).
func NewSaramaConsumerFromGroup(group sarama.ConsumerGroup, topic string, logger *slog.Logger) *SaramaConsumer {
	return newFromGroup(group, topic, logger)
}

func newFromGroup(group sarama.ConsumerGroup, topic string, logger *slog.Logger) *SaramaConsumer {
	return &SaramaConsumer{group: group, topic: topic, logger: logger}
}

// Start blocks, consuming messages until ctx is cancelled.
func (sc *SaramaConsumer) Start(ctx context.Context) error {
	handler := &groupHandler{logger: sc.logger}
	for {
		if err := sc.group.Consume(ctx, []string{sc.topic}, handler); err != nil {
			return fmt.Errorf("consumer group error: %w", err)
		}
		if ctx.Err() != nil {
			return nil
		}
	}
}

// Close closes the underlying consumer group, committing offsets.
func (sc *SaramaConsumer) Close() error {
	return sc.group.Close()
}

// groupHandler implements sarama.ConsumerGroupHandler.
type groupHandler struct {
	logger *slog.Logger
}

func (h *groupHandler) Setup(_ sarama.ConsumerGroupSession) error   { return nil }
func (h *groupHandler) Cleanup(_ sarama.ConsumerGroupSession) error { return nil }

func (h *groupHandler) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	for {
		select {
		case msg, ok := <-claim.Messages():
			if !ok {
				return nil
			}
			h.logger.Info("kafka message received",
				slog.String("topic", msg.Topic),
				slog.String("key", string(msg.Key)),
				slog.String("value", string(msg.Value)),
				slog.Int64("offset", msg.Offset),
			)
			session.MarkMessage(msg, "")
		case <-session.Context().Done():
			return nil
		}
	}
}
