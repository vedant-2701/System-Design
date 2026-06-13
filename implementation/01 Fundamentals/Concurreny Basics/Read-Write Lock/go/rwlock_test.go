package rwlock

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// -------------------------------------------------------------------------
// 1. Basic Correctness
// -------------------------------------------------------------------------

func TestSingleReaderAcquireRelease(t *testing.T) {
	rw := NewReadWriteLock()
	rw.LockRead()

	state := rw.Snapshot()
	if state.ActiveReaders != 1 {
		t.Fatalf("expected 1 active reader, got %d", state.ActiveReaders)
	}
	if state.IsWriting {
		t.Fatal("IsWriting should be false while reader holds lock")
	}

	rw.UnlockRead()

	state = rw.Snapshot()
	if state.ActiveReaders != 0 {
		t.Fatalf("expected 0 active readers after unlock, got %d", state.ActiveReaders)
	}
}

func TestSingleWriterAcquireRelease(t *testing.T) {
	rw := NewReadWriteLock()
	rw.LockWrite()

	state := rw.Snapshot()
	if !state.IsWriting {
		t.Fatal("IsWriting should be true while writer holds lock")
	}
	if state.ActiveReaders != 0 {
		t.Fatalf("expected 0 active readers during write, got %d", state.ActiveReaders)
	}

	rw.UnlockWrite()

	state = rw.Snapshot()
	if state.IsWriting {
		t.Fatal("IsWriting should be false after write unlock")
	}
}

func TestMultipleReadersSimultaneous(t *testing.T) {
	rw := NewReadWriteLock()
	const numReaders = 10

	var wg sync.WaitGroup
	barrier := make(chan struct{})
	maxConcurrent := int32(0)

	for i := 0; i < numReaders; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			rw.LockRead()
			<-barrier // wait until all readers have acquired
			current := int32(rw.Snapshot().ActiveReaders)
			if current > atomic.LoadInt32(&maxConcurrent) {
				atomic.StoreInt32(&maxConcurrent, current)
			}
			time.Sleep(10 * time.Millisecond)
			rw.UnlockRead()
		}()
	}

	// Give all goroutines time to acquire
	time.Sleep(50 * time.Millisecond)
	close(barrier)
	wg.Wait()

	if atomic.LoadInt32(&maxConcurrent) < int32(numReaders/2) {
		t.Errorf("expected multiple concurrent readers, max observed: %d", maxConcurrent)
	}
}

// -------------------------------------------------------------------------
// 2. Concurrency Correctness — Writer Exclusivity
// -------------------------------------------------------------------------

func TestWriterExcludesReaders(t *testing.T) {
	rw := NewReadWriteLock()
	var events []string
	var mu sync.Mutex

	addEvent := func(e string) {
		mu.Lock()
		events = append(events, e)
		mu.Unlock()
	}

	writerHolding := make(chan struct{})
	writerRelease := make(chan struct{})
	readerDone := make(chan struct{})

	go func() {
		rw.LockWrite()
		addEvent("writer-acquired")
		close(writerHolding)
		<-writerRelease
		addEvent("writer-releasing")
		rw.UnlockWrite()
	}()

	go func() {
		<-writerHolding
		rw.LockRead()
		addEvent("reader-acquired")
		rw.UnlockRead()
		close(readerDone)
	}()

	time.Sleep(50 * time.Millisecond)
	close(writerRelease)

	select {
	case <-readerDone:
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for reader")
	}

	mu.Lock()
	defer mu.Unlock()
	expected := []string{"writer-acquired", "writer-releasing", "reader-acquired"}
	for i, e := range expected {
		if i >= len(events) || events[i] != e {
			t.Fatalf("event sequence wrong: expected %v, got %v", expected, events)
		}
	}
}

func TestWritersExclusiveOfEachOther(t *testing.T) {
	rw := NewReadWriteLock()
	var events []string
	var mu sync.Mutex

	addEvent := func(e string) {
		mu.Lock()
		events = append(events, e)
		mu.Unlock()
	}

	w1Holding := make(chan struct{})
	w1Release := make(chan struct{})
	bothDone := make(chan struct{})

	go func() {
		rw.LockWrite()
		addEvent("w1-acquired")
		close(w1Holding)
		<-w1Release
		addEvent("w1-releasing")
		rw.UnlockWrite()
	}()

	go func() {
		<-w1Holding
		rw.LockWrite()
		addEvent("w2-acquired")
		rw.UnlockWrite()
		close(bothDone)
	}()

	time.Sleep(50 * time.Millisecond)
	close(w1Release)

	select {
	case <-bothDone:
	case <-time.After(3 * time.Second):
		t.Fatal("timed out")
	}

	expected := []string{"w1-acquired", "w1-releasing", "w2-acquired"}
	mu.Lock()
	defer mu.Unlock()
	for i, e := range expected {
		if i >= len(events) || events[i] != e {
			t.Fatalf("expected %v, got %v", expected, events)
		}
	}
}

