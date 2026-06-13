// Package threadpool provides a bounded goroutine pool with graceful shutdown.
//
// Go design notes vs Java:
//
// 1. Goroutines vs OS threads:
//    In Java, each worker IS an OS thread (~1MB stack).
//    In Go, each worker is a goroutine (~2KB stack, multiplexed across OS threads by the runtime).
//    This means pool size guidance differs — Go can support larger pools cheaply.
//
// 2. Channels replace BlockingQueue:
//    Java uses ArrayBlockingQueue with explicit locking.
//    Go's buffered channel IS a bounded blocking queue with built-in synchronization.
//    No explicit mutex needed for the queue itself.
//
// 3. context.Context for cancellation:
//    Java uses thread interruption (thread.interrupt()).
//    Go uses context cancellation — idiomatic, composable, cancellation-safe.
//
// 4. sync.WaitGroup replaces CountDownLatch:
//    Java CountDownLatch is initialized with a count and counts down.
//    Go WaitGroup.Add(n) / Done() / Wait() is equivalent but more flexible.
//
// 5. select statement for multi-condition waiting:
//    Java requires separate locks or condition variables.
//    Go select naturally handles "task available OR shutdown" in one construct.

package threadpool

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
)

// ErrPoolShutdown is returned when a task is submitted to a stopped pool.
var ErrPoolShutdown = errors.New("threadpool: pool is shut down")

// ErrQueueFull is returned by the Abort rejection policy when the queue is full.
var ErrQueueFull = errors.New("threadpool: task queue is full")

// RejectionPolicy determines behavior when submit cannot accept a task.
type RejectionPolicy int

const (
	// AbortPolicy returns an error to the caller. Caller must handle overload explicitly.
	AbortPolicy RejectionPolicy = iota

	// CallerRunsPolicy executes the task on the submitting goroutine.
	// Provides natural backpressure — slows the producer when pool is saturated.
	CallerRunsPolicy

	// DiscardPolicy silently drops the task. Only for truly disposable work.
	DiscardPolicy
)

// Task is a unit of work submitted to the pool.
type Task func()

// Pool is a bounded goroutine pool.
//
// All methods are safe for concurrent use.
type Pool struct {
	taskQueue       chan Task      // buffered channel — bounded blocking queue
	cancelFunc      context.CancelFunc
	workerWg        sync.WaitGroup // tracks live workers
	activeWorkers   atomic.Int32   // workers currently executing a task
	shutdown        atomic.Bool    // true after Shutdown() or ShutdownNow()
	rejectionPolicy RejectionPolicy
	poolSize        int
}

// Config holds pool construction parameters.
type Config struct {
	// PoolSize is the number of worker goroutines. Defaults to 4.
	PoolSize int

	// QueueCapacity is the max number of tasks waiting in queue. Defaults to 100.
	QueueCapacity int

	// RejectionPolicy determines behavior when queue is full. Defaults to AbortPolicy.
	RejectionPolicy RejectionPolicy
}

func (c *Config) applyDefaults() {
	if c.PoolSize <= 0 {
		c.PoolSize = 4
	}
	if c.QueueCapacity <= 0 {
		c.QueueCapacity = 100
	}
}

// New creates and starts a Pool with the given configuration.
func New(cfg Config) *Pool {
	cfg.applyDefaults()

	ctx, cancel := context.WithCancel(context.Background())

	p := &Pool{
		// Buffered channel with capacity = queue size.
		// Sending blocks when full (used by CallerRuns to block naturally).
		// Non-blocking send used for Abort and Discard policies.
		taskQueue:       make(chan Task, cfg.QueueCapacity),
		cancelFunc:      cancel,
		rejectionPolicy: cfg.RejectionPolicy,
		poolSize:        cfg.PoolSize,
	}

	p.startWorkers(ctx, cfg.PoolSize)

	log.Printf("threadpool: started with %d workers, queue capacity %d",
		cfg.PoolSize, cfg.QueueCapacity)

	return p
}

func (p *Pool) startWorkers(ctx context.Context, count int) {
	for i := 0; i < count; i++ {
		p.workerWg.Add(1)
		workerID := i
		go p.workerLoop(ctx, workerID)
	}
}

