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

// blockingClaim has an open, never-sending channel so ConsumeClaim blocks until
// the session context is cancelled.
type blockingClaim struct{}

func (c *blockingClaim) Topic() string              { return "orders" }
func (c *blockingClaim) Partition() int32           { return 0 }
func (c *blockingClaim) InitialOffset() int64       { return 0 }
func (c *blockingClaim) HighWaterMarkOffset() int64 { return 0 }
func (c *blockingClaim) Messages() <-chan *sarama.ConsumerMessage {
	return make(chan *sarama.ConsumerMessage) // never closed, never receives
}

// cancellingGroup invokes ConsumeClaim with a blocking claim using the passed-in
// context so that the context.Done() branch is exercised.
type cancellingGroup struct {
	closed bool
}

func (g *cancellingGroup) Consume(ctx context.Context, _ []string, handler sarama.ConsumerGroupHandler) error {
	sess := &stubSession{ctx: ctx}
	if err := handler.Setup(sess); err != nil {
		return err
	}
	_ = handler.ConsumeClaim(sess, &blockingClaim{})
	_ = handler.Cleanup(sess)
	return nil
}
func (g *cancellingGroup) Errors() <-chan error        { return make(chan error) }
func (g *cancellingGroup) Close() error                { g.closed = true; return nil }
func (g *cancellingGroup) Pause(_ map[string][]int32)  {}
func (g *cancellingGroup) Resume(_ map[string][]int32) {}
func (g *cancellingGroup) PauseAll()                   {}
func (g *cancellingGroup) ResumeAll()                  {}

// errorGroup returns a non-nil error from Consume, exercising Start's error path.
type errorGroup struct{}

func (g *errorGroup) Consume(_ context.Context, _ []string, _ sarama.ConsumerGroupHandler) error {
	return assert.AnError
}
func (g *errorGroup) Errors() <-chan error        { return make(chan error) }
func (g *errorGroup) Close() error                { return nil }
func (g *errorGroup) Pause(_ map[string][]int32)  {}
func (g *errorGroup) Resume(_ map[string][]int32) {}
func (g *errorGroup) PauseAll()                   {}
func (g *errorGroup) ResumeAll()                  {}

// TestSaramaConsumer_ConsumeClaimContextDone covers the session.Context().Done()
// branch inside ConsumeClaim.
func TestSaramaConsumer_ConsumeClaimContextDone(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // pre-cancel so Done() fires immediately inside ConsumeClaim

	grp := &cancellingGroup{}
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	c := consumer.NewSaramaConsumerFromGroup(grp, "orders", logger)

	err := c.Start(ctx)
	assert.NoError(t, err)
}

// TestSaramaConsumer_ConsumeError covers the error-return branch in Start.
func TestSaramaConsumer_ConsumeError(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	grp := &errorGroup{}
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	c := consumer.NewSaramaConsumerFromGroup(grp, "orders", logger)

	err := c.Start(ctx)
	assert.Error(t, err)
}

// TestNewSaramaConsumer_NoBrokers covers the constructor's error path.
func TestNewSaramaConsumer_NoBrokers(t *testing.T) {
	t.Parallel()

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	c, err := consumer.NewSaramaConsumer([]string{}, "order-processor", "orders", logger)
	assert.Error(t, err)
	assert.Nil(t, c)
}

// messageConsumingGroup drives ConsumeClaim through the Messages() branch by
// cancelling the context only after ConsumeClaim returns, so the Done() case
// cannot win the select race.
type messageConsumingGroup struct {
	msgs   []*sarama.ConsumerMessage
	cancel context.CancelFunc
}

func (g *messageConsumingGroup) Consume(ctx context.Context, _ []string, handler sarama.ConsumerGroupHandler) error {
	sess := &stubSession{ctx: ctx}
	claim := &stubClaim{msgs: g.msgs}
	if err := handler.Setup(sess); err != nil {
		return err
	}
	_ = handler.ConsumeClaim(sess, claim)
	_ = handler.Cleanup(sess)
	g.cancel() // cancel ctx so Start's loop exits after this one Consume call
	return nil
}
func (g *messageConsumingGroup) Errors() <-chan error        { return make(chan error) }
func (g *messageConsumingGroup) Close() error                { return nil }
func (g *messageConsumingGroup) Pause(_ map[string][]int32)  {}
func (g *messageConsumingGroup) Resume(_ map[string][]int32) {}
func (g *messageConsumingGroup) PauseAll()                   {}
func (g *messageConsumingGroup) ResumeAll()                  {}

// TestSaramaConsumer_ConsumeClaimMessages covers the Messages() path in
// ConsumeClaim: ok=true (message processed) and ok=false (channel closed).
func TestSaramaConsumer_ConsumeClaimMessages(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())

	msgs := []*sarama.ConsumerMessage{
		{
			Topic: "orders",
			Key:   []byte("order-1"),
			Value: []byte(`{"event_type":"order.created","order_id":"order-1","status":"PENDING","timestamp":1724793600}`),
		},
	}
	grp := &messageConsumingGroup{msgs: msgs, cancel: cancel}
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	c := consumer.NewSaramaConsumerFromGroup(grp, "orders", logger)

	// ctx is NOT pre-cancelled; the group cancels it after ConsumeClaim returns.
	err := c.Start(ctx)
	assert.NoError(t, err)
}