// -------------------------------------------------------------------------
// 3. Starvation Prevention
// -------------------------------------------------------------------------

func TestWriterNotStarved(t *testing.T) {
	rw := NewReadWriteLock()
	stop := int32(0)
	writerAcquired := make(chan struct{})

	// Flood of readers
	for i := 0; i < 10; i++ {
		go func() {
			for atomic.LoadInt32(&stop) == 0 {
				rw.LockRead()
				time.Sleep(5 * time.Millisecond)
				rw.UnlockRead()
			}
		}()
	}

	time.Sleep(50 * time.Millisecond) // let readers establish

	go func() {
		rw.LockWrite()
		close(writerAcquired)
		rw.UnlockWrite()
	}()

	select {
	case <-writerAcquired:
		// success
	case <-time.After(5 * time.Second):
		t.Fatal("writer was starved — starvation prevention is broken")
	}

	atomic.StoreInt32(&stop, 1)
}

func TestWaitingWriterBlocksNewReaders(t *testing.T) {
	rw := NewReadWriteLock()

	// Reader 1 holds lock
	rw.LockRead()

	// Writer queues
	writerDone := make(chan struct{})
	go func() {
		rw.LockWrite()
		rw.UnlockWrite()
		close(writerDone)
	}()

	time.Sleep(50 * time.Millisecond) // writer should now be waiting

	state := rw.Snapshot()
	if state.WaitingWriters != 1 {
		t.Fatalf("expected 1 waiting writer, got %d", state.WaitingWriters)
	}

	// New reader must be blocked
	reader2Acquired := int32(0)
	go func() {
		rw.LockRead()
		atomic.StoreInt32(&reader2Acquired, 1)
		rw.UnlockRead()
	}()

	time.Sleep(50 * time.Millisecond)
	if atomic.LoadInt32(&reader2Acquired) == 1 {
		t.Fatal("new reader jumped ahead of waiting writer — starvation prevention broken")
	}

	// Release reader1 — writer proceeds, then reader2
	rw.UnlockRead()

	select {
	case <-writerDone:
	case <-time.After(2 * time.Second):
		t.Fatal("writer never acquired after reader released")
	}

	// After writer releases, reader2 should acquire
	time.Sleep(100 * time.Millisecond)
	if atomic.LoadInt32(&reader2Acquired) == 0 {
		t.Fatal("reader2 never acquired after writer completed")
	}
}

// -------------------------------------------------------------------------
// 4. Error Handling
// -------------------------------------------------------------------------

func TestUnlockReadPanicsWhenNotHeld(t *testing.T) {
	rw := NewReadWriteLock()
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic on UnlockRead without LockRead")
		}
	}()
	rw.UnlockRead()
}

func TestUnlockWritePanicsWhenNotHeld(t *testing.T) {
	rw := NewReadWriteLock()
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic on UnlockWrite without LockWrite")
		}
	}()
	rw.UnlockWrite()
}

// -------------------------------------------------------------------------
// 5. Race Detector Validation
// -------------------------------------------------------------------------

func TestNoDataRaceOnSharedValue(t *testing.T) {
	// Run with: go test -race
	// Validates that the lock actually protects shared state from data races.
	rw := NewReadWriteLock()
	sharedValue := 0
	const iterations = 1000

	var wg sync.WaitGroup

	// Readers
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				rw.LockRead()
				_ = sharedValue // read
				rw.UnlockRead()
			}
		}()
	}

	// Writer
	wg.Add(1)
	go func() {
		defer wg.Done()
		for j := 0; j < iterations; j++ {
			rw.LockWrite()
			sharedValue++ // write
			rw.UnlockWrite()
		}
	}()

	wg.Wait()

	if sharedValue != iterations {
		t.Fatalf("expected sharedValue=%d, got %d (lost writes)", iterations, sharedValue)
	}
}