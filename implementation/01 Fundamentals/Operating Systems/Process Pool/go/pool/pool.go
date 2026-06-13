// Package pool implements a process pool backed by OS processes
// communicating via pipes. Each worker is an isolated OS process —
// a crash in one worker cannot corrupt others or the pool manager.
//
// Architecture:
//
//	Pool Manager
//	├── Worker 0: taskPipe(write) → [pipe] → worker stdin
//	│            worker stdout → [pipe] → resultPipe(read)
//	├── Worker 1: same
//	└── Worker N: same
//
// The pool manager runs one goroutine per worker to read results.
// This avoids blocking on one slow worker while others finish —
// the same problem epoll solves for file descriptors, but expressed
// naturally in Go using goroutines and channels.
package pool

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Task represents a unit of work submitted to the pool.
type Task struct {
	ID      string
	Command string
}

// Result holds the outcome of a task execution.
type Result struct {
	TaskID  string
	Output  string
	Err     error
	// WorkerPID lets callers correlate results with workers —
	// useful for debugging which worker handled which task.
	WorkerPID int
}

// workerState tracks a single managed worker process.
// All fields except cmd/pid are owned by the pool manager goroutine —
// no mutex needed for those. The result channel is safe for concurrent
// use by design (single writer goroutine, single reader).
type workerState struct {
	pid       int
	cmd       *exec.Cmd

	// taskWriter is the write end of the pipe connected to worker stdin.
	// Pool manager writes tasks here. Worker reads from its stdin (fd 0).
	taskWriter *os.File

	// resultReader is the read end of the pipe connected to worker stdout.
	// A dedicated goroutine reads from this continuously.
	resultReader *os.File

	// results carries parsed results from the reader goroutine to the pool.
	// Buffered to prevent the reader goroutine blocking if pool is slow
	// to consume — avoids head-of-line blocking.
	results chan Result

	// busy indicates whether this worker currently has an outstanding task.
	// Accessed only from the pool's dispatch logic — no concurrent access.
	busy bool

	// crashCount tracks how many times this worker slot has been restarted.
	// Used for logging and to detect persistently failing workers.
	crashCount int
}

// Pool manages a fixed number of worker processes.
type Pool struct {
	workerBinaryPath string
	size             int
	workers          []*workerState

	// taskQueue buffers submitted tasks when all workers are busy.
	// Bounded to prevent unbounded memory growth under sustained load.
	taskQueue chan Task

	// results is the unified result channel exposed to callers.
	results chan Result

	// nextTaskID generates unique IDs without locks using atomic increment.
	nextTaskID atomic.Uint64

	// wg tracks active goroutines for clean shutdown.
	wg sync.WaitGroup

	// shutdown signals all goroutines to stop.
	shutdown chan struct{}

	mu sync.Mutex // protects workers slice during respawn
}

// New creates a pool of `size` worker processes.
// taskQueueDepth controls how many tasks buffer before Submit blocks.
func New(workerBinaryPath string, size int, taskQueueDepth int) (*Pool, error) {
	if size <= 0 {
		return nil, fmt.Errorf("pool size must be positive, got %d", size)
	}
	if taskQueueDepth <= 0 {
		taskQueueDepth = size * 4 // sensible default: 4x workers
	}

	p := &Pool{
		workerBinaryPath: workerBinaryPath,
		size:             size,
		workers:          make([]*workerState, size),
		taskQueue:        make(chan Task, taskQueueDepth),
		results:          make(chan Result, taskQueueDepth),
		shutdown:         make(chan struct{}),
	}

	for i := range size {
		w, err := p.spawnWorker(i)
		if err != nil {
			p.Stop() // clean up already-started workers
			return nil, fmt.Errorf("failed to spawn worker %d: %w", i, err)
		}
		p.workers[i] = w
	}

	p.wg.Add(1)
	go p.dispatchLoop()

	return p, nil
}