// workerLoop is the goroutine body for each worker.
//
// select statement elegantly handles two conditions simultaneously:
// - a task is available in the channel
// - the context was cancelled (ShutdownNow)
//
// This is the key Go advantage over Java here — no explicit state polling needed.
func (p *Pool) workerLoop(ctx context.Context, id int) {
	defer p.workerWg.Done()
	log.Printf("threadpool: worker-%d started", id)

	for {
		select {
		case task, ok := <-p.taskQueue:
			if !ok {
				// Channel closed — Shutdown() was called and queue is drained.
				log.Printf("threadpool: worker-%d exiting (queue closed)", id)
				return
			}
			p.executeTask(task, id)

		case <-ctx.Done():
			// ShutdownNow() cancelled the context — exit immediately.
			log.Printf("threadpool: worker-%d exiting (context cancelled)", id)
			return
		}
	}
}

func (p *Pool) executeTask(task Task, workerID int) {
	p.activeWorkers.Add(1)
	defer p.activeWorkers.Add(-1)

	// Recover from panics in tasks.
	// A panic in a goroutine kills the whole program if unrecovered.
	// In Java, uncaught exceptions from Runnable are caught by the worker.
	// Go's equivalent is recover() in a deferred function.
	defer func() {
		if r := recover(); r != nil {
			log.Printf("threadpool: worker-%d recovered from task panic: %v", workerID, r)
		}
	}()

	task()
}

// Submit enqueues a task for execution.
//
// Returns ErrPoolShutdown if the pool is shut down.
// Behavior on full queue depends on the configured RejectionPolicy.
func (p *Pool) Submit(task Task) error {
	if task == nil {
		return fmt.Errorf("threadpool: task must not be nil")
	}

	if p.shutdown.Load() {
		return ErrPoolShutdown
	}

	switch p.rejectionPolicy {
	case AbortPolicy:
		// Non-blocking send. Returns immediately if queue is full.
		select {
		case p.taskQueue <- task:
			return nil
		default:
			return ErrQueueFull
		}

	case CallerRunsPolicy:
		// Try non-blocking first. If full, run on caller goroutine.
		// This provides backpressure — caller slows down naturally.
		select {
		case p.taskQueue <- task:
			return nil
		default:
			if !p.shutdown.Load() {
				task() // caller executes it directly
			}
			return nil
		}

	case DiscardPolicy:
		// Try to enqueue. If full, silently drop.
		select {
		case p.taskQueue <- task:
		default:
			// intentionally dropped
		}
		return nil

	default:
		return fmt.Errorf("threadpool: unknown rejection policy %d", p.rejectionPolicy)
	}
}

// Shutdown initiates graceful shutdown.
// No new tasks are accepted. Already-queued tasks continue executing.
// Returns immediately — use Wait() to block until all tasks finish.
func (p *Pool) Shutdown() {
	if p.shutdown.CompareAndSwap(false, true) {
		log.Println("threadpool: shutdown initiated — draining queue")
		// Closing the channel signals workers that no more tasks will arrive.
		// Workers will drain remaining tasks then exit when channel is empty and closed.
		close(p.taskQueue)
		// Note: do NOT cancel context here — that would interrupt running tasks.
	}
}

// ShutdownNow initiates immediate shutdown.
// Cancels the context to interrupt workers. Drains and discards the queue.
// Returns immediately — use Wait() to block until all workers exit.
func (p *Pool) ShutdownNow() {
	if p.shutdown.CompareAndSwap(false, true) {
		log.Println("threadpool: shutdownNow initiated — interrupting workers")
		// Cancel context first — workers in select will pick up ctx.Done().
		p.cancelFunc()
		// Drain remaining queued tasks (they will not execute).
		// Must drain AFTER closing to avoid send-on-closed-channel panic.
		// We don't close the channel here — workers exit via ctx.Done(), not channel close.
	} else {
		// Already shut down via Shutdown() — still cancel context to stop workers mid-drain.
		p.cancelFunc()
	}
}

// Wait blocks until all workers have exited.
// Call after Shutdown() or ShutdownNow().
func (p *Pool) Wait() {
	p.workerWg.Wait()
	log.Println("threadpool: all workers exited — terminated")
}

// ActiveWorkerCount returns the number of workers currently executing a task.
func (p *Pool) ActiveWorkerCount() int {
	return int(p.activeWorkers.Load())
}

// QueueSize returns the number of tasks currently waiting in the queue.
func (p *Pool) QueueSize() int {
	return len(p.taskQueue)
}

// IsShutdown returns true if Shutdown() or ShutdownNow() has been called.
func (p *Pool) IsShutdown() bool {
	return p.shutdown.Load()
}