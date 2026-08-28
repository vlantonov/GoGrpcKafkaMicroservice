package handler_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	ordersv1 "github.com/vlantonov/GoGrpcKafkaMicroservice/gen/orders/v1"
	"github.com/vlantonov/GoGrpcKafkaMicroservice/internal/domain"
	grpchandler "github.com/vlantonov/GoGrpcKafkaMicroservice/internal/grpc/handler"
	"github.com/vlantonov/GoGrpcKafkaMicroservice/internal/store"

	"log/slog"
	"os"
)

// ── Fakes ────────────────────────────────────────────────────────────────────

type fakeStore struct {
	orders map[string]*domain.Order
	err    error
}

func newFakeStore() *fakeStore { return &fakeStore{orders: make(map[string]*domain.Order)} }

func (f *fakeStore) Create(_ context.Context, o *domain.Order) error {
	if f.err != nil {
		return f.err
	}
	cp := *o
	f.orders[o.ID] = &cp
	return nil
}

func (f *fakeStore) Get(_ context.Context, id string) (*domain.Order, error) {
	if f.err != nil {
		return nil, f.err
	}
	o, ok := f.orders[id]
	if !ok {
		return nil, store.ErrNotFound
	}
	cp := *o
	return &cp, nil
}

func (f *fakeStore) List(_ context.Context, filter domain.Status) ([]*domain.Order, error) {
	if f.err != nil {
		return nil, f.err
	}
	var result []*domain.Order
	for _, o := range f.orders {
		if filter == domain.StatusUnspecified || o.Status == filter {
			cp := *o
			result = append(result, &cp)
		}
	}
	return result, nil
}

func (f *fakeStore) UpdateStatus(_ context.Context, id string, newStatus domain.Status) (*domain.Order, error) {
	if f.err != nil {
		return nil, f.err
	}
	o, ok := f.orders[id]
	if !ok {
		return nil, store.ErrNotFound
	}
	if err := domain.ValidateTransition(o.Status, newStatus); err != nil {
		return nil, store.ErrInvalidStatus
	}
	o.Status = newStatus
	cp := *o
	return &cp, nil
}

type fakePublisher struct{ published []*domain.OrderEvent }

func (fp *fakePublisher) Publish(_ context.Context, e *domain.OrderEvent) error {
	fp.published = append(fp.published, e)
	return nil
}
func (fp *fakePublisher) Close() error { return nil }

// ── Helpers ──────────────────────────────────────────────────────────────────

func newServer(s grpchandler.OrderStore, p grpchandler.EventPublisher) *grpchandler.OrderServiceServer {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	return grpchandler.NewOrderServiceServer(s, p, logger)
}

func grpcCode(err error) codes.Code {
	return status.Code(err)
}

// ── Tests: CreateOrder ────────────────────────────────────────────────────────

func TestCreateOrder_HappyPath(t *testing.T) {
	t.Parallel()
	s := newFakeStore()
	p := &fakePublisher{}
	srv := newServer(s, p)

	resp, err := srv.CreateOrder(context.Background(), &ordersv1.CreateOrderRequest{Item: "widget", Quantity: 2})
	require.NoError(t, err)
	assert.NotEmpty(t, resp.Order.Id)
	assert.Equal(t, "widget", resp.Order.Item)
	assert.Equal(t, uint32(2), resp.Order.Quantity)
	assert.Equal(t, ordersv1.Status_STATUS_PENDING, resp.Order.Status)
}

func TestCreateOrder_ValidationError(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		req  *ordersv1.CreateOrderRequest
	}{
		{"empty item", &ordersv1.CreateOrderRequest{Item: "", Quantity: 1}},
		{"zero quantity", &ordersv1.CreateOrderRequest{Item: "x", Quantity: 0}},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			srv := newServer(newFakeStore(), &fakePublisher{})
			_, err := srv.CreateOrder(context.Background(), tc.req)
			assert.Equal(t, codes.InvalidArgument, grpcCode(err))
		})
	}
}

