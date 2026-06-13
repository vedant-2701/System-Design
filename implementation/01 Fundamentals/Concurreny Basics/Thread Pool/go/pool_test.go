package threadpool

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// Tests mirror the Java test suite — same scenarios, Go idioms.
//
// Go testing differences from Java/JUnit:
// - t.Fatal() stops the current test (like assertion failure)
// - sync.WaitGroup instead of CountDownLatch
// - goroutines instead of threads for concurrent submission
// - time.Sleep used sparingly — prefer synchronization primitives

func TestSubmittedTasksExecute(t *testing.T) {
	pool := New(Config{PoolSize: 2, QueueCapacity: 10})
	defer pool.Shutdown()

	var wg sync.WaitGroup
	var executedCount atomic.Int32

	for i := 0; i < 5; i++ {
		wg.Add(1)
		pool.Submit(func() {
			executedCount.Add(1)
			wg.Done()
		})
	}

	waitWithTimeout(t, &wg, 3*time.Second, "all tasks should execute")

	if executedCount.Load() != 5 {
		t.Fatalf("expected 5 executed, got %d", executedCount.Load())
	}
}

func TestConcurrentSubmissionsAllExecute(t *testing.T) {
	pool := New(Config{PoolSize: 4, QueueCapacity: 200})
	defer pool.Shutdown()

	var allExecuted sync.WaitGroup
	var executedCount atomic.Int32
	taskCount := 100

	allExecuted.Add(taskCount)

	// 10 goroutines each submit 10 tasks simultaneously
	var submitterWg sync.WaitGroup
	var startGun sync.WaitGroup
	startGun.Add(1)

	for t := 0; t < 10; t++ {
		submitterWg.Add(1)
		go func() {
			defer submitterWg.Done()
			startGun.Wait() // all goroutines start at same time
			for i := 0; i < 10; i++ {
				pool.Submit(func() {
					executedCount.Add(1)
					allExecuted.Done()
				})
			}
		}()
	}

	startGun.Done() // release all submitters
	waitWithTimeout(t, &allExecuted, 5*time.Second, "all concurrently submitted tasks should execute")

	if int(executedCount.Load()) != taskCount {
		t.Fatalf("expected %d executed, got %d", taskCount, executedCount.Load())
	}
}

func TestWorkerSurvivesPanic(t *testing.T) {
	// Go equivalent of Java's "worker survives task exception" test.
	// In Go, tasks can panic. Worker must recover and continue.
	pool := New(Config{PoolSize: 1, QueueCapacity: 10})
	defer pool.Shutdown()

	var wg sync.WaitGroup
	var executedAfterPanic atomic.Int32

	// First task panics
	wg.Add(1)
	pool.Submit(func() {
		wg.Done()
		panic("intentional task panic")
	})
	wg.Wait()

	// Short sleep to let recovery complete before next task
	time.Sleep(50 * time.Millisecond)

	// Second task should still execute
	wg.Add(1)
	pool.Submit(func() {
		executedAfterPanic.Add(1)
		wg.Done()
	})

	waitWithTimeout(t, &wg, 3*time.Second, "worker should survive panic and execute next task")

	if executedAfterPanic.Load() != 1 {
		t.Fatal("worker did not survive task panic")
	}
}

func TestShutdownDrainsQueue(t *testing.T) {
	pool := New(Config{PoolSize: 1, QueueCapacity: 20})

	var executedCount atomic.Int32
	blockWorker := make(chan struct{})

	// Block the single worker
	pool.Submit(func() {
		<-blockWorker
	})

	// Queue 5 more tasks
	for i := 0; i < 5; i++ {
		pool.Submit(func() {
			executedCount.Add(1)
		})
	}

	pool.Shutdown()

	// Submit after shutdown should fail
	err := pool.Submit(func() {})
	if err != ErrPoolShutdown {
		t.Fatalf("expected ErrPoolShutdown, got %v", err)
	}

	close(blockWorker) // unblock worker

	// Wait with timeout
	done := make(chan struct{})
	go func() {
		pool.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("pool did not terminate after draining queue")
	}

	if executedCount.Load() != 5 {
		t.Fatalf("expected 5 tasks executed after shutdown, got %d", executedCount.Load())
	}
}

