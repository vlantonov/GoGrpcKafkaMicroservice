package producer_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/IBM/sarama/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vlantonov/GoGrpcKafkaMicroservice/internal/domain"
	"github.com/vlantonov/GoGrpcKafkaMicroservice/internal/kafka/producer"
)

func TestSaramaProducer_Publish(t *testing.T) {
	t.Parallel()

	mockProducer := mocks.NewSyncProducer(t, nil)
	const topic = "orders"
	event := &domain.OrderEvent{
		EventType: "order.created",
		OrderID:   "abc-123",
		Status:    "PENDING",
		Timestamp: 1724793600,
	}

	// Expect one message; validate key and value.
	mockProducer.ExpectSendMessageWithCheckerFunctionAndSucceed(func(msg []byte) error {
		var got domain.OrderEvent
		if err := json.Unmarshal(msg, &got); err != nil {
			return err
		}
		assert.Equal(t, event.EventType, got.EventType)
		assert.Equal(t, event.OrderID, got.OrderID)
		assert.Equal(t, event.Status, got.Status)
		assert.Equal(t, event.Timestamp, got.Timestamp)
		return nil
	})

	p := producer.NewSaramaProducerFromSync(mockProducer, topic)
	err := p.Publish(context.Background(), event)
	require.NoError(t, err)

	require.NoError(t, mockProducer.Close())
}

func TestSaramaProducer_PublishError(t *testing.T) {
	t.Parallel()

	mockProducer := mocks.NewSyncProducer(t, nil)
	mockProducer.ExpectSendMessageAndFail(assert.AnError)

	p := producer.NewSaramaProducerFromSync(mockProducer, "orders")
	err := p.Publish(context.Background(), &domain.OrderEvent{OrderID: "x"})
	assert.Error(t, err)

	require.NoError(t, mockProducer.Close())
}