func TestCreateOrder_StoreError(t *testing.T) {
	t.Parallel()
	fs := newFakeStore()
	fs.err = assert.AnError
	srv := newServer(fs, &fakePublisher{})

	_, err := srv.CreateOrder(context.Background(), &ordersv1.CreateOrderRequest{Item: "x", Quantity: 1})
	assert.Equal(t, codes.Internal, grpcCode(err))
}

// ── Tests: GetOrder ───────────────────────────────────────────────────────────

func TestGetOrder_HappyPath(t *testing.T) {
	t.Parallel()
	fs := newFakeStore()
	o := domain.NewOrder("item", 1)
	_ = fs.Create(context.Background(), o)
	srv := newServer(fs, &fakePublisher{})

	resp, err := srv.GetOrder(context.Background(), &ordersv1.GetOrderRequest{Id: o.ID})
	require.NoError(t, err)
	assert.Equal(t, o.ID, resp.Order.Id)
}

func TestGetOrder_NotFound(t *testing.T) {
	t.Parallel()
	srv := newServer(newFakeStore(), &fakePublisher{})
	_, err := srv.GetOrder(context.Background(), &ordersv1.GetOrderRequest{Id: "nope"})
	assert.Equal(t, codes.NotFound, grpcCode(err))
}

// ── Tests: ListOrders ─────────────────────────────────────────────────────────

func TestListOrders_All(t *testing.T) {
	t.Parallel()
	fs := newFakeStore()
	_ = fs.Create(context.Background(), domain.NewOrder("a", 1))
	_ = fs.Create(context.Background(), domain.NewOrder("b", 2))
	srv := newServer(fs, &fakePublisher{})

	resp, err := srv.ListOrders(context.Background(), &ordersv1.ListOrdersRequest{})
	require.NoError(t, err)
	assert.Len(t, resp.Orders, 2)
}

func TestListOrders_Filtered(t *testing.T) {
	t.Parallel()
	fs := newFakeStore()
	_ = fs.Create(context.Background(), domain.NewOrder("a", 1))
	srv := newServer(fs, &fakePublisher{})

	resp, err := srv.ListOrders(context.Background(), &ordersv1.ListOrdersRequest{
		StatusFilter: ordersv1.Status_STATUS_CONFIRMED,
	})
	require.NoError(t, err)
	assert.Empty(t, resp.Orders)
}

// ── Tests: UpdateOrderStatus ──────────────────────────────────────────────────

func TestUpdateOrderStatus_HappyPath(t *testing.T) {
	t.Parallel()
	fs := newFakeStore()
	o := domain.NewOrder("item", 1)
	_ = fs.Create(context.Background(), o)
	srv := newServer(fs, &fakePublisher{})

	resp, err := srv.UpdateOrderStatus(context.Background(), &ordersv1.UpdateOrderStatusRequest{
		Id:        o.ID,
		NewStatus: ordersv1.Status_STATUS_CONFIRMED,
	})
	require.NoError(t, err)
	assert.Equal(t, ordersv1.Status_STATUS_CONFIRMED, resp.Order.Status)
}

func TestUpdateOrderStatus_NotFound(t *testing.T) {
	t.Parallel()
	srv := newServer(newFakeStore(), &fakePublisher{})
	_, err := srv.UpdateOrderStatus(context.Background(), &ordersv1.UpdateOrderStatusRequest{
		Id: "missing", NewStatus: ordersv1.Status_STATUS_CONFIRMED,
	})
	assert.Equal(t, codes.NotFound, grpcCode(err))
}

func TestUpdateOrderStatus_InvalidTransition(t *testing.T) {
	t.Parallel()
	fs := newFakeStore()
	o := domain.NewOrder("item", 1)
	_ = fs.Create(context.Background(), o)
	srv := newServer(fs, &fakePublisher{})

	_, err := srv.UpdateOrderStatus(context.Background(), &ordersv1.UpdateOrderStatusRequest{
		Id: o.ID, NewStatus: ordersv1.Status_STATUS_PENDING,
	})
	assert.Equal(t, codes.InvalidArgument, grpcCode(err))
}
