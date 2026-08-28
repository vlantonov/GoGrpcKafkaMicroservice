package domain

// OrderEvent is the Kafka message payload published on every order state change.
type OrderEvent struct {
	EventType string `json:"event_type"`
	OrderID   string `json:"order_id"`
	Status    string `json:"status"`
	Timestamp int64  `json:"timestamp"`
}
