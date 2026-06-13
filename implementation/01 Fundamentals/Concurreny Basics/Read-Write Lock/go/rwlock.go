// Package rwlock provides a Read-Write Lock with writer starvation prevention.
//
// Multiple goroutines may hold the read lock simultaneously.
// A write lock is exclusive — no readers or other writers may hold it concurrently.
//
// Writer starvation prevention: once a writer is waiting, new readers block
// until the writer acquires and releases the lock.
//
// This implementation is intentionally written from scratch to demonstrate
// the underlying mechanics. In production, prefer sync.RWMutex which is
// optimised by the Go runtime. Use this when you need custom fairness
// policies or observability into lock state.
package rwlock

import (
	"fmt"
	"sync"
	"time"
)

// LockState is a snapshot of internal lock state.
// Intended for debugging, logging, and testing only.
// Not guaranteed to be consistent after the call returns.
type LockState struct {
	ActiveReaders  int
	WaitingWriters int
	IsWriting      bool
}

func (s LockState) String() string {
	return fmt.Sprintf(
		"LockState{activeReaders=%d, waitingWriters=%d, isWriting=%v}",
		s.ActiveReaders, s.WaitingWriters, s.IsWriting,
	)
}

// ReadWriteLock is a Read-Write Lock with writer starvation prevention.
//
// Acquisition rules:
//
//	Reader acquires when: !isWriting && waitingWriters == 0
//	Writer acquires when: !isWriting && activeReaders == 0
//
// Zero value is not usable — use NewReadWriteLock().
type ReadWriteLock struct {
	mu   sync.Mutex
	cond *sync.Cond

	activeReaders  int
	waitingWriters int
	isWriting      bool
}

// NewReadWriteLock returns an initialised ReadWriteLock.
func NewReadWriteLock() *ReadWriteLock {
	rw := &ReadWriteLock{}
	rw.cond = sync.NewCond(&rw.mu)
	return rw
}

// LockRead acquires the read lock.
// Blocks if a writer holds the lock or writers are waiting.
// Returns when the lock is acquired.
func (rw *ReadWriteLock) LockRead() {
	rw.mu.Lock()
	defer rw.mu.Unlock()

	// Block while a writer is active or writers are queued.
	// Loop — not if — to handle spurious wakeups from Broadcast/Signal.
	for rw.isWriting || rw.waitingWriters > 0 {
		rw.cond.Wait()
	}
	rw.activeReaders++
}

// TryLockRead attempts to acquire the read lock within the given timeout.
// Returns true if acquired, false if the timeout elapsed.
func (rw *ReadWriteLock) TryLockRead(timeout time.Duration) bool {
	acquired := make(chan struct{}, 1)

	go func() {
		rw.LockRead()
		acquired <- struct{}{}
	}()

	select {
	case <-acquired:
		return true
	case <-time.After(timeout):
		// Note: the goroutine may still acquire the lock after timeout.
		// In production, a cancellable context-based approach is preferable.
		// This implementation demonstrates the timeout concept simply.
		return false
	}
}

// UnlockRead releases the read lock.
// Panics if no read lock is currently held.
func (rw *ReadWriteLock) UnlockRead() {
	rw.mu.Lock()
	defer rw.mu.Unlock()

	if rw.activeReaders <= 0 {
		panic("rwlock: UnlockRead called but no read lock is held")
	}

	rw.activeReaders--

	// Last reader leaving — signal a waiting writer if any.
	// No need to broadcast to readers; they are blocked by waitingWriters > 0.
	if rw.activeReaders == 0 && rw.waitingWriters > 0 {
		rw.cond.Signal() // one writer proceeds — Signal not Broadcast
	}
}

// LockWrite acquires the write lock.
// Blocks until all active readers finish and no other writer holds the lock.
func (rw *ReadWriteLock) LockWrite() {
	rw.mu.Lock()
	defer rw.mu.Unlock()

	// Register intent before waiting — blocks new readers immediately.
	rw.waitingWriters++

	for rw.isWriting || rw.activeReaders > 0 {
		rw.cond.Wait()
	}

	// Successfully exited wait loop — transition to writing.
	// Decrement waitingWriters: we are no longer waiting, we are writing.
	rw.waitingWriters--
	rw.isWriting = true
}

// UnlockWrite releases the write lock.
// Panics if the write lock is not currently held.
func (rw *ReadWriteLock) UnlockWrite() {
	rw.mu.Lock()
	defer rw.mu.Unlock()

	if !rw.isWriting {
		panic("rwlock: UnlockWrite called but write lock is not held")
	}

	rw.isWriting = false

	if rw.waitingWriters > 0 {
		// Prefer queued writers — prevents newly arriving readers
		// from starving the writer queue.
		rw.cond.Signal()
	} else {
		// No writers waiting — wake all blocked readers.
		rw.cond.Broadcast()
	}
}

// Snapshot returns a copy of internal lock state.
// Not guaranteed to be consistent after the call returns.
// Intended for logging, debugging, and testing only.
func (rw *ReadWriteLock) Snapshot() LockState {
	rw.mu.Lock()
	defer rw.mu.Unlock()
	return LockState{
		ActiveReaders:  rw.activeReaders,
		WaitingWriters: rw.waitingWriters,
		IsWriting:      rw.isWriting,
	}
}