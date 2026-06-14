package server

import (
	"context"
	"sync"
	"testing"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "grpc-demo/gen/order/proto"
)

// ─── Unit Tests: GetOrder ───────────────────────────────────────────────────

func TestGetOrder_Success(t *testing.T) {
	s := NewOrderServer()

	order, err := s.GetOrder(context.Background(), &pb.GetOrderRequest{OrderId: 0})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if order.OrderId != 0 {
		t.Errorf("expected order_id 0, got %d", order.OrderId)
	}
	if order.Status != pb.OrderStatus_ORDER_STATUS_CONFIRMED {
		t.Errorf("expected CONFIRMED status, got %v", order.Status)
	}
}

func TestGetOrder_NotFound(t *testing.T) {
	s := NewOrderServer()

	_, err := s.GetOrder(context.Background(), &pb.GetOrderRequest{OrderId: 9999})
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	// Verify the EXACT status code — not just "an error occurred".
	// A test that only checks err != nil would pass even if we returned
	// codes.Internal, which would be a production bug (client can't distinguish
	// "doesn't exist" from "server broke").
	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("expected gRPC status error, got %T", err)
	}
	if st.Code() != codes.NotFound {
		t.Errorf("expected NotFound, got %v", st.Code())
	}
}

// ─── Unit Tests: CreateOrder ────────────────────────────────────────────────

func TestCreateOrder_Success(t *testing.T) {
	s := NewOrderServer()

	resp, err := s.CreateOrder(context.Background(), &pb.CreateOrderRequest{
		Items: []*pb.Item{
			{Name: "Widget", Quantity: 3, PriceCents: 1000},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.TotalCents != 3000 {
		t.Errorf("expected total 3000, got %d", resp.TotalCents)
	}

	// Verify the order is retrievable afterwards — tests the write actually
	// landed in the store, not just that the response looked right.
	order, err := s.GetOrder(context.Background(), &pb.GetOrderRequest{OrderId: resp.OrderId})
	if err != nil {
		t.Fatalf("created order not retrievable: %v", err)
	}
	if order.Status != pb.OrderStatus_ORDER_STATUS_PENDING {
		t.Errorf("expected PENDING status, got %v", order.Status)
	}
}

func TestCreateOrder_EmptyItems(t *testing.T) {
	s := NewOrderServer()

	_, err := s.CreateOrder(context.Background(), &pb.CreateOrderRequest{Items: nil})
	requireCode(t, err, codes.InvalidArgument)
}

func TestCreateOrder_ZeroQuantity(t *testing.T) {
	s := NewOrderServer()

	_, err := s.CreateOrder(context.Background(), &pb.CreateOrderRequest{
		Items: []*pb.Item{{Name: "Bad", Quantity: 0, PriceCents: 100}},
	})
	requireCode(t, err, codes.InvalidArgument)
}

// ─── Concurrency Tests: CreateOrder under contention ────────────────────────

// TestCreateOrder_ConcurrentIDsAreUnique verifies that concurrent CreateOrder
// calls never produce duplicate order IDs.
//
// This is the single most important concurrency property of the server:
// nextID++ under sync.Mutex must serialize correctly. If this test fails
// intermittently, it indicates a race on nextID — run with -race to confirm.
func TestCreateOrder_ConcurrentIDsAreUnique(t *testing.T) {
	s := NewOrderServer()

	const goroutines = 100
	var wg sync.WaitGroup
	ids := make([]uint32, goroutines)
	errs := make([]error, goroutines)

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			resp, err := s.CreateOrder(context.Background(), &pb.CreateOrderRequest{
				Items: []*pb.Item{{Name: "Item", Quantity: 1, PriceCents: 100}},
			})
			if err != nil {
				errs[idx] = err
				return
			}
			ids[idx] = resp.OrderId
		}(i)
	}
	wg.Wait()

	seen := make(map[uint32]bool)
	for i, err := range errs {
		if err != nil {
			t.Fatalf("goroutine %d failed: %v", i, err)
		}
		if seen[ids[i]] {
			t.Fatalf("duplicate order ID generated: %d", ids[i])
		}
		seen[ids[i]] = true
	}

	if len(seen) != goroutines {
		t.Errorf("expected %d unique IDs, got %d", goroutines, len(seen))
	}
}

// ─── Concurrency Tests: WatchOrders fan-out ──────────────────────────────────

// TestWatchOrders_FanOutToMultipleSubscribers verifies that a single
// CreateOrder call delivers an event to ALL active subscribers — not just one.
//
// This is the correctness property of fanOut(): it must iterate every
// subscriber, not pick one at random.
func TestWatchOrders_FanOutToMultipleSubscribers(t *testing.T) {
	s := NewOrderServer()

	const numSubscribers = 5
	received := make([]chan *pb.OrderEvent, numSubscribers)
	streams := make([]*fakeWatchStream, numSubscribers)

	var wg sync.WaitGroup
	for i := 0; i < numSubscribers; i++ {
		received[i] = make(chan *pb.OrderEvent, 1)
		ctx, cancel := context.WithCancel(context.Background())
		streams[i] = newFakeWatchStream(ctx, received[i])

		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			_ = s.WatchOrders(&pb.WatchOrdersRequest{}, streams[idx])
		}(i)
		defer cancel()
	}

	// Give WatchOrders goroutines time to register as subscribers.
	waitForSubscriberCount(t, s, numSubscribers, 2*time.Second)

	_, err := s.CreateOrder(context.Background(), &pb.CreateOrderRequest{
		Items: []*pb.Item{{Name: "Broadcast Test", Quantity: 1, PriceCents: 500}},
	})
	if err != nil {
		t.Fatalf("CreateOrder failed: %v", err)
	}

	// Every subscriber should receive the event.
	for i := 0; i < numSubscribers; i++ {
		select {
		case event := <-received[i]:
			if event.NewStatus != pb.OrderStatus_ORDER_STATUS_PENDING {
				t.Errorf("subscriber %d: expected PENDING, got %v", i, event.NewStatus)
			}
		case <-time.After(2 * time.Second):
			t.Errorf("subscriber %d: did not receive event within timeout", i)
		}
	}
}

