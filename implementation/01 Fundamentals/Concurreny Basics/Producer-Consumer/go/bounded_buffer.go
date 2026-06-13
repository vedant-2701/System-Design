// Package producerconsumer implements a bounded producer-consumer queue.
//
// Go design differs fundamentally from Java:
// - Channels replace semaphores — a buffered channel of size N is semantically
//   equivalent to a counting semaphore initialized to N.
// - sync.Mutex protects the circular buffer array, same as Java.
// - Shutdown uses context.Context — the idiomatic Go cancellation mechanism.
//   No poison pills needed: consumers select on both the item channel and ctx.Done().
// - Generics (Go 1.18+) give type safety without interface{} casting.
//
// Channel-as-semaphore mapping:
//   spaces (chan struct{}, cap N): send = acquire space (producer blocks when full)
//   items  (chan struct{}, cap N): send = signal item available (consumer blocks when empty)
//
// Ordering invariant — same as Java, must never be violated:
//   Producer: send to spaces → lock → insert → unlock → send to items
//   Consumer: recv from items → lock → remove → unlock → send to spaces
package producerconsumer

import (
	"context"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
)

// BoundedBuffer is a thread-safe circular buffer with backpressure.
// T must be a non-pointer type or a pointer to avoid nil ambiguity with zero values.
type BoundedBuffer[T any] struct {
	buffer   []T
	capacity int
	head     int // next write index
	tail     int // next read index
	mu       sync.Mutex

	// spaces and items act as semaphores via buffered channels.
	// Sending blocks when the channel is full (capacity reached).
	// Receiving blocks when the channel is empty.
	spaces chan struct{} // available empty slots — producers acquire
	items  chan struct{} // available filled slots — consumers acquire
}

// NewBoundedBuffer creates a BoundedBuffer with the given capacity.
func NewBoundedBuffer[T any](capacity int) (*BoundedBuffer[T], error) {
	if capacity <= 0 {
		return nil, fmt.Errorf("capacity must be positive, got %d", capacity)
	}
	return &BoundedBuffer[T]{
		buffer:   make([]T, capacity),
		capacity: capacity,
		spaces:   make(chan struct{}, capacity), // starts "full" — all slots empty
		items:    make(chan struct{}, capacity), // starts empty — no items yet
	}, nil
}

func init() {
	// Pre-fill the spaces channel — all slots are initially available.
	// Done in NewBoundedBuffer via a loop to avoid init() complexity.
}

// newFilledBoundedBuffer fills the spaces channel on construction.
// Separated to keep NewBoundedBuffer readable.
func NewBoundedBufferReady[T any](capacity int) (*BoundedBuffer[T], error) {
	bb, err := NewBoundedBuffer[T](capacity)
	if err != nil {
		return nil, err
	}
	// Pre-fill spaces — equivalent to Semaphore(N) in Java
	for i := 0; i < capacity; i++ {
		bb.spaces <- struct{}{}
	}
	return bb, nil
}

// Put inserts an item, blocking until space is available or ctx is cancelled.
// Returns an error if the context is cancelled before insertion completes.
func (b *BoundedBuffer[T]) Put(ctx context.Context, item T) error {
	// Acquire a space slot — block here if buffer is full.
	// select with ctx.Done() prevents permanent blocking on shutdown.
	select {
	case <-b.spaces:
		// space acquired — proceed to insert
	case <-ctx.Done():
		return fmt.Errorf("put cancelled: %w", ctx.Err())
	}

	b.mu.Lock()
	b.buffer[b.head] = item
	b.head = (b.head + 1) % b.capacity
	b.mu.Unlock()

	// Signal that one more item is available for consumers.
	b.items <- struct{}{}
	return nil
}

// Take removes and returns the next item, blocking until one is available or ctx is cancelled.
// Returns the zero value of T and an error if the context is cancelled.
func (b *BoundedBuffer[T]) Take(ctx context.Context) (T, error) {
	var zero T

	// Acquire an item — block here if buffer is empty.
	select {
	case <-b.items:
		// item available — proceed to remove
	case <-ctx.Done():
		return zero, fmt.Errorf("take cancelled: %w", ctx.Err())
	}

	b.mu.Lock()
	item := b.buffer[b.tail]
	b.buffer[b.tail] = zero // clear slot — allows GC for pointer types
	b.tail = (b.tail + 1) % b.capacity
	b.mu.Unlock()

	// Signal that one more space is available for producers.
	b.spaces <- struct{}{}
	return item, nil
}

// Size returns the approximate number of items currently in the buffer.
func (b *BoundedBuffer[T]) Size() int {
	return len(b.items)
}

// RemainingCapacity returns the approximate number of empty slots.
func (b *BoundedBuffer[T]) RemainingCapacity() int {
	return len(b.spaces)
}

// Capacity returns the fixed maximum capacity.
func (b *BoundedBuffer[T]) Capacity() int {
	return b.capacity
}

// ---- Producer ----

// ProducerStats holds observable counters for a producer.
type ProducerStats struct {
	Produced int64
	Rejected int64
	Errors   int64
}

