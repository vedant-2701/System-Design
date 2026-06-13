package producerconsumer

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// ---- Basic correctness ----

func TestSingleProducerConsumer_OrderPreserved(t *testing.T) {
	buf, _ := NewBoundedBufferReady[int](10)
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		if err := buf.Put(ctx, i); err != nil {
			t.Fatalf("Put failed: %v", err)
		}
	}

	for i := 0; i < 5; i++ {
		got, err := buf.Take(ctx)
		if err != nil {
			t.Fatalf("Take failed: %v", err)
		}
		if got != i {
			t.Errorf("expected %d, got %d", i, got)
		}
	}
}

func TestNewBoundedBuffer_InvalidCapacity(t *testing.T) {
	_, err := NewBoundedBuffer[int](0)
	if err == nil {
		t.Error("expected error for capacity 0")
	}
	_, err = NewBoundedBuffer[int](-5)
	if err == nil {
		t.Error("expected error for negative capacity")
	}
}

// ---- Backpressure ----

func TestPut_BlocksWhenFull(t *testing.T) {
	buf, _ := NewBoundedBufferReady[int](2)

	ctx := context.Background()
	_ = buf.Put(ctx, 1)
	_ = buf.Put(ctx, 2) // buffer now full

	// Put with a short-lived context — should fail quickly, not block forever
	ctxTimeout, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err := buf.Put(ctxTimeout, 3)
	if err == nil {
		t.Error("expected Put to fail when buffer is full and context times out")
	}
}

func TestTake_BlocksWhenEmpty(t *testing.T) {
	buf, _ := NewBoundedBufferReady[int](5)
	// buffer is empty

	ctxTimeout, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, err := buf.Take(ctxTimeout)
	if err == nil {
		t.Error("expected Take to fail when buffer is empty and context times out")
	}
}

func TestProducer_UnblocksAfterConsumerDrains(t *testing.T) {
	buf, _ := NewBoundedBufferReady[int](1)
	ctx := context.Background()

	_ = buf.Put(ctx, 1) // fill buffer

	unblocked := make(chan struct{})
	go func() {
		_ = buf.Put(ctx, 2) // should block until drained
		close(unblocked)
	}()

	time.Sleep(50 * time.Millisecond) // let goroutine block

	_, _ = buf.Take(ctx) // drain — unblocks producer

	select {
	case <-unblocked:
		// success
	case <-time.After(1 * time.Second):
		t.Error("producer did not unblock after consumer drained")
	}
}

// ---- Concurrency correctness ----

func TestConcurrentProducersConsumers_NoItemsLost(t *testing.T) {
	const (
		capacity      = 50
		producerCount = 4
		consumerCount = 4
		itemsPerProd  = 1000
		totalItems    = producerCount * itemsPerProd
	)

	buf, _ := NewBoundedBufferReady[int](capacity)

	// Use a cancellable context — cancelled after all producers finish,
	// which unblocks any consumers waiting on an empty buffer.
	ctx, cancel := context.WithCancel(context.Background())

	var consumed int64
	var producerWg sync.WaitGroup
	var consumerWg sync.WaitGroup

	// Start consumers — exit when ctx is cancelled (producers done + buffer drained)
	for c := 0; c < consumerCount; c++ {
		consumerWg.Add(1)
		go func() {
			defer consumerWg.Done()
			for {
				_, err := buf.Take(ctx)
				if err != nil {
					return // context cancelled — exit cleanly
				}
				atomic.AddInt64(&consumed, 1)
			}
		}()
	}

	// Start producers — each writes a distinct range
	for p := 0; p < producerCount; p++ {
		base := p * itemsPerProd
		producerWg.Add(1)
		go func() {
			defer producerWg.Done()
			for i := 0; i < itemsPerProd; i++ {
				if err := buf.Put(ctx, base+i); err != nil {
					return
				}
			}
		}()
	}

	// Wait for all producers to finish, then drain remaining items, then cancel
	go func() {
		producerWg.Wait()
		// Drain: wait until buffer is empty before cancelling consumers
		for buf.Size() > 0 {
			time.Sleep(time.Millisecond)
		}
		// Brief settle — let consumers finish the last in-flight Take
		time.Sleep(10 * time.Millisecond)
		cancel()
	}()

	done := make(chan struct{})
	go func() {
		consumerWg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(8 * time.Second):
		t.Fatalf("test timed out — consumed=%d expected=%d", atomic.LoadInt64(&consumed), totalItems)
	}

	if atomic.LoadInt64(&consumed) != totalItems {
		t.Errorf("items lost: consumed=%d expected=%d", consumed, totalItems)
	}
}

// ---- Shutdown via context cancellation ----

func TestContextCancellation_UnblocksWaitingConsumers(t *testing.T) {
	buf, _ := NewBoundedBufferReady[int](10)
	// buffer is empty — consumers will block on Take

	ctx, cancel := context.WithCancel(context.Background())
	consumerCount := 3
	var exitCount int64
	var wg sync.WaitGroup

	for i := 0; i < consumerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := buf.Take(ctx)
			if err != nil {
				atomic.AddInt64(&exitCount, 1)
			}
		}()
	}

	time.Sleep(50 * time.Millisecond) // let consumers block
	cancel()                          // cancel context — unblocks all consumers

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(1 * time.Second):
		t.Error("consumers did not unblock after context cancellation")
	}

	if atomic.LoadInt64(&exitCount) != int64(consumerCount) {
		t.Errorf("expected %d consumers to exit, got %d", consumerCount, exitCount)
	}
}

// ---- Observability ----

func TestSizeAndRemainingCapacity(t *testing.T) {
	buf, _ := NewBoundedBufferReady[int](5)
	ctx := context.Background()

	if buf.Size() != 0 {
		t.Errorf("expected size 0, got %d", buf.Size())
	}
	if buf.RemainingCapacity() != 5 {
		t.Errorf("expected remaining 5, got %d", buf.RemainingCapacity())
	}

	_ = buf.Put(ctx, 1)
	_ = buf.Put(ctx, 2)

	if buf.Size() != 2 {
		t.Errorf("expected size 2, got %d", buf.Size())
	}
	if buf.RemainingCapacity() != 3 {
		t.Errorf("expected remaining 3, got %d", buf.RemainingCapacity())
	}

	_, _ = buf.Take(ctx)

	if buf.Size() != 1 {
		t.Errorf("expected size 1, got %d", buf.Size())
	}
}

// ---- Race detector validation ----
// Run with: go test -race ./...

func TestRaceDetector_ConcurrentAccess(t *testing.T) {
	buf, _ := NewBoundedBufferReady[int](20)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	var wg sync.WaitGroup

	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				_ = buf.Put(ctx, id*100+j)
			}
		}(i)
	}

	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				_, _ = buf.Take(ctx)
			}
		}()
	}

	wg.Wait()
}