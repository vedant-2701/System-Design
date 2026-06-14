// Package server implements the OrderService gRPC server.
package server

import (
	"context"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "grpc-demo/gen/order/protos"
)

// OrderServer implements pb.OrderServiceServer.
//
// In production, this delegates to a service layer that talks to a database.
// Here we use an in-memory store to keep the focus on gRPC mechanics.
type OrderServer struct {
	// pb.UnimplementedOrderServiceServer must be embedded.
	// It provides default implementations of all RPC methods so that
	// adding a new RPC to the .proto doesn't immediately break existing servers.
	// Any unimplemented method returns UNIMPLEMENTED — not a panic.
	pb.UnimplementedOrderServiceServer

	mu       sync.RWMutex          // guards orders and subscribers
	orders   map[uint32]*pb.Order  // in-memory store
	nextID   uint32
	// subscribers: each WatchOrders call registers a channel here.
	// When an order is created or updated, all subscribers receive the event.
	subscribers map[uint64]chan *pb.OrderEvent
	nextSubID   uint64

	// droppedEvents counts events dropped because a subscriber's buffer was full.
	// In production this would be a Prometheus counter.
	droppedEvents atomic.Uint64
}

// NewOrderServer constructs a server with some seed data.
func NewOrderServer() *OrderServer {
	s := &OrderServer{
		orders:      make(map[uint32]*pb.Order),
		subscribers: make(map[uint64]chan *pb.OrderEvent),
		nextID:      1,
	}
	// Seed an order so GetOrder immediately returns something useful.
	s.orders[0] = &pb.Order{
		OrderId: 0,
		Items: []*pb.Item{
			{Name: "Laptop", Quantity: 1, PriceCents: 99900},
		},
		Status:     pb.OrderStatus_ORDER_STATUS_CONFIRMED,
		CreatedAt:  timestamppb.Now(),
		TotalCents: 99900,
	}
	return s
}

// ─── Unary: GetOrder ──────────────────────────────────────────────────────────

// GetOrder returns a single order by ID.
//
// Error mapping:
//   - NOT_FOUND       → order does not exist
//   - INVALID_ARGUMENT → order_id is 0 (sentinel for "not provided" in proto3)
func (s *OrderServer) GetOrder(ctx context.Context, req *pb.GetOrderRequest) (*pb.Order, error) {
	// proto3 default for uint32 is 0. We treat 0 as "not provided".
	// In production, you'd use optional uint32 or a string UUID to avoid this.
	if req.OrderId == 0 && !s.hasOrderZero() {
		return nil, status.Error(codes.InvalidArgument, "order_id is required")
	}

	s.mu.RLock()
	order, ok := s.orders[req.OrderId]
	s.mu.RUnlock()

	if !ok {
		// NOT_FOUND is the correct code — not INTERNAL, not INVALID_ARGUMENT.
		// The client needs to know the resource doesn't exist so it can handle it
		// appropriately (e.g., don't retry, show 404 to the user).
		return nil, status.Errorf(codes.NotFound, "order %d not found", req.OrderId)
	}

	return order, nil
}

// ─── Unary: CreateOrder ───────────────────────────────────────────────────────

// CreateOrder creates a new order and notifies all active WatchOrders subscribers.
func (s *OrderServer) CreateOrder(ctx context.Context, req *pb.CreateOrderRequest) (*pb.CreateOrderResponse, error) {
	if len(req.Items) == 0 {
		return nil, status.Error(codes.InvalidArgument, "at least one item is required")
	}

	// Compute total
	var total uint32
	for _, item := range req.Items {
		if item.Quantity == 0 {
			return nil, status.Errorf(codes.InvalidArgument, "item %q has zero quantity", item.Name)
		}
		total += item.PriceCents * item.Quantity
	}

	now := timestamppb.Now()

	s.mu.Lock()
	id := s.nextID
	s.nextID++
	order := &pb.Order{
		OrderId:    id,
		Items:      req.Items,
		Status:     pb.OrderStatus_ORDER_STATUS_PENDING,
		CreatedAt:  now,
		TotalCents: total,
	}
	s.orders[id] = order
	s.mu.Unlock()

	slog.Info("order created", "order_id", id, "total_cents", total)

	// Notify all WatchOrders subscribers about the new order.
	// This is a fan-out: one write event goes to N subscribers.
	// Non-blocking send: if a subscriber's buffer is full, we skip it
	// rather than blocking CreateOrder. Slow subscribers don't back-pressure writers.
	event := &pb.OrderEvent{
		OrderId:    id,
		OldStatus:  pb.OrderStatus_ORDER_STATUS_UNKNOWN,
		NewStatus:  pb.OrderStatus_ORDER_STATUS_PENDING,
		OccurredAt: now,
	}
	s.fanOut(event)

	return &pb.CreateOrderResponse{
		OrderId:    id,
		TotalCents: total,
		CreatedAt:  now,
	}, nil
}

