package queue

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// -------------------------------------------------------------------------
// Basic correctness
// -------------------------------------------------------------------------

func TestFifoOrdering(t *testing.T) {
	q, _ := New[int](5)

	q.Put(1)
	q.Put(2)
	q.Put(3)

	assertTake(t, q, 1)
	assertTake(t, q, 2)
	assertTake(t, q, 3)
}

func TestSizeTracking(t *testing.T) {
	q, _ := New[int](5)

	if q.Size() != 0 {
		t.Fatalf("expected size 0, got %d", q.Size())
	}
	q.Put(1)
	if q.Size() != 1 {
		t.Fatalf("expected size 1, got %d", q.Size())
	}
	q.Take()
	if q.Size() != 0 {
		t.Fatalf("expected size 0 after take, got %d", q.Size())
	}
}

func TestFullAndEmptyFlags(t *testing.T) {
	q, _ := New[int](3)

	if !q.IsEmpty() {
		t.Fatal("expected empty queue")
	}
	q.Put(1)
	q.Put(2)
	q.Put(3)
	if !q.IsFull() {
		t.Fatal("expected full queue")
	}
}

// -------------------------------------------------------------------------
// Blocking semantics
// -------------------------------------------------------------------------

func TestProducerBlocksWhenFull(t *testing.T) {
	q, _ := New[int](3)
	q.Put(1)
	q.Put(2)
	q.Put(3)

	var inserted atomic.Bool

	go func() {
		q.Put(4) // should block
		inserted.Store(true)
	}()

	time.Sleep(150 * time.Millisecond)

	if inserted.Load() {
		t.Fatal("producer should be blocked — queue is full")
	}

	// Unblock the producer by consuming one item
	q.Take()
	time.Sleep(100 * time.Millisecond)

	if !inserted.Load() {
		t.Fatal("producer should have unblocked after take")
	}
}

func TestConsumerBlocksWhenEmpty(t *testing.T) {
	q, _ := New[int](3)

	var consumed atomic.Bool

	go func() {
		q.Take() // should block
		consumed.Store(true)
	}()

	time.Sleep(150 * time.Millisecond)

	if consumed.Load() {
		t.Fatal("consumer should be blocked — queue is empty")
	}

	q.Put(42)
	time.Sleep(100 * time.Millisecond)

	if !consumed.Load() {
		t.Fatal("consumer should have unblocked after put")
	}
}

// -------------------------------------------------------------------------
// Timeout semantics
// -------------------------------------------------------------------------

func TestOfferReturnsErrOnTimeout(t *testing.T) {
	q, _ := New[int](3)
	q.Put(1)
	q.Put(2)
	q.Put(3)

	err := q.Offer(4, 150*time.Millisecond)
	if err != ErrQueueFull {
		t.Fatalf("expected ErrQueueFull, got %v", err)
	}
}

func TestPollReturnsErrOnTimeout(t *testing.T) {
	q, _ := New[int](3)

	_, err := q.Poll(150 * time.Millisecond)
	if err != ErrQueueEmpty {
		t.Fatalf("expected ErrQueueEmpty, got %v", err)
	}
}

func TestOfferSucceedsWhenSpaceOpens(t *testing.T) {
	q, _ := New[int](3)
	q.Put(1)
	q.Put(2)
	q.Put(3)

	// Consumer frees space after 100ms
	go func() {
		time.Sleep(100 * time.Millisecond)
		q.Take()
	}()

	err := q.Offer(4, 500*time.Millisecond)
	if err != nil {
		t.Fatalf("expected offer to succeed, got %v", err)
	}
}

// -------------------------------------------------------------------------
// Context cancellation
// -------------------------------------------------------------------------

func TestOfferRespectsContextCancellation(t *testing.T) {
	q, _ := New[int](3)
	q.Put(1)
	q.Put(2)
	q.Put(3)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err := q.OfferWithContext(ctx, 4)
	if err != ErrQueueFull {
		t.Fatalf("expected ErrQueueFull after context timeout, got %v", err)
	}
}

// -------------------------------------------------------------------------
// Concurrent correctness — most important
// -------------------------------------------------------------------------

func TestNoItemsLostUnderHighConcurrency(t *testing.T) {
	const (
		itemCount    = 5000
		producerCount = 5
		consumerCount = 5
	)

	q, _ := New[int](100)

	var wgProducers sync.WaitGroup
	var wgConsumers sync.WaitGroup
	var consumedCount atomic.Int64

	// Start producers
	for p := 0; p < producerCount; p++ {
		wgProducers.Add(1)
		go func(id int) {
			defer wgProducers.Done()
			for i := 0; i < itemCount/producerCount; i++ {
				if err := q.Put(id*10000 + i); err != nil {
					t.Errorf("unexpected put error: %v", err)
					return
				}
			}
		}(p)
	}

	// Start consumers
	for c := 0; c < consumerCount; c++ {
		wgConsumers.Add(1)
		go func() {
			defer wgConsumers.Done()
			for {
				_, err := q.Poll(500 * time.Millisecond)
				if err == nil {
					consumedCount.Add(1)
					if consumedCount.Load() == itemCount {
						return
					}
				} else if err == ErrQueueEmpty {
					if consumedCount.Load() >= itemCount {
						return
					}
				} else if err == ErrQueueShutdown {
					return
				}
			}
		}()
	}

	wgProducers.Wait()
	wgConsumers.Wait()

	if got := consumedCount.Load(); got != itemCount {
		t.Fatalf("expected %d items consumed, got %d — items were lost or duplicated", itemCount, got)
	}
}

// -------------------------------------------------------------------------
// Shutdown semantics
// -------------------------------------------------------------------------

func TestRejectsNewItemsAfterShutdown(t *testing.T) {
	q, _ := New[int](5)
	q.Put(1)
	q.Shutdown()

	err := q.Put(2)
	if err != ErrQueueShutdown {
		t.Fatalf("expected ErrQueueShutdown, got %v", err)
	}
}

func TestAllowsDrainingAfterShutdown(t *testing.T) {
	q, _ := New[int](5)
	q.Put(1)
	q.Put(2)
	q.Shutdown()

	// Existing items should still be consumable
	assertTake(t, q, 1)
	assertTake(t, q, 2)

	if !q.IsTerminated() {
		t.Fatal("expected queue to be terminated after draining")
	}
}

func TestShutdownIdempotent(t *testing.T) {
	q, _ := New[int](5)

	// Should not panic on multiple shutdown calls
	q.Shutdown()
	q.Shutdown()
	q.Shutdown()
}

// -------------------------------------------------------------------------
// Edge cases
// -------------------------------------------------------------------------

func TestRejectsZeroCapacity(t *testing.T) {
	_, err := New[int](0)
	if err == nil {
		t.Fatal("expected error for zero capacity")
	}

	_, err = New[int](-1)
	if err == nil {
		t.Fatal("expected error for negative capacity")
	}
}

// -------------------------------------------------------------------------
// Helpers
// -------------------------------------------------------------------------

func assertTake[T comparable](t *testing.T, q *BoundedQueue[T], expected T) {
	t.Helper()
	item, err := q.Take()
	if err != nil {
		t.Fatalf("unexpected error on take: %v", err)
	}
	if item != expected {
		t.Fatalf("expected %v, got %v", expected, item)
	}
}