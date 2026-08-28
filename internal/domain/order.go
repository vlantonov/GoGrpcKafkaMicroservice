package domain

import (
	"time"

	"github.com/google/uuid"
)

// Status represents the lifecycle state of an Order.
type Status int

const (
	StatusUnspecified Status = 0
	StatusPending     Status = 1
	StatusConfirmed   Status = 2
	StatusCancelled   Status = 3
)

// Order is the central domain entity.
type Order struct {
	ID        string
	Item      string
	Quantity  uint32
	Status    Status
	CreatedAt time.Time
}

// NewOrder creates a new Order with a generated UUID, StatusPending, and the current time.
func NewOrder(item string, quantity uint32) *Order {
	return &Order{
		ID:        uuid.New().String(),
		Item:      item,
		Quantity:  quantity,
		Status:    StatusPending,
		CreatedAt: time.Now().UTC(),
	}
}
