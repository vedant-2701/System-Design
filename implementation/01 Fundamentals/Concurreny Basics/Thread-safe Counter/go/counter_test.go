package counter

import (
	"sync"
	"testing"
)

// Test constants — high enough to make races statistically certain to manifest.
const (
	threadCount   = 10
	opsPerThread  = 100_000
	expectedTotal = int64(threadCount * opsPerThread)
)

// Run these tests with: go test -race ./...
// The -race flag enables Go's built-in race detector.
// UnsafeCounter will trigger race detector warnings immediately.
// MutexCounter and AtomicCounter should produce zero race warnings.

// =============================================================================
// Shared test helpers
// =============================================================================

// runConcurrent launches n goroutines that all start simultaneously via WaitGroup
// and each execute fn opsPerThread times. Blocks until all goroutines complete.
//
// sync.WaitGroup pattern:
//   wg.Add(n)        — register n goroutines to wait for
//   go func() {
//     defer wg.Done() — signal completion
//     ...
//   }()
//   wg.Wait()        — block until Done() called n times
func runConcurrent(n int, fn func()) {
	// ready channel acts as a gate — all goroutines wait until closed simultaneously.
	// This maximizes contention — worst-case scenario for the synchronization mechanism.
	ready := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(n)

	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			<-ready // block until gate opens
			for j := 0; j < opsPerThread; j++ {
				fn()
			}
		}()
	}

	close(ready) // release all goroutines simultaneously
	wg.Wait()
}

// =============================================================================
// Functional correctness — single goroutine
// =============================================================================

func TestMutexCounter_Increment(t *testing.T) {
	c := &MutexCounter{}
	c.Increment()
	c.Increment()
	c.Increment()
	if got := c.Get(); got != 3 {
		t.Errorf("expected 3, got %d", got)
	}
}

func TestAtomicCounter_Increment(t *testing.T) {
	c := &AtomicCounter{}
	c.Increment()
	c.Increment()
	c.Increment()
	if got := c.Get(); got != 3 {
		t.Errorf("expected 3, got %d", got)
	}
}

func TestCounter_Decrement_BelowZero(t *testing.T) {
	counters := []Counter{&MutexCounter{}, &AtomicCounter{}}
	for _, c := range counters {
		c.Decrement()
		if got := c.Get(); got != -1 {
			t.Errorf("expected -1, got %d", got)
		}
	}
}

func TestCounter_IncrementBy_ValidDelta(t *testing.T) {
	counters := []Counter{&MutexCounter{}, &AtomicCounter{}}
	for _, c := range counters {
		if err := c.IncrementBy(50); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if got := c.Get(); got != 50 {
			t.Errorf("expected 50, got %d", got)
		}
	}
}

func TestCounter_IncrementBy_InvalidDelta(t *testing.T) {
	counters := []Counter{&MutexCounter{}, &AtomicCounter{}}
	for _, c := range counters {
		if err := c.IncrementBy(0); err == nil {
			t.Error("expected error for delta=0, got nil")
		}
		if err := c.IncrementBy(-5); err == nil {
			t.Error("expected error for delta=-5, got nil")
		}
	}
}

func TestCounter_Reset(t *testing.T) {
	counters := []Counter{&MutexCounter{}, &AtomicCounter{}}
	for _, c := range counters {
		_ = c.IncrementBy(1000)
		c.Reset()
		if got := c.Get(); got != 0 {
			t.Errorf("expected 0 after reset, got %d", got)
		}
	}
}

// =============================================================================
// Concurrency correctness — thread-safe implementations must lose zero increments
// =============================================================================

func TestMutexCounter_HighConcurrency_NoLostUpdates(t *testing.T) {
	c := &MutexCounter{}
	runConcurrent(threadCount, c.Increment)
	if got := c.Get(); got != expectedTotal {
		t.Errorf("lost increments: expected %d, got %d (lost %d)",
			expectedTotal, got, expectedTotal-got)
	}
}