// spawnWorker forks a new worker process and wires up its pipes.
// This is the Go equivalent of: fork() + pipe() + dup2() + exec()
// that strace showed us — exec.Cmd handles those syscalls internally.
func (p *Pool) spawnWorker(slotIndex int) (*workerState, error) {
	// Create task pipe: pool manager writes, worker reads via stdin
	taskRead, taskWrite, err := os.Pipe()
	if err != nil {
		return nil, fmt.Errorf("pipe for task channel: %w", err)
	}

	// Create result pipe: worker writes via stdout, pool manager reads
	resultRead, resultWrite, err := os.Pipe()
	if err != nil {
		taskRead.Close()
		taskWrite.Close()
		return nil, fmt.Errorf("pipe for result channel: %w", err)
	}

	cmd := exec.Command(p.workerBinaryPath)

	// Wire pipes to worker's stdin and stdout.
	// exec.Cmd.Stdin/Stdout does the dup2() for us:
	//   taskRead  → worker's fd 0 (stdin)
	//   resultWrite → worker's fd 1 (stdout)
	// Worker's stderr (fd 2) inherits from pool manager — crash output
	// appears in pool manager's logs, which is exactly what we want.
	cmd.Stdin = taskRead
	cmd.Stdout = resultWrite
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		taskRead.Close()
		taskWrite.Close()
		resultRead.Close()
		resultWrite.Close()
		return nil, fmt.Errorf("start worker process: %w", err)
	}

	// Critical: close the ends we don't own in the pool manager process.
	//
	// If we keep taskRead open in the pool manager, the worker will never
	// see EOF on its stdin when we close taskWrite — because the read end
	// is still open somewhere. This is the classic pipe leak bug.
	//
	// Similarly, resultWrite must be closed here so we get EOF on
	// resultRead when the worker exits. Without this, our reader goroutine
	// blocks forever even after the worker dies.
	taskRead.Close()
	resultWrite.Close()

	w := &workerState{
		pid:          cmd.Process.Pid,
		cmd:          cmd,
		taskWriter:   taskWrite,
		resultReader: resultRead,
		results:      make(chan Result, 16),
	}

	// One goroutine per worker reads results continuously.
	// This is the goroutine-per-fd pattern — equivalent to epoll but
	// expressed naturally in Go. Each goroutine blocks on its pipe;
	// the Go scheduler multiplexes them onto OS threads efficiently.
	p.wg.Add(1)
	go p.readWorkerResults(w, slotIndex)

	log.Printf("pool: worker slot=%d pid=%d started", slotIndex, w.pid)
	return w, nil
}

// readWorkerResults reads result lines from a worker's stdout pipe.
// Runs as a dedicated goroutine per worker for its entire lifetime.
// When the worker exits (crash or shutdown), the pipe returns EOF
// and this goroutine triggers respawn logic.
func (p *Pool) readWorkerResults(w *workerState, slotIndex int) {
	defer p.wg.Done()

	reader := bufio.NewReader(w.resultReader)

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			// EOF or read error means worker's stdout pipe closed —
			// worker process has exited. Check if it was intentional.
			select {
			case <-p.shutdown:
				// Pool is shutting down — expected exit, do not respawn.
				w.resultReader.Close()
				return
			default:
				// Unexpected exit — worker crashed. Trigger respawn.
				log.Printf("pool: worker slot=%d pid=%d exited unexpectedly: %v",
					slotIndex, w.pid, err)
				w.resultReader.Close()
				p.handleWorkerCrash(slotIndex)
				return
			}
		}

		result, parseErr := parseResult(strings.TrimSpace(line), w.pid)
		if parseErr != nil {
			log.Printf("pool: worker pid=%d malformed result %q: %v",
				w.pid, line, parseErr)
			continue
		}

		// Forward result to the unified results channel.
		// Non-blocking send with select to avoid goroutine leak if
		// the caller has stopped consuming results.
		select {
		case p.results <- result:
		case <-p.shutdown:
			return
		}
	}
}

// handleWorkerCrash respawns a crashed worker.
// The crashed worker's slot is replaced transparently — callers
// never know a crash occurred except via logged output.
//
// Key concern: the task that was in-flight when the worker crashed
// is lost. Production pools would requeue it. We log the loss here
// to make the gap visible rather than silently dropping it.
func (p *Pool) handleWorkerCrash(slotIndex int) {
	p.mu.Lock()
	defer p.mu.Unlock()

	select {
	case <-p.shutdown:
		return
	default:
	}

	oldWorker := p.workers[slotIndex]

	// Brief backoff before respawn — prevents tight crash loops from
	// overwhelming the system if the worker binary itself is broken.
	crashCount := oldWorker.crashCount + 1
	backoff := time.Duration(crashCount*100) * time.Millisecond
	if backoff > 5*time.Second {
		backoff = 5 * time.Second
	}

	log.Printf("pool: respawning slot=%d after %v (crash #%d)",
		slotIndex, backoff, crashCount)
	time.Sleep(backoff)

	newWorker, err := p.spawnWorker(slotIndex)
	if err != nil {
		log.Printf("pool: FATAL failed to respawn slot=%d: %v", slotIndex, err)
		// In production: alert, circuit break, or fall back to
		// reduced pool size. Here we leave the slot nil and let
		// the dispatch loop skip it.
		p.workers[slotIndex] = nil
		return
	}

	newWorker.crashCount = crashCount
	p.workers[slotIndex] = newWorker
}

// dispatchLoop is the pool manager's main loop.
// It reads tasks from the queue and routes them to idle workers.
// Single goroutine — no locking needed on worker state.
func (p *Pool) dispatchLoop() {
	defer p.wg.Done()

	for {
		select {
		case <-p.shutdown:
			return

		case task := <-p.taskQueue:
			p.dispatchTask(task)
		}
	}
}