// ─── Server Streaming: WatchOrders ───────────────────────────────────────────

// WatchOrders streams OrderEvents to the client until:
//   - The client disconnects (ctx.Done())
//   - The server shuts down
//
// Each call to WatchOrders registers a subscription channel.
// When orders are created or updated, all subscribed streams receive the event.
//
// This demonstrates server streaming: one request, infinite responses.
func (s *OrderServer) WatchOrders(req *pb.WatchOrdersRequest, stream pb.OrderService_WatchOrdersServer) error {
	// Register this subscriber
	ch := make(chan *pb.OrderEvent, 32) // buffered: don't block the writer

	s.mu.Lock()
	subID := s.nextSubID
	s.nextSubID++
	s.subscribers[subID] = ch
	s.mu.Unlock()

	slog.Info("watcher connected", "sub_id", subID)

	// Ensure we clean up when the client disconnects or the stream ends.
	defer func() {
		s.mu.Lock()
		delete(s.subscribers, subID)
		close(ch)
		s.mu.Unlock()
		slog.Info("watcher disconnected", "sub_id", subID)
	}()

	// Send a heartbeat every 10 seconds so the client knows the stream is alive.
	// In production: use gRPC keepalive pings instead of application-level heartbeats.
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	ctx := stream.Context()

	for {
		select {
		case <-ctx.Done():
			// Client disconnected or deadline exceeded.
			// ctx.Err() is either context.Canceled or context.DeadlineExceeded.
			slog.Info("watcher context done", "sub_id", subID, "reason", ctx.Err())
			return nil // returning nil closes the stream cleanly

		case event, ok := <-ch:
			if !ok {
				// Channel closed — server shutting down
				return nil
			}
			if err := stream.Send(event); err != nil {
				// Send error usually means the client disconnected mid-stream.
				// Log and return — don't return a gRPC status error here because
				// the transport is already broken.
				slog.Warn("stream send failed", "sub_id", subID, "error", err)
				return err
			}

		case <-ticker.C:
			// Heartbeat: send a synthetic "keep-alive" event.
			// Alternative: rely on gRPC keepalive pings at the transport layer.
			heartbeat := &pb.OrderEvent{
				OrderId:    0,
				OccurredAt: timestamppb.Now(),
			}
			if err := stream.Send(heartbeat); err != nil {
				return err
			}
		}
	}
}

// ─── Internal helpers ─────────────────────────────────────────────────────────

// fanOut sends an event to all active subscribers.
// Non-blocking: slow subscribers are skipped, not back-pressured.
func (s *OrderServer) fanOut(event *pb.OrderEvent) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for id, ch := range s.subscribers {
		select {
		case ch <- event:
		default:
			// Subscriber buffer full. Skip rather than blocking.
			// In production: track dropped events as a metric and alert on it.
			s.droppedEvents.Add(1)
			slog.Warn("subscriber buffer full, dropping event", "sub_id", id)
		}
	}
}

// DroppedEventCount returns the number of events dropped due to full subscriber
// buffers. Exposed for tests and metrics endpoints.
func (s *OrderServer) DroppedEventCount() uint64 {
	return s.droppedEvents.Load()
}

// SubscriberCount returns the number of active WatchOrders subscribers.
// Exposed for tests.
func (s *OrderServer) SubscriberCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.subscribers)
}

func (s *OrderServer) hasOrderZero() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, ok := s.orders[0]
	return ok
}