func TestShutdownNowExitsIdleWorkers(t *testing.T) {
	// Important Go limitation vs Java:
	// context cancellation only interrupts workers blocked in select (idle workers).
	// A worker mid-task running time.Sleep() or pure computation is NOT interrupted —
	// Go has no equivalent of Java's Thread.interrupt() for arbitrary goroutines.
	//
	// Production implication: tasks that may run long should accept a context.Context
	// and check ctx.Done() internally to support cooperative cancellation.
	//
	// This test verifies: idle workers exit promptly on ShutdownNow.

	pool := New(Config{PoolSize: 3, QueueCapacity: 10})
	// Submit no tasks — all workers are idle in select

	pool.ShutdownNow()

	done := make(chan struct{})
	go func() {
		pool.Wait()
		close(done)
	}()

	select {
	case <-done:
		// All idle workers exited via ctx.Done() — correct
	case <-time.After(2 * time.Second):
		t.Fatal("idle workers should exit promptly on ShutdownNow")
	}
}

func TestShutdownNowDoesNotWaitForMidTaskWorker(t *testing.T) {
	// Demonstrates the cooperative cancellation limitation.
	// Worker mid-task with no context check will finish its task before exiting.
	// This is expected Go behavior — not a bug.

	pool := New(Config{PoolSize: 1, QueueCapacity: 5})

	taskStarted := make(chan struct{})
	taskAllowedToFinish := make(chan struct{})

	pool.Submit(func() {
		close(taskStarted)
		<-taskAllowedToFinish // cooperative wait — responds when we unblock it
	})

	<-taskStarted
	pool.ShutdownNow()

	// Worker is mid-task. Pool.Wait() will block until task finishes.
	// Unblock the task — simulates task completing after shutdown.
	close(taskAllowedToFinish)

	done := make(chan struct{})
	go func() {
		pool.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("pool should terminate after mid-task goroutine finishes")
	}
}

func TestAbortPolicyOnFullQueue(t *testing.T) {
	pool := New(Config{
		PoolSize:        1,
		QueueCapacity:   1,
		RejectionPolicy: AbortPolicy,
	})
	defer func() { pool.ShutdownNow(); pool.Wait() }()

	blockWorker := make(chan struct{})
	pool.Submit(func() { <-blockWorker }) // occupy worker
	pool.Submit(func() {})                // fill queue

	err := pool.Submit(func() {})
	if err != ErrQueueFull {
		t.Fatalf("expected ErrQueueFull, got %v", err)
	}

	close(blockWorker)
}

func TestCallerRunsPolicyExecutesOnCallerGoroutine(t *testing.T) {
	pool := New(Config{
		PoolSize:        1,
		QueueCapacity:   1,
		RejectionPolicy: CallerRunsPolicy,
	})
	defer func() { pool.ShutdownNow(); pool.Wait() }()

	blockWorker := make(chan struct{})
	pool.Submit(func() { <-blockWorker }) // occupy worker
	pool.Submit(func() {})                // fill queue

	// CallerRuns: task should execute synchronously before Submit returns
	executed := false
	pool.Submit(func() {
		executed = true
	})

	// If CallerRuns worked, executed is true immediately after Submit returns
	if !executed {
		t.Fatal("CallerRunsPolicy should have executed task on caller goroutine")
	}

	close(blockWorker)
}

func TestSubmitNilTaskReturnsError(t *testing.T) {
	pool := New(Config{PoolSize: 2})
	defer pool.Shutdown()

	err := pool.Submit(nil)
	if err == nil {
		t.Fatal("submitting nil task should return an error")
	}
}

// waitWithTimeout blocks on wg.Wait() with a timeout.
// Fails the test if timeout elapses first.
func waitWithTimeout(t *testing.T, wg *sync.WaitGroup, timeout time.Duration, msg string) {
	t.Helper()
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(timeout):
		t.Fatalf("timeout after %v: %s", timeout, msg)
	}
}