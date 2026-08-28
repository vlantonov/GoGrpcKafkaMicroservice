package store

import (
	"context"
	"errors"
	"sync"

	"github.com/vlantonov/GoGrpcKafkaMicroservice/internal/domain"
)

var (
	ErrNotFound      = errors.New("order not found")
	ErrAlreadyExists = errors.New("order already exists")
	ErrInvalidStatus = errors.New("invalid status transition")
)

// OrderStore is the persistence interface for orders.
type OrderStore interface {
	Create(ctx context.Context, order *domain.Order) error
	Get(ctx context.Context, id string) (*domain.Order, error)
	List(ctx context.Context, filter domain.Status) ([]*domain.Order, error)
	UpdateStatus(ctx context.Context, id string, newStatus domain.Status) (*domain.Order, error)
}

// MemoryStore is an in-memory, concurrency-safe implementation of OrderStore.
type MemoryStore struct {
	mu     sync.RWMutex
	orders map[string]*domain.Order
}

// NewMemoryStore creates an initialised MemoryStore.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{orders: make(map[string]*domain.Order)}
}

// Create stores a new order. Returns ErrAlreadyExists if the ID is taken.
func (s *MemoryStore) Create(_ context.Context, order *domain.Order) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.orders[order.ID]; ok {
		return ErrAlreadyExists
	}
	// Store a copy to prevent external mutation.
	cp := *order
	s.orders[order.ID] = &cp
	return nil
}

// Get retrieves an order by ID. Returns ErrNotFound if absent.
func (s *MemoryStore) Get(_ context.Context, id string) (*domain.Order, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	o, ok := s.orders[id]
	if !ok {
		return nil, ErrNotFound
	}
	cp := *o
	return &cp, nil
}

// List returns all orders, optionally filtered by status.
// A filter of StatusUnspecified (0) returns all orders.
func (s *MemoryStore) List(_ context.Context, filter domain.Status) ([]*domain.Order, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]*domain.Order, 0, len(s.orders))
	for _, o := range s.orders {
		if filter == domain.StatusUnspecified || o.Status == filter {
			cp := *o
			result = append(result, &cp)
		}
	}
	return result, nil
}

// UpdateStatus transitions an order to newStatus.
// Returns ErrNotFound or ErrInvalidStatus on failure.
func (s *MemoryStore) UpdateStatus(_ context.Context, id string, newStatus domain.Status) (*domain.Order, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	o, ok := s.orders[id]
	if !ok {
		return nil, ErrNotFound
	}
	if err := domain.ValidateTransition(o.Status, newStatus); err != nil {
		return nil, ErrInvalidStatus
	}
	o.Status = newStatus
	cp := *o
	return &cp, nil
}
