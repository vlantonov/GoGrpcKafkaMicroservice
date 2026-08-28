package consumer_test

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/IBM/sarama"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vlantonov/GoGrpcKafkaMicroservice/internal/kafka/consumer"
)

// stubConsumerGroup simulates a sarama.ConsumerGroup for testing.
type stubConsumerGroup struct {
	msgs   []*sarama.ConsumerMessage
	closed bool
}

func (g *stubConsumerGroup) Consume(ctx context.Context, _ []string, handler sarama.ConsumerGroupHandler) error {
	sess := &stubSession{ctx: ctx}
	claim := &stubClaim{msgs: g.msgs}
	if err := handler.Setup(sess); err != nil {
		return err
	}
	_ = handler.ConsumeClaim(sess, claim)
	_ = handler.Cleanup(sess)
	// Simulate session ending so caller can check ctx.
	return nil
}
func (g *stubConsumerGroup) Errors() <-chan error        { return make(chan error) }
func (g *stubConsumerGroup) Close() error                { g.closed = true; return nil }
func (g *stubConsumerGroup) Pause(_ map[string][]int32)  {}
func (g *stubConsumerGroup) Resume(_ map[string][]int32) {}
func (g *stubConsumerGroup) PauseAll()                   {}
func (g *stubConsumerGroup) ResumeAll()                  {}

type stubSession struct {
	ctx    context.Context
	marked []*sarama.ConsumerMessage
}

func (s *stubSession) Claims() map[string][]int32                       { return nil }
func (s *stubSession) MemberID() string                                 { return "test" }
func (s *stubSession) GenerationID() int32                              { return 1 }
func (s *stubSession) MarkOffset(_ string, _ int32, _ int64, _ string)  {}
func (s *stubSession) Commit()                                          {}
func (s *stubSession) ResetOffset(_ string, _ int32, _ int64, _ string) {}
func (s *stubSession) MarkMessage(msg *sarama.ConsumerMessage, _ string) {
	s.marked = append(s.marked, msg)
}
func (s *stubSession) Context() context.Context { return s.ctx }

type stubClaim struct {
	msgs []*sarama.ConsumerMessage
	ch   chan *sarama.ConsumerMessage
}

func (c *stubClaim) Topic() string              { return "orders" }
func (c *stubClaim) Partition() int32           { return 0 }
func (c *stubClaim) InitialOffset() int64       { return 0 }
func (c *stubClaim) HighWaterMarkOffset() int64 { return int64(len(c.msgs)) }
func (c *stubClaim) Messages() <-chan *sarama.ConsumerMessage {
	if c.ch == nil {
		c.ch = make(chan *sarama.ConsumerMessage, len(c.msgs))
		for _, m := range c.msgs {
			c.ch <- m
		}
		close(c.ch)
	}
	return c.ch
}

func TestSaramaConsumer_ConsumeMessage(t *testing.T) {
	t.Parallel()

	msgs := []*sarama.ConsumerMessage{
		{
			Topic: "orders",
			Key:   []byte("order-1"),
			Value: []byte(`{"event_type":"order.created","order_id":"order-1","status":"PENDING","timestamp":1724793600}`),
		},
	}
	grp := &stubConsumerGroup{msgs: msgs}
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	c := consumer.NewSaramaConsumerFromGroup(grp, "orders", logger)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Consume once; the stub returns after draining messages.
	// Then cancel ctx so Start loop exits.
	cancel() // cancel immediately after one cycle
	err := c.Start(ctx)
	assert.NoError(t, err)
}

func TestSaramaConsumer_Close(t *testing.T) {
	t.Parallel()

	grp := &stubConsumerGroup{}
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	c := consumer.NewSaramaConsumerFromGroup(grp, "orders", logger)

	require.NoError(t, c.Close())
	assert.True(t, grp.closed)
}