// dispatchTask finds an idle worker and sends a task to it.
// Blocks until a worker is available — provides natural backpressure.
// If all workers are busy, the caller's Submit() will block on
// taskQueue (which is bounded), propagating pressure upstream.
func (p *Pool) dispatchTask(task Task) {
	for {
		p.mu.Lock()
		for _, w := range p.workers {
			if w != nil && !w.busy {
				w.busy = true
				p.mu.Unlock()
				p.sendTaskToWorker(w, task)
				return
			}
		}
		p.mu.Unlock()

		// No idle worker — wait briefly before retrying.
		// In production: use a condition variable or idle worker channel
		// to avoid this polling. Kept simple here to avoid obscuring
		// the core IPC concepts.
		select {
		case <-p.shutdown:
			return
		case <-time.After(10 * time.Millisecond):
		}
	}
}

// sendTaskToWorker writes a task message to the worker's stdin pipe.
// The format matches what worker/main.go expects: "TASK:<id>:<cmd>\n"
func (p *Pool) sendTaskToWorker(w *workerState, task Task) {
	message := fmt.Sprintf("TASK:%s:%s\n", task.ID, task.Command)
	_, err := fmt.Fprint(w.taskWriter, message)
	if err != nil {
		// Write failed — worker's read end of pipe is closed (crashed).
		// The result reader goroutine will detect and handle the crash.
		// Mark worker not busy so dispatch loop doesn't skip it forever.
		p.mu.Lock()
		w.busy = false
		p.mu.Unlock()
		log.Printf("pool: failed to send task %s to worker pid=%d: %v",
			task.ID, w.pid, err)
	}
}

// Submit enqueues a task for execution and returns its assigned ID.
// Blocks if the task queue is full — provides backpressure to callers.
// Returns error if the pool is shut down.
func (p *Pool) Submit(command string) (taskID string, err error) {
	id := fmt.Sprintf("%d", p.nextTaskID.Add(1))

	task := Task{ID: id, Command: command}

	select {
	case p.taskQueue <- task:
		return id, nil
	case <-p.shutdown:
		return "", fmt.Errorf("pool is shut down")
	}
}

// Results returns the channel on which completed results arrive.
// Callers must consume from this channel continuously — if it fills,
// the result reader goroutines will block, eventually stalling workers.
func (p *Pool) Results() <-chan Result {
	return p.results
}

// Stop initiates graceful shutdown.
// Sends SHUTDOWN to each worker, waits for all goroutines to exit,
// then closes all pipe file descriptors.
func (p *Pool) Stop() {
	close(p.shutdown)

	p.mu.Lock()
	for _, w := range p.workers {
		if w == nil {
			continue
		}
		// Polite shutdown: tell worker to exit cleanly.
		// If write fails, worker already exited — that's fine.
		fmt.Fprint(w.taskWriter, "SHUTDOWN\n")
		w.taskWriter.Close()
	}
	p.mu.Unlock()

	// Wait for all reader goroutines and dispatch loop to exit.
	p.wg.Wait()

	close(p.results)
	log.Printf("pool: shutdown complete")
}

// parseResult parses a result line from worker stdout.
// Format: "RESULT:<id>:OK:<output>" or "RESULT:<id>:ERR:<msg>|<output>"
func parseResult(line string, workerPID int) (Result, error) {
	if !strings.HasPrefix(line, "RESULT:") {
		return Result{}, fmt.Errorf("missing RESULT: prefix in %q", line)
	}

	rest := strings.TrimPrefix(line, "RESULT:")
	parts := strings.SplitN(rest, ":", 3)
	if len(parts) != 3 {
		return Result{}, fmt.Errorf("malformed result line: %q", line)
	}

	taskID := parts[0]
	status := parts[1]
	payload := parts[2]

	// Restore newlines that worker sanitized for transport
	output := strings.ReplaceAll(payload, "\\n", "\n")

	result := Result{TaskID: taskID, WorkerPID: workerPID}

	switch status {
	case "OK":
		result.Output = output
	case "ERR":
		// payload is "errMsg|commandOutput"
		subparts := strings.SplitN(output, "|", 2)
		result.Err = fmt.Errorf("%s", subparts[0])
		if len(subparts) == 2 {
			result.Output = subparts[1]
		}
	default:
		return Result{}, fmt.Errorf("unknown status %q in result line", status)
	}

	return result, nil
}

// markWorkerIdle marks a worker available after its result has been received.
// Called from the result consumer — not from the reader goroutine —
// because we want the worker marked idle only after the caller has
// acknowledged the result, not just when we received it internally.
func (p *Pool) MarkWorkerIdleByPID(pid int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, w := range p.workers {
		if w != nil && w.pid == pid {
			w.busy = false
			return
		}
	}
}

// workerCount returns the number of currently alive workers.
// Useful for health checks and monitoring.
func (p *Pool) WorkerCount() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	count := 0
	for _, w := range p.workers {
		if w != nil {
			count++
		}
	}
	return count
}

// readWorkerResults needs access to io.EOF for pipe close detection —
// importing here to make the dependency explicit.
var _ = io.EOF