func TestAtomicCounter_HighConcurrency_NoLostUpdates(t *testing.T) {
	c := &AtomicCounter{}
	runConcurrent(threadCount, c.Increment)
	if got := c.Get(); got != expectedTotal {
		t.Errorf("lost increments: expected %d, got %d (lost %d)",
			expectedTotal, got, expectedTotal-got)
	}
}

func TestMutexCounter_MixedOperations_CorrectNetValue(t *testing.T) {
	// Half goroutines increment, half decrement. Net must be zero.
	c := &MutexCounter{}
	ready := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(threadCount)

	for i := 0; i < threadCount; i++ {
		increment := i%2 == 0
		go func(inc bool) {
			defer wg.Done()
			<-ready
			for j := 0; j < opsPerThread; j++ {
				if inc {
					c.Increment()
				} else {
					c.Decrement()
				}
			}
		}(increment)
	}

	close(ready)
	wg.Wait()

	if got := c.Get(); got != 0 {
		t.Errorf("mixed ops should net to 0, got %d", got)
	}
}

func TestAtomicCounter_MixedOperations_CorrectNetValue(t *testing.T) {
	c := &AtomicCounter{}
	ready := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(threadCount)

	for i := 0; i < threadCount; i++ {
		increment := i%2 == 0
		go func(inc bool) {
			defer wg.Done()
			<-ready
			for j := 0; j < opsPerThread; j++ {
				if inc {
					c.Increment()
				} else {
					c.Decrement()
				}
			}
		}(increment)
	}

	close(ready)
	wg.Wait()

	if got := c.Get(); got != 0 {
		t.Errorf("mixed ops should net to 0, got %d", got)
	}
}

// =============================================================================
// AtomicCounter-specific: GetAndReset
// =============================================================================

func TestAtomicCounter_GetAndReset_ReturnsSnapshotAndZeros(t *testing.T) {
	c := &AtomicCounter{}
	_ = c.IncrementBy(500)

	snapshot := c.GetAndReset()

	if snapshot != 500 {
		t.Errorf("expected snapshot=500, got %d", snapshot)
	}
	if got := c.Get(); got != 0 {
		t.Errorf("expected 0 after GetAndReset, got %d", got)
	}
}

func TestAtomicCounter_GetAndReset_NoIncrementLost(t *testing.T) {
	// Concurrent increments + GetAndReset: snapshot + remaining must equal total.
	c := &AtomicCounter{}
	incrementGoroutines := 4
	opsEach := 10_000
	totalOps := int64(incrementGoroutines * opsEach)

	ready := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(incrementGoroutines)

	for i := 0; i < incrementGoroutines; i++ {
		go func() {
			defer wg.Done()
			<-ready
			for j := 0; j < opsEach; j++ {
				c.Increment()
			}
		}()
	}

	close(ready)
	wg.Wait()

	snapshot := c.GetAndReset()
	remaining := c.Get()

	// Any increment that raced with GetAndReset lands in remaining.
	if snapshot+remaining != totalOps {
		t.Errorf("lost increments across GetAndReset: snapshot=%d + remaining=%d = %d, want %d",
			snapshot, remaining, snapshot+remaining, totalOps)
	}
}

// =============================================================================
// Benchmarks — run with: go test -bench=. -benchmem
// =============================================================================

func BenchmarkMutexCounter_Increment(b *testing.B) {
	c := &MutexCounter{}
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			c.Increment()
		}
	})
}

func BenchmarkAtomicCounter_Increment(b *testing.B) {
	c := &AtomicCounter{}
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			c.Increment()
		}
	})
}

// BenchmarkAtomicCounter_GetAndReset measures the CAS retry loop cost.
func BenchmarkAtomicCounter_GetAndReset(b *testing.B) {
	c := &AtomicCounter{}
	_ = c.IncrementBy(1_000_000)
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			c.Increment()
			if c.Get() > 10_000 {
				c.GetAndReset()
			}
		}
	})
}