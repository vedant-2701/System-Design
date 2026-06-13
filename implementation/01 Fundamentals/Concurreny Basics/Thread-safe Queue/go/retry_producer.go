package queue

import (
	"context"
	"log"
	"time"
)

// RetryConfig holds configuration for exponential backoff retry behavior.
type RetryConfig struct {
	MaxRetries     int
	InitialDelayMs time.Duration
	MaxDelayMs     time.Duration
}

// DefaultRetryConfig provides sensible production defaults.
var DefaultRetryConfig = RetryConfig{
	MaxRetries:     5,
	InitialDelayMs: 50 * time.Millisecond,
	MaxDelayMs:     2 * time.Second,
}

// ProduceWithRetry attempts to enqueue an item with exponential backoff.
//
// Go vs Java difference:
//   Java: Thread.sleep() for backoff — blocks the OS thread.
//   Go:   time.Sleep() or select with timer — blocks the goroutine only,
//         not the OS thread. Go scheduler runs other goroutines during sleep.
//         This makes goroutine-based retry far cheaper than thread-based retry.
//
// Returns true if enqueued, false after all retries exhausted.
// Respects ctx cancellation — stops retrying if context is cancelled.
func ProduceWithRetry[T any](ctx context.Context, q *BoundedQueue[T], item T, cfg RetryConfig) bool {
	delay := cfg.InitialDelayMs

	for attempt := 1; attempt <= cfg.MaxRetries; attempt++ {
		err := q.OfferWithContext(ctx, item)
		if err == nil {
			if attempt > 1 {
				log.Printf("Item accepted on attempt %d", attempt)
			}
			return true
		}

		if err == ErrQueueShutdown {
			log.Printf("Queue shut down — abandoning item after %d attempts", attempt)
			return false
		}

		if ctx.Err() != nil {
			log.Printf("Context cancelled — abandoning item after %d attempts", attempt)
			return false
		}

		if attempt == cfg.MaxRetries {
			log.Printf("Failed to enqueue after %d attempts — queue full. "+
				"Consider: scaling consumers, increasing capacity, or shedding load.", cfg.MaxRetries)
			return false
		}

		log.Printf("Queue full on attempt %d — backing off %v", attempt, delay)

		// Use select + timer instead of time.Sleep — allows context cancellation
		// to interrupt the backoff wait immediately rather than sleeping through it
		select {
		case <-time.After(delay):
		case <-ctx.Done():
			return false
		}

		// Exponential backoff capped at maxDelayMs
		delay = delay * 2
		if delay > cfg.MaxDelayMs {
			delay = cfg.MaxDelayMs
		}
	}

	return false
}