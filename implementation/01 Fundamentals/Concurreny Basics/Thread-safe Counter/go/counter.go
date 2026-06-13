// Package counter provides thread-safe counter implementations with
// different synchronization strategies. Use this package to understand
// the correctness and performance tradeoffs between mutex-based and
// lock-free (CAS) approaches.
package counter

import (
	"fmt"
	"sync"
	"sync/atomic"
)

// Counter defines the behavior of a concurrent counter.
// All implementations must be safe for concurrent use by multiple goroutines.
//
// Design note: the interface exposes behavior (increment, get), not mechanism
// (lock, unlock). The caller never knows or cares how thread safety is achieved.
type Counter interface {
	Increment()
	Decrement()
	IncrementBy(delta int64) error
	Reset()
	Get() int64
}

// =============================================================================
// UnsafeCounter — demonstrates the race condition. NOT for production use.
// =============================================================================

// UnsafeCounter is intentionally non-thread-safe.
// Run with go test -race to observe data race detection.
//
// Race condition: counter++ compiles to:
//   MOVQ  value → register
//   ADDQ  $1, register
//   MOVQ  register → value
//
// Two goroutines can interleave these steps and both write value+1,
// losing one increment.
type UnsafeCounter struct {
	value int64
}

func (c *UnsafeCounter) Increment()              { c.value++ }
func (c *UnsafeCounter) Decrement()              { c.value-- }
func (c *UnsafeCounter) Reset()                  { c.value = 0 }
func (c *UnsafeCounter) Get() int64              { return c.value }
func (c *UnsafeCounter) IncrementBy(delta int64) error {
	if delta <= 0 {
		return fmt.Errorf("delta must be positive, got %d", delta)
	}
	c.value += delta
	return nil
}

// =============================================================================
// MutexCounter — correct under concurrency, mutex-based synchronization.
// =============================================================================

// MutexCounter uses sync.Mutex to serialize all operations.
//
// sync.Mutex in Go:
//   - Not reentrant (unlike Java's ReentrantLock).
//     Calling lock() twice from the same goroutine deadlocks immediately.
//   - Zero value is a valid unlocked mutex — no initialization needed.
//   - The Go memory model guarantees: unlock happens-before any subsequent lock.
//     This ensures all writes inside the critical section are visible to
//     the next goroutine that acquires the lock.
//
// Why embed mutex in the struct rather than use a package-level lock:
//   Lock granularity. One lock per counter instance means multiple counters
//   don't contend with each other. A package-level lock would serialize
//   operations across all counters — a coarse lock anti-pattern.
//
// Convention: embed mu sync.Mutex as a field, never pass by value.
// Copying a mutex copies its internal state, breaking the synchronization.
// Always use *MutexCounter (pointer receiver) to prevent accidental copying.
type MutexCounter struct {
	mu    sync.Mutex
	value int64
}

func (c *MutexCounter) Increment() {
	c.mu.Lock()
	defer c.mu.Unlock() // defer guarantees unlock even if panic occurs
	c.value++
}

func (c *MutexCounter) Decrement() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.value--
}

func (c *MutexCounter) IncrementBy(delta int64) error {
	if delta <= 0 {
		return fmt.Errorf("delta must be positive, got %d", delta)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.value += delta
	return nil
}

// Reset is not atomic with concurrent increments — see interface doc.
func (c *MutexCounter) Reset() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.value = 0
}

// Get acquires the lock to guarantee a consistent read.
// Without the lock, a concurrent write could produce a partially
// written value (torn read) on architectures where int64 isn't
// naturally atomic. On modern 64-bit hardware this rarely manifests,
// but the Go memory model does not guarantee it without synchronization.
func (c *MutexCounter) Get() int64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.value
}

// QueueLength returns the number of goroutines waiting for the lock.
// Useful for monitoring lock contention in production.
// Note: sync.Mutex doesn't expose this directly — use expvar or metrics
// middleware in production rather than this simplified diagnostic.
func (c *MutexCounter) IsLocked() bool {
	// TryLock returns false if the mutex is currently held.
	// If we acquire it, immediately release it — this is a diagnostic only.
	if c.mu.TryLock() {
		c.mu.Unlock()
		return false
	}
	return true
}

// =============================================================================
// AtomicCounter — lock-free, CAS-based. Production default for simple counters.
// =============================================================================

// AtomicCounter uses sync/atomic operations, which map to single CPU instructions
// (LOCK XADD on x86). No OS involvement, no goroutine sleeping, no context switch.
//
// sync/atomic in Go:
//   atomic.AddInt64(&v, 1) — atomically adds 1, returns new value.
//   atomic.LoadInt64(&v)   — atomically reads the value.
//   atomic.StoreInt64(&v, 0) — atomically writes 0.
//   atomic.CompareAndSwapInt64(&v, old, new) — CAS primitive.
//
// Important: the field being operated on must be 64-bit aligned in memory.
// On 32-bit systems, int64 fields in structs may not be aligned.
// Convention: place atomic int64 fields at the start of the struct,
// or use atomic.Int64 (Go 1.19+) which handles alignment automatically.
//
// Go 1.19+ alternative:
//   Use atomic.Int64 struct type instead of int64 + sync/atomic functions.
//   It provides a cleaner API and alignment guarantees.
//   We use the function-based API here because it's more widely seen in codebases.
type AtomicCounter struct {
	// value must be 64-bit aligned. Placing it first in the struct guarantees
	// this on all platforms including 32-bit ARM.
	value int64
}

func (c *AtomicCounter) Increment() {
	atomic.AddInt64(&c.value, 1)
}

func (c *AtomicCounter) Decrement() {
	atomic.AddInt64(&c.value, -1)
}

func (c *AtomicCounter) IncrementBy(delta int64) error {
	if delta <= 0 {
		return fmt.Errorf("delta must be positive, got %d", delta)
	}
	atomic.AddInt64(&c.value, delta)
	return nil
}

func (c *AtomicCounter) Reset() {
	atomic.StoreInt64(&c.value, 0)
}

func (c *AtomicCounter) Get() int64 {
	return atomic.LoadInt64(&c.value)
}

// GetAndReset atomically reads the current value and resets to zero.
// Uses CAS loop to guarantee no increment is lost between read and reset.
//
// Why CAS loop instead of simple StoreInt64(0):
//   Without CAS: read=100, concurrent increment makes it 101, Store(0) loses that +1.
//   With CAS: if value changed between read and swap, retry — captures all increments.
//
// This is the correct pattern for "collect metrics and reset" use cases.
func (c *AtomicCounter) GetAndReset() int64 {
	for {
		current := atomic.LoadInt64(&c.value)
		if atomic.CompareAndSwapInt64(&c.value, current, 0) {
			return current
		}
		// CAS failed — another goroutine modified value between Load and CAS.
		// Retry with the new current value. Under normal load, this loop
		// executes once. Under extreme contention, a few retries at most.
	}
}