package handler

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	ordersv1 "github.com/vlantonov/GoGrpcKafkaMicroservice/gen/orders/v1"
	"github.com/vlantonov/GoGrpcKafkaMicroservice/internal/domain"
	"github.com/vlantonov/GoGrpcKafkaMicroservice/internal/store"
)

// OrderStore is the persistence abstraction required by the gRPC handler.
type OrderStore interface {
	Create(ctx context.Context, order *domain.Order) error
	Get(ctx context.Context, id string) (*domain.Order, error)
	List(ctx context.Context, filter domain.Status) ([]*domain.Order, error)
	UpdateStatus(ctx context.Context, id string, newStatus domain.Status) (*domain.Order, error)
}

// EventPublisher is the messaging abstraction required by the gRPC handler.
type EventPublisher interface {
	Publish(ctx context.Context, event *domain.OrderEvent) error
	Close() error
}

// OrderServiceServer handles gRPC calls for the OrderService.
type OrderServiceServer struct {
	ordersv1.UnimplementedOrderServiceServer
	store     OrderStore
	publisher EventPublisher
	logger    *slog.Logger
}

// NewOrderServiceServer creates an OrderServiceServer.
func NewOrderServiceServer(s OrderStore, p EventPublisher, logger *slog.Logger) *OrderServiceServer {
	return &OrderServiceServer{store: s, publisher: p, logger: logger}
}

// CreateOrder validates the request, persists a new order, and publishes an event.
func (h *OrderServiceServer) CreateOrder(ctx context.Context, req *ordersv1.CreateOrderRequest) (*ordersv1.CreateOrderResponse, error) {
	if err := domain.ValidateCreate(req.GetItem(), req.GetQuantity()); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "%v", err)
	}
	o := domain.NewOrder(req.GetItem(), req.GetQuantity())
	if err := h.store.Create(ctx, o); err != nil {
		h.logger.ErrorContext(ctx, "store.Create failed", slog.Any("error", err))
		return nil, status.Error(codes.Internal, "internal error")
	}
	h.publishAsync(ctx, &domain.OrderEvent{
		EventType: "order.created",
		OrderID:   o.ID,
		Status:    statusName(o.Status),
		Timestamp: o.CreatedAt.Unix(),
	})
	return &ordersv1.CreateOrderResponse{Order: domainToProto(o)}, nil
}

// GetOrder returns a single order by ID.
func (h *OrderServiceServer) GetOrder(ctx context.Context, req *ordersv1.GetOrderRequest) (*ordersv1.GetOrderResponse, error) {
	o, err := h.store.Get(ctx, req.GetId())
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, status.Errorf(codes.NotFound, "order %s not found", req.GetId())
		}
		h.logger.ErrorContext(ctx, "store.Get failed", slog.Any("error", err))
		return nil, status.Error(codes.Internal, "internal error")
	}
	return &ordersv1.GetOrderResponse{Order: domainToProto(o)}, nil
}

// ListOrders returns orders, optionally filtered by status.
func (h *OrderServiceServer) ListOrders(ctx context.Context, req *ordersv1.ListOrdersRequest) (*ordersv1.ListOrdersResponse, error) {
	filter := protoStatusToDomain(req.GetStatusFilter())
	orders, err := h.store.List(ctx, filter)
	if err != nil {
		h.logger.ErrorContext(ctx, "store.List failed", slog.Any("error", err))
		return nil, status.Error(codes.Internal, "internal error")
	}
	resp := &ordersv1.ListOrdersResponse{Orders: make([]*ordersv1.Order, 0, len(orders))}
	for _, o := range orders {
		resp.Orders = append(resp.Orders, domainToProto(o))
	}
	return resp, nil
}

// UpdateOrderStatus transitions an order to the requested status and publishes an event.
func (h *OrderServiceServer) UpdateOrderStatus(ctx context.Context, req *ordersv1.UpdateOrderStatusRequest) (*ordersv1.UpdateOrderStatusResponse, error) {
	newStatus := protoStatusToDomain(req.GetNewStatus())
	o, err := h.store.UpdateStatus(ctx, req.GetId(), newStatus)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, status.Errorf(codes.NotFound, "order %s not found", req.GetId())
		}
		if errors.Is(err, store.ErrInvalidStatus) {
			return nil, status.Errorf(codes.InvalidArgument, "invalid status transition")
		}
		h.logger.ErrorContext(ctx, "store.UpdateStatus failed", slog.Any("error", err))
		return nil, status.Error(codes.Internal, "internal error")
	}
	h.publishAsync(ctx, &domain.OrderEvent{
		EventType: "order.status_updated",
		OrderID:   o.ID,
		Status:    statusName(o.Status),
		Timestamp: time.Now().Unix(),
	})
	return &ordersv1.UpdateOrderStatusResponse{Order: domainToProto(o)}, nil
}

// publishAsync fires the publish in a separate goroutine; errors are only logged.
func (h *OrderServiceServer) publishAsync(ctx context.Context, event *domain.OrderEvent) {
	go func() {
		if err := h.publisher.Publish(ctx, event); err != nil {
			h.logger.ErrorContext(ctx, "publish event failed", slog.Any("error", err))
		}
	}()
}

// domainToProto converts a domain Order to its proto representation.
func domainToProto(o *domain.Order) *ordersv1.Order {
	return &ordersv1.Order{
		Id:        o.ID,
		Item:      o.Item,
		Quantity:  o.Quantity,
		Status:    domainStatusToProto(o.Status),
		CreatedAt: o.CreatedAt.Unix(),
	}
}

func domainStatusToProto(s domain.Status) ordersv1.Status {
	switch s {
	case domain.StatusPending:
		return ordersv1.Status_STATUS_PENDING
	case domain.StatusConfirmed:
		return ordersv1.Status_STATUS_CONFIRMED
	case domain.StatusCancelled:
		return ordersv1.Status_STATUS_CANCELLED
	default:
		return ordersv1.Status_STATUS_UNSPECIFIED
	}
}

func protoStatusToDomain(s ordersv1.Status) domain.Status {
	switch s {
	case ordersv1.Status_STATUS_PENDING:
		return domain.StatusPending
	case ordersv1.Status_STATUS_CONFIRMED:
		return domain.StatusConfirmed
	case ordersv1.Status_STATUS_CANCELLED:
		return domain.StatusCancelled
	default:
		return domain.StatusUnspecified
	}
}

func statusName(s domain.Status) string {
	switch s {
	case domain.StatusPending:
		return "PENDING"
	case domain.StatusConfirmed:
		return "CONFIRMED"
	case domain.StatusCancelled:
		return "CANCELLED"
	default:
		return "UNSPECIFIED"
	}
}
