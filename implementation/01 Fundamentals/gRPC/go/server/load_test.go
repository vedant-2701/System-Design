package server

import (
	"context"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	pb "grpc-demo/gen/order/proto"
)

// ─── Benchmarks ──────────────────────────────────────────────────────────────
//
// Run with:
//   go test -bench=. -benchmem -race ./server/...
//
// -race is intentional here even for benchmarks: it's the cheapest way to
// catch data races under realistic concurrent load, at the cost of slower
// execution and inflated allocation numbers (ignore absolute ns/op under
// -race; compare relative numbers between runs instead).

// BenchmarkGetOrder measures read-path throughput.
// Expectation: scales near-linearly with GOMAXPROCS since GetOrder only
// takes RLock — concurrent reads should not serialize against each other.
func BenchmarkGetOrder(b *testing.B) {
	s := NewOrderServer()
	ctx := context.Background()
	req := &pb.GetOrderRequest{OrderId: 0}

	b.RunParallel(func(p *testing.PB) {
		for p.Next() {
			if _, err := s.GetOrder(ctx, req); err != nil {
				b.Fatal(err)
			}
		}
	})
}

// BenchmarkCreateOrder measures write-path throughput under contention.
// Expectation: throughput is bounded by the single Mutex around nextID and
// the orders map. This benchmark establishes the baseline for "how many
// orders/sec can a single instance sustain" — useful for capacity planning
// discussions (Step 2 of HLD framework: scale estimation).
func BenchmarkCreateOrder(b *testing.B) {
	s := NewOrderServer()
	ctx := context.Background()
	req := &pb.CreateOrderRequest{
		Items: []*pb.Item{{Name: "Bench Item", Quantity: 1, PriceCents: 1000}},
	}

	b.RunParallel(func(p *testing.PB) {
		for p.Next() {
			if _, err := s.CreateOrder(ctx, req); err != nil {
				b.Fatal(err)
			}
		}
	})
}