// TestWatchOrders_SlowSubscriberDoesNotBlockCreateOrder is the test that
// directly validates the design decision discussed in the session:
// a slow/stuck subscriber must not block CreateOrder.
//
// We register one subscriber that NEVER reads from its stream (simulating
// a stuck client), fill its buffer past capacity, then verify CreateOrder
// still returns promptly and the event was dropped (not blocked on).
func TestWatchOrders_SlowSubscriberDoesNotBlockCreateOrder(t *testing.T) {
	s := NewOrderServer()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// received has capacity 0 — the fake stream's Send will never be drained
	// by this test, simulating a client that stopped reading.
	received := make(chan *pb.OrderEvent) // unbuffered, never read from
	stream := newFakeWatchStream(ctx, received)

	go func() { _ = s.WatchOrders(&pb.WatchOrdersRequest{}, stream) }()
	waitForSubscriberCount(t, s, 1, 2*time.Second)

	// Fill the subscriber's internal channel (capacity 32) past its limit.
	// CreateOrder must not block on any of these — each call must return
	// within the test timeout.
	const numOrders = 40 // > 32 buffer capacity
	for i := 0; i < numOrders; i++ {
		done := make(chan struct{})
		go func() {
			_, err := s.CreateOrder(context.Background(), &pb.CreateOrderRequest{
				Items: []*pb.Item{{Name: "Flood", Quantity: 1, PriceCents: 100}},
			})
			if err != nil {
				t.Errorf("CreateOrder %d failed: %v", i, err)
			}
			close(done)
		}()

		select {
		case <-done:
			// good — did not block
		case <-time.After(500 * time.Millisecond):
			t.Fatalf("CreateOrder %d blocked — slow subscriber backpressure leaked into writer path", i)
		}
	}

	// Some events should have been dropped, proving the buffer filled up
	// and the non-blocking path was exercised (not just luck).
	if s.DroppedEventCount() == 0 {
		t.Error("expected at least one dropped event, got 0 — test may not be exercising the full-buffer path")
	}
}

// TestWatchOrders_ContextCancellationCleansUpSubscriber verifies that when
// a client disconnects (context cancelled), the subscriber is removed from
// the server's subscriber map. Without this, disconnected clients would
// accumulate as a memory leak.
func TestWatchOrders_ContextCancellationCleansUpSubscriber(t *testing.T) {
	s := NewOrderServer()

	ctx, cancel := context.WithCancel(context.Background())
	received := make(chan *pb.OrderEvent, 1)
	stream := newFakeWatchStream(ctx, received)

	done := make(chan struct{})
	go func() {
		_ = s.WatchOrders(&pb.WatchOrdersRequest{}, stream)
		close(done)
	}()

	waitForSubscriberCount(t, s, 1, 2*time.Second)

	cancel() // simulate client disconnect

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("WatchOrders did not return after context cancellation")
	}

	if s.SubscriberCount() != 0 {
		t.Errorf("expected 0 subscribers after disconnect, got %d", s.SubscriberCount())
	}
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

func requireCode(t *testing.T, err error, want codes.Code) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected error with code %v, got nil", want)
	}
	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("expected gRPC status error, got %T: %v", err, err)
	}
	if st.Code() != want {
		t.Errorf("expected code %v, got %v (%v)", want, st.Code(), err)
	}
}

// tHelper is satisfied by both *testing.T and *testing.B.
type tHelper interface {
	Helper()
	Fatalf(format string, args ...any)
}

func waitForSubscriberCount(t tHelper, s *OrderServer, want int, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if s.SubscriberCount() == want {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for subscriber count %d, got %d", want, s.SubscriberCount())
}

// fakeWatchStream implements pb.OrderService_WatchOrdersServer for testing,
// without requiring a real gRPC connection.
type fakeWatchStream struct {
	ctx      context.Context
	sendCh   chan *pb.OrderEvent
	pb.OrderService_WatchOrdersServer // embed for forward-compat with unused methods
}

func newFakeWatchStream(ctx context.Context, sendCh chan *pb.OrderEvent) *fakeWatchStream {
	return &fakeWatchStream{ctx: ctx, sendCh: sendCh}
}

func (f *fakeWatchStream) Send(event *pb.OrderEvent) error {
	select {
	case f.sendCh <- event:
		return nil
	case <-f.ctx.Done():
		return f.ctx.Err()
	}
}

func (f *fakeWatchStream) Context() context.Context {
	return f.ctx
}