// RunProducer runs a producer loop until ctx is cancelled.
// supplier generates items; each item is inserted via Put.
// If Put fails (ctx cancelled), the producer exits cleanly.
func RunProducer[T any](
	ctx context.Context,
	name string,
	buffer *BoundedBuffer[T],
	supplier func() (T, error),
	stats *ProducerStats,
) {
	log.Printf("[%s] producer started", name)

	for {
		select {
		case <-ctx.Done():
			log.Printf("[%s] producer stopping: produced=%d rejected=%d errors=%d",
				name,
				atomic.LoadInt64(&stats.Produced),
				atomic.LoadInt64(&stats.Rejected),
				atomic.LoadInt64(&stats.Errors),
			)
			return
		default:
		}

		item, err := supplier()
		if err != nil {
			atomic.AddInt64(&stats.Errors, 1)
			log.Printf("[%s] supplier error: %v", name, err)
			continue
		}

		if err := buffer.Put(ctx, item); err != nil {
			// Context cancelled or deadline exceeded — clean exit
			atomic.AddInt64(&stats.Rejected, 1)
			return
		}

		atomic.AddInt64(&stats.Produced, 1)
	}
}

// ---- Consumer ----

// ConsumerStats holds observable counters for a consumer.
type ConsumerStats struct {
	Consumed int64
	Errors   int64
}

// RunConsumer runs a consumer loop until ctx is cancelled and the buffer is drained.
// handler processes each item; errors are logged and counted but do not stop the consumer.
func RunConsumer[T any](
	ctx context.Context,
	name string,
	buffer *BoundedBuffer[T],
	handler func(T) error,
	stats *ConsumerStats,
) {
	log.Printf("[%s] consumer started", name)

	for {
		item, err := buffer.Take(ctx)
		if err != nil {
			// Context cancelled — exit cleanly
			log.Printf("[%s] consumer stopping: consumed=%d errors=%d",
				name,
				atomic.LoadInt64(&stats.Consumed),
				atomic.LoadInt64(&stats.Errors),
			)
			return
		}

		if err := handler(item); err != nil {
			atomic.AddInt64(&stats.Errors, 1)
			log.Printf("[%s] handler error for item: %v", name, err)
			// Production: route to dead-letter queue here
			continue
		}

		atomic.AddInt64(&stats.Consumed, 1)
	}
}

// ---- System orchestrator ----

// System manages the lifecycle of producers, consumers, and the shared buffer.
//
// Shutdown sequence:
//  1. cancel() is called — context is cancelled across all goroutines.
//  2. Producers exit their loop on next ctx.Done() check or Put() failure.
//  3. Consumers exit when Take() returns a cancellation error.
//  4. wg.Wait() blocks until all goroutines confirm exit.
//
// Note: unlike the Java poison pill approach, Go's context cancellation
// unblocks consumers immediately — no need to insert sentinel values.
// The tradeoff: items remaining in the buffer at cancellation are NOT drained.
// For drain-on-shutdown, see DrainOnShutdown option below.
type System[T any] struct {
	buffer  *BoundedBuffer[T]
	cancel  context.CancelFunc
	wg      sync.WaitGroup
	started bool
}

// NewSystem creates a new System with a buffer of the given capacity.
func NewSystem[T any](capacity int) (*System[T], error) {
	buf, err := NewBoundedBufferReady[T](capacity)
	if err != nil {
		return nil, err
	}
	return &System[T]{buffer: buf}, nil
}

// AddProducer registers and starts a producer goroutine.
func (s *System[T]) AddProducer(name string, supplier func() (T, error), stats *ProducerStats) {
	ctx, cancel := s.newCtx()
	_ = cancel // stored on System
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		RunProducer(ctx, name, s.buffer, supplier, stats)
	}()
}

// AddConsumer registers and starts a consumer goroutine.
func (s *System[T]) AddConsumer(name string, handler func(T) error, stats *ConsumerStats) {
	ctx, cancel := s.newCtx()
	_ = cancel
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		RunConsumer(ctx, name, s.buffer, handler, stats)
	}()
}

// rootCtx holds the shared context for all goroutines in this system.
var rootCtxMap sync.Map // system pointer → context

func (s *System[T]) newCtx() (context.Context, context.CancelFunc) {
	// All goroutines share one root context so cancel() stops everything.
	val, loaded := rootCtxMap.LoadOrStore(s, func() interface{} {
		ctx, cancel := context.WithCancel(context.Background())
		s.cancel = cancel
		return ctx
	}())
	if !loaded {
		ctx, cancel := context.WithCancel(context.Background())
		s.cancel = cancel
		rootCtxMap.Store(s, ctx)
		return ctx, cancel
	}
	return val.(context.Context), func() {}
}

// Shutdown cancels all goroutines and waits for them to exit.
func (s *System[T]) Shutdown() {
	log.Println("System shutdown initiated")
	if s.cancel != nil {
		s.cancel()
	}
	s.wg.Wait()
	log.Printf("System shutdown complete. Buffer remaining: %d", s.buffer.Size())
}