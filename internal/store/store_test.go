package store_test

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vlantonov/GoGrpcKafkaMicroservice/internal/domain"
	"github.com/vlantonov/GoGrpcKafkaMicroservice/internal/store"
)

func newOrder(item string, qty uint32) *domain.Order {
	return domain.NewOrder(item, qty)
}

func TestCreate(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		order   *domain.Order
		wantErr error
	}{
		{name: "happy path", order: newOrder("widget", 3), wantErr: nil},
		{name: "duplicate", order: func() *domain.Order {
			o := newOrder("dup", 1)
			o.ID = "fixed-id"
			return o
		}(), wantErr: store.ErrAlreadyExists},
	}

	s := store.NewMemoryStore()
	ctx := context.Background()

	// Seed a known ID for the duplicate test.
	seed := newOrder("dup", 1)
	seed.ID = "fixed-id"
	require.NoError(t, s.Create(ctx, seed))

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := s.Create(ctx, tc.order)
			if tc.wantErr != nil {
				assert.ErrorIs(t, err, tc.wantErr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestGet(t *testing.T) {
	t.Parallel()
	s := store.NewMemoryStore()
	ctx := context.Background()

	o := newOrder("apple", 2)
	require.NoError(t, s.Create(ctx, o))

	tests := []struct {
		name    string
		id      string
		wantErr error
	}{
		{name: "found", id: o.ID, wantErr: nil},
		{name: "not found", id: "missing", wantErr: store.ErrNotFound},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := s.Get(ctx, tc.id)
			if tc.wantErr != nil {
				assert.ErrorIs(t, err, tc.wantErr)
			} else {
				require.NoError(t, err)
				assert.Equal(t, o.ID, got.ID)
			}
		})
	}
}

func TestList(t *testing.T) {
	t.Parallel()
	s := store.NewMemoryStore()
	ctx := context.Background()

	o1 := newOrder("a", 1)
	o2 := newOrder("b", 2)
	require.NoError(t, s.Create(ctx, o1))
	require.NoError(t, s.Create(ctx, o2))

	t.Run("all orders", func(t *testing.T) {
		t.Parallel()
		list, err := s.List(ctx, domain.StatusUnspecified)
		require.NoError(t, err)
		assert.Len(t, list, 2)
	})
	t.Run("filter pending", func(t *testing.T) {
		t.Parallel()
		list, err := s.List(ctx, domain.StatusPending)
		require.NoError(t, err)
		assert.Len(t, list, 2)
	})
	t.Run("filter confirmed returns empty", func(t *testing.T) {
		t.Parallel()
		list, err := s.List(ctx, domain.StatusConfirmed)
		require.NoError(t, err)
		assert.Empty(t, list)
	})
}

func TestUpdateStatus(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("pending to confirmed", func(t *testing.T) {
		t.Parallel()
		s := store.NewMemoryStore()
		o := newOrder("x", 1)
		require.NoError(t, s.Create(ctx, o))
		updated, err := s.UpdateStatus(ctx, o.ID, domain.StatusConfirmed)
		require.NoError(t, err)
		assert.Equal(t, domain.StatusConfirmed, updated.Status)
	})
	t.Run("pending to cancelled", func(t *testing.T) {
		t.Parallel()
		s := store.NewMemoryStore()
		o := newOrder("y", 1)
		require.NoError(t, s.Create(ctx, o))
		_, err := s.UpdateStatus(ctx, o.ID, domain.StatusCancelled)
		require.NoError(t, err)
	})
	t.Run("not found", func(t *testing.T) {
		t.Parallel()
		s := store.NewMemoryStore()
		_, err := s.UpdateStatus(ctx, "no-such", domain.StatusConfirmed)
		assert.ErrorIs(t, err, store.ErrNotFound)
	})
	t.Run("invalid transition", func(t *testing.T) {
		t.Parallel()
		s := store.NewMemoryStore()
		o := newOrder("z", 1)
		require.NoError(t, s.Create(ctx, o))
		// Cannot go Pending → Pending.
		_, err := s.UpdateStatus(ctx, o.ID, domain.StatusPending)
		assert.ErrorIs(t, err, store.ErrInvalidStatus)
	})
}

func TestConcurrentWrites(t *testing.T) {
	t.Parallel()
	s := store.NewMemoryStore()
	ctx := context.Background()

	const n = 50
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			o := newOrder("concurrent", 1)
			_ = s.Create(ctx, o)
		}()
	}
	wg.Wait()

	list, err := s.List(ctx, domain.StatusUnspecified)
	require.NoError(t, err)
	assert.Len(t, list, n)
}