// BenchmarkCreateOrder_WithSubscribers measures the COST of fan-out.
// Compare against BenchmarkCreateOrder: the delta is the overhead of
// broadcasting to N WatchOrders subscribers.
//
// This benchmark answers a concrete production question: "if we have 1000
// dashboard clients subscribed via WatchOrders, how much does that degrade
// our order-creation throughput?"
func BenchmarkCreateOrder_WithSubscribers(b *testing.B) {
	for _, numSubs := range []int{0, 10, 100, 1000} {
		b.Run(subscriberLabel(numSubs), func(b *testing.B) {
			s := NewOrderServer()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			// Register numSubs subscribers that actively drain their channel
			// (realistic — a stuck subscriber would be a different benchmark).
			for i := 0; i < numSubs; i++ {
				recv := make(chan *pb.OrderEvent, 32)
				stream := newFakeWatchStream(ctx, recv)
				go func() { _ = s.WatchOrders(&pb.WatchOrdersRequest{}, stream) }()
				go func() {
					for {
						select {
						case <-recv:
						case <-ctx.Done():
							return
						}
					}
				}()
			}
			waitForSubscriberCount(b, s, numSubs, 5*time.Second)

			req := &pb.CreateOrderRequest{
				Items: []*pb.Item{{Name: "Bench Item", Quantity: 1, PriceCents: 1000}},
			}

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if _, err := s.CreateOrder(ctx, req); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func subscriberLabel(n int) string {
	switch n {
	case 0:
		return "subs=0"
	case 10:
		return "subs=10"
	case 100:
		return "subs=100"
	default:
		return "subs=1000"
	}
}

// ─── High-Concurrency Stress Tests ───────────────────────────────────────────

// TestStress_ManyWatchersManyCreates simulates a realistic high-load scenario:
//   - 500 concurrent WatchOrders subscribers (e.g., dashboard clients)
//   - 200 concurrent CreateOrder callers (e.g., checkout traffic)
//
// Assertions:
//   1. No CreateOrder call takes longer than 100ms (writer isolation holds)
//   2. No goroutine leaks after all subscribers disconnect
//   3. No data race (run with -race)
//
// This is the test that would have CAUGHT the unbuffered-channel deadlock
// scenario discussed in the session — if fanOut blocked, CreateOrder calls
// would pile up and the 100ms assertion would fail immediately.
func TestStress_ManyWatchersManyCreates(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in -short mode")
	}

	s := NewOrderServer()

	const numWatchers = 500
	const numCreators = 200
	const createsPerCreator = 10

	baselineGoroutines := runtime.NumGoroutine()

	ctx, cancel := context.WithCancel(context.Background())

	var watcherWg sync.WaitGroup
	for i := 0; i < numWatchers; i++ {
		recv := make(chan *pb.OrderEvent, 32)
		stream := newFakeWatchStream(ctx, recv)

		watcherWg.Add(1)
		go func() {
			defer watcherWg.Done()
			_ = s.WatchOrders(&pb.WatchOrdersRequest{}, stream)
		}()

		// Drain — most watchers keep up. A few "slow" watchers drain with delay
		// to exercise the drop path realistically.
		go func(idx int) {
			for {
				select {
				case <-recv:
					if idx%50 == 0 {
						time.Sleep(time.Millisecond) // simulate a slow consumer
					}
				case <-ctx.Done():
					return
				}
			}
		}(i)
	}

	waitForSubscriberCount(t, s, numWatchers, 5*time.Second)

	// ── Run concurrent creators, tracking max latency ─────────────────────────
	var maxLatency atomic.Int64 // nanoseconds
	var creatorWg sync.WaitGroup
	errCh := make(chan error, numCreators*createsPerCreator)

	for i := 0; i < numCreators; i++ {
		creatorWg.Add(1)
		go func() {
			defer creatorWg.Done()
			for j := 0; j < createsPerCreator; j++ {
				start := time.Now()
				_, err := s.CreateOrder(context.Background(), &pb.CreateOrderRequest{
					Items: []*pb.Item{{Name: "Stress Item", Quantity: 1, PriceCents: 100}},
				})
				elapsed := time.Since(start).Nanoseconds()

				for {
					cur := maxLatency.Load()
					if elapsed <= cur || maxLatency.CompareAndSwap(cur, elapsed) {
						break
					}
				}

				if err != nil {
					errCh <- err
				}
			}
		}()
	}

	creatorWg.Wait()
	close(errCh)

	for err := range errCh {
		t.Errorf("CreateOrder error: %v", err)
	}

	maxLatencyMs := time.Duration(maxLatency.Load()).Milliseconds()
	t.Logf("max CreateOrder latency under load: %dms", maxLatencyMs)
	t.Logf("dropped events: %d", s.DroppedEventCount())

	// The core assertion: writer latency must remain low regardless of
	// subscriber count or slow subscribers. 100ms is generous for an
	// in-memory operation — a real regression (blocking fan-out) would
	// show latencies in the seconds or a full deadlock (test timeout).
	if maxLatencyMs > 100 {
		t.Errorf("CreateOrder latency too high under load: %dms (writer/reader coupling regression?)", maxLatencyMs)
	}

	// ── Cleanup and goroutine leak check ──────────────────────────────────────
	cancel() // disconnect all watchers
	watcherWg.Wait()

	// Allow goroutines time to exit after context cancellation.
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if s.SubscriberCount() == 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	if s.SubscriberCount() != 0 {
		t.Errorf("expected 0 subscribers after disconnect, got %d (cleanup not happening)", s.SubscriberCount())
	}

	// Goroutine count should return close to baseline. Some slack for
	// runtime/test goroutines, but should NOT scale with numWatchers —
	// that would indicate a leak.
	time.Sleep(100 * time.Millisecond) // let GC/scheduler settle
	finalGoroutines := runtime.NumGoroutine()
	leaked := finalGoroutines - baselineGoroutines
	t.Logf("goroutines: baseline=%d final=%d delta=%d", baselineGoroutines, finalGoroutines, leaked)

	if leaked > 20 { // generous slack; should be near 0
		t.Errorf("possible goroutine leak: %d extra goroutines after cleanup", leaked)
	}
}

// TestStress_FloodSingleSlowSubscriber is a focused version of the production
// incident this design protects against: ONE subscriber that never reads,
// while orders are created at high rate.
//
// Without the non-blocking fan-out, this test would hang until timeout
// (deadlock on the unbuffered/blocking channel send).
func TestStress_FloodSingleSlowSubscriber(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in -short mode")
	}

	s := NewOrderServer()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Never-draining subscriber
	recv := make(chan *pb.OrderEvent) // nobody reads this
	stream := newFakeWatchStream(ctx, recv)
	go func() { _ = s.WatchOrders(&pb.WatchOrdersRequest{}, stream) }()
	waitForSubscriberCount(t, s, 1, 2*time.Second)

	const numOrders = 1000
	start := time.Now()

	for i := 0; i < numOrders; i++ {
		_, err := s.CreateOrder(context.Background(), &pb.CreateOrderRequest{
			Items: []*pb.Item{{Name: "Flood", Quantity: 1, PriceCents: 1}},
		})
		if err != nil {
			t.Fatalf("CreateOrder %d failed: %v", i, err)
		}
	}

	elapsed := time.Since(start)
	t.Logf("created %d orders with 1 stuck subscriber in %v", numOrders, elapsed)
	t.Logf("dropped events: %d", s.DroppedEventCount())

	// 1000 in-memory creates should complete in well under a second if
	// the stuck subscriber isn't blocking the writer path.
	if elapsed > 2*time.Second {
		t.Errorf("creates took too long (%v) — stuck subscriber may be blocking writer", elapsed)
	}

	// Expect drops once the 32-buffer fills (after the first 32 orders).
	if s.DroppedEventCount() < uint64(numOrders-32) {
		t.Errorf("expected ~%d dropped events, got %d", numOrders-32, s.DroppedEventCount())
	}
}