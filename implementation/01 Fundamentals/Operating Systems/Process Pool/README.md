# Process Pool

## Problem Statement

A process pool maintains a fixed set of pre-forked worker processes ready to execute tasks. Instead of spawning a new process per task — paying fork+exec cost each time — the pool amortizes that cost across many tasks by keeping workers alive and reusing them.

This pattern appears in:
- **Web servers** (Nginx, Apache prefork MPM) — pre-forked workers handle HTTP requests
- **Job runners** (Celery, Sidekiq) — worker pools consume from a task queue
- **Database connection proxies** (PgBouncer) — process pool fronts database connections
- **Build systems** (make -j, Bazel) — parallel compilation via worker processes
- **Shell command execution services** — safely execute user-submitted commands in isolated processes

The key motivation over threads: **a process crash cannot corrupt the pool manager or other workers**. Each worker has its own address space. Isolation is enforced by the OS, not by convention.

---

## Design Overview

### Components

```text
┌─────────────────────────────────────────────────────┐
│                    Pool Manager                     │
│                                                     │
│  taskQueue (buffered chan)                          │
│       ↓                                             │
│  dispatchLoop goroutine                             │
│       ↓ finds idle worker                           │
│  sendTaskToWorker()                                 │
│                                                     │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐           │
│  │ Worker 0 │  │ Worker 1 │  │ Worker N │           │
│  │ pid=907  │  │ pid=909  │  │ pid=911  │           │
│  └──────────┘  └──────────┘  └──────────┘           │
│                                                     │
│  resultReaderGoroutine × N                          │
│       ↓ forwards to                                 │
│  results (unified chan) → caller                    │
└─────────────────────────────────────────────────────┘
```

### Per-Worker IPC

Each worker has exactly two pipes:

```text
Pool Manager                          Worker Process
─────────────────────────────────────────────────────
taskWriter (fd W) ──[task pipe]──→ stdin (fd 0)
                                   reads "TASK:<id>:<cmd>\n"
                                   executes via sh -c
                                   writes "RESULT:<id>:OK:<out>\n"
resultReader (fd R) ←─[result pipe]── stdout (fd 1)
```

Stderr (fd 2) is inherited from the pool manager — worker crash output appears directly in pool manager logs.

### Wire Protocol

Line-delimited text over pipes. Chosen for simplicity and debuggability — you can manually cat to/from a worker pipe during debugging.

```text
Task message:   "TASK:<id>:<command>\n"
Result (ok):    "RESULT:<id>:OK:<output>\n"
Result (error): "RESULT:<id>:ERR:<errMsg>|<output>\n"
Shutdown:       "SHUTDOWN\n"
EOF on task pipe signals worker to exit (crash recovery path)
```

Newlines in output are escaped as `\n` before transport and restored after parsing — preserves the line-delimited framing invariant.

### Goroutine Responsibilities

| Goroutine | Count | Responsibility |
|---|---|---|
| `dispatchLoop` | 1 | Reads taskQueue, finds idle worker, sends task |
| `readWorkerResults` | 1 per worker | Blocks on worker's result pipe, forwards results, detects crashes |
| Caller goroutines | N | Submit tasks, consume Results() channel |

The result reader goroutines are the equivalent of `epoll_wait` on result pipes — each blocks on its own fd, the Go scheduler multiplexes them onto OS threads without the pool manager polling.

---

## Why This Approach Was Chosen

### Processes over Threads

Threads share memory — a bug in one worker can corrupt shared state in another or in the pool manager. Processes have isolated address spaces enforced by the MMU. A worker that segfaults, corrupts its heap, or receives SIGKILL does not affect the pool manager or other workers. For command execution workloads (untrusted or unpredictable commands), this isolation is not optional.

### Dedicated Pipes over Shared Pipe

A single shared task pipe (all workers reading from one pipe) would distribute tasks via Linux's pipe read atomicity — but breaks result routing: the pool manager cannot know which worker handled which task, making it impossible to route results back or track worker state. Dedicated pipes per worker add `O(N)` file descriptors but enable precise dispatch, result correlation, and crash detection.

### One Goroutine per Worker for Result Reading

The alternative — using `select` or `epoll` across all result pipes — is correct but more complex in Go. One goroutine per pipe maps naturally to Go's scheduler: goroutines are cheap (4KB initial stack), blocking a goroutine on a pipe read does not block an OS thread. The Go runtime's netpoller integrates with the OS to multiplex blocked goroutines efficiently. This gives us the performance of epoll with the readability of sequential code.

### Line-Delimited Text Protocol

Binary length-prefixed framing (e.g. 4-byte length header + payload) would be more efficient and handle embedded newlines naturally. It was not chosen here because: text protocol is human-readable during debugging, inspectable with `cat`, and sufficient for command output. The newline-escaping tradeoff is acceptable at this scale.

### Atomic Task ID Generation

```go
p.nextTaskID.Add(1)  // sync/atomic — no mutex
```

Task IDs are generated with an atomic increment rather than a mutex-protected counter. Atomic operations are implemented as CPU instructions (LOCK XADD on x86) — no kernel involvement, no goroutine blocking. At high submission rates, a mutex here would become a bottleneck. Atomics eliminate that contention entirely.

---

## Alternatives Considered

### Spawn Per Task (rejected)

Fork + exec per task. Simple — no pool management code. But pays full process startup cost (~10ms on Linux including dynamic linking) per task. At 1000 tasks/sec this is 10 seconds of startup overhead per second of work. Viable only for infrequent, long-running tasks (cron-style jobs).

### Thread Pool instead of Process Pool (not chosen for this use case)

Threads share memory — significantly lower IPC overhead (shared heap instead of pipes), lower context switch cost (no TLB flush), faster spawn. The correct choice for CPU-bound parallelism within a trusted codebase. Wrong choice when workers execute untrusted commands — a worker thread crashing with a segfault kills the entire process.

### Shared Memory IPC (rejected)

Could replace pipes with a shared memory ring buffer for zero-copy task dispatch. Lower latency, higher throughput. But requires explicit synchronization (mutex or CAS), complex crash recovery (worker crash with a held mutex corrupts the ring buffer), and OS-specific APIs (`shmget`/`mmap`). Pipes give us implicit synchronization (the kernel manages the pipe buffer), clean EOF-based crash detection, and portable APIs. For a command execution pool the pipe overhead is irrelevant compared to task execution time.

### Single Shared Result Channel (rejected)

Instead of per-worker result goroutines, use one goroutine that polls all workers with `select`. Reduces goroutine count but introduces polling latency and coupling between workers — a slow result read from one worker delays checking others. Per-worker goroutines give true independence.

---

## Complexity Analysis

| Operation | Complexity | Notes |
|---|---|---|
| Task submission | O(1) | Channel send — amortized constant |
| Task dispatch | O(N) per dispatch | Linear scan for idle worker — acceptable for small N |
| Result delivery | O(1) | Channel send from reader goroutine |
| Worker spawn | O(1) | fork + exec — constant cost, paid once per worker slot |
| Crash detection | O(1) | EOF on result pipe — detected immediately by reader goroutine |
| Pool shutdown | O(N) | Send SHUTDOWN to each worker, wait for goroutines |

**Memory:**
- Per-worker: 2 pipe file descriptors (kernel buffers ~64KB each), 1 goroutine stack (~4KB initial), result channel buffer
- Total for N workers: O(N) goroutines, O(N) file descriptors, O(N × pipe buffer) kernel memory

**Scalability limit:** The O(N) dispatch scan becomes meaningful at N > 100 workers. Replace with an idle worker channel for O(1) dispatch at larger pool sizes.

---

## Edge Cases and Failure Scenarios

### Worker Crash Mid-Task

**What happens:** Worker process killed (SIGSEGV, OOM, SIGKILL). Write end of result pipe closes. `readWorkerResults` goroutine gets EOF from `reader.ReadString()`. Goroutine logs the crash, calls `handleWorkerCrash`, exits.

**What is lost:** The in-flight task. No result is ever sent for it. The caller's result collection will time out waiting for that task ID.

**Production fix:** Track `currentTask *Task` per worker. On crash, requeue `w.currentTask` back to `taskQueue` before respawning.

### Worker Crash Loop

**Mitigation:** Exponential backoff in `handleWorkerCrash`:
```go
backoff := time.Duration(crashCount*100) * time.Millisecond  // 100ms, 200ms, 300ms...
if backoff > 5*time.Second { backoff = 5*time.Second }
```
Prevents a broken worker binary from spinning the CPU. After repeated failures, the slot is left nil and skipped by dispatch — pool continues with reduced capacity rather than failing entirely.

### Pipe Write Blocked

If the worker's task pipe buffer fills (64KB kernel buffer — roughly 64KB worth of task messages), `sendTaskToWorker`'s `fmt.Fprint` blocks. This propagates backpressure: dispatch loop blocks → taskQueue fills → `Submit()` blocks on channel send → caller blocks. This is correct behavior — backpressure should propagate upstream rather than dropping tasks silently.

### Pool Manager Crash

All workers receive EOF on their stdin pipes (pool manager held the write ends). Workers detect EOF in their read loop and exit cleanly. No orphan processes — the OS delivers EOF automatically when the write end of a pipe closes, regardless of whether the writer crashed or exited normally.

### Malformed Worker Output

`parseResult()` returns an error on malformed lines. The reader goroutine logs the error and continues — it does not crash or stop reading. The corresponding task result is never delivered to the caller (who will time out). This is preferable to crashing the reader goroutine, which would trigger spurious crash recovery.

### File Descriptor Exhaustion

Each worker consumes 4 file descriptors in the pool manager (2 pipe ends per direction × 2 directions, minus the ends closed after spawn = 2 remaining per worker). At N=1000 workers, that's 2000 fds — within default Linux limits (1024) only up to ~512 workers. Raise `ulimit -n` for large pools.

---

## Production Considerations

### Observability

The current implementation uses `log.Printf` throughout. In production:
- Replace with structured logging (JSON) with fields: `worker_slot`, `worker_pid`, `task_id`, `crash_count`
- Expose metrics: tasks submitted, tasks completed, tasks failed, tasks in-flight, worker crash count, active worker count
- A crash rate metric is the primary health signal for this system

### Graceful Shutdown

Current implementation sends `SHUTDOWN\n` to each worker, then waits for goroutines via `sync.WaitGroup`. This does not drain in-flight tasks — tasks in `taskQueue` at shutdown time are dropped. Production fix: drain `taskQueue` before sending SHUTDOWN, or implement a `Drain()` method that blocks until the queue is empty.

### Task Timeouts

No per-task deadline is implemented. A `sleep 9999` command holds a worker indefinitely. Production fix:
```go
ctx, cancel := context.WithTimeout(context.Background(), taskTimeout)
defer cancel()
cmd := exec.CommandContext(ctx, "sh", "-c", command)
```
`exec.CommandContext` sends SIGKILL to the child when the context expires.

### Health Checks

`WorkerCount()` returns the number of alive worker slots. Expose this via an HTTP `/health` endpoint. Alert when `WorkerCount() < poolSize` — means some workers are crash-looping.

### Resource Limits on Workers

Workers run arbitrary shell commands. In production, constrain them:
- `cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}` — put worker in its own process group, kill entire group on timeout
- cgroups for CPU and memory limits per worker
- seccomp profile to restrict which syscalls workers can make

### Requeue on Crash

Implement task requeue in `handleWorkerCrash`:
```go
if oldWorker.currentTask != nil {
    select {
    case p.taskQueue <- *oldWorker.currentTask:
        log.Printf("pool: requeued task %s after worker crash", oldWorker.currentTask.ID)
    default:
        log.Printf("pool: dropped task %s — queue full after crash", oldWorker.currentTask.ID)
    }
}
```

---

## Known Limitations

| Limitation | Impact | Fix |
|---|---|---|
| O(N) idle worker scan | Latency grows with pool size | Use `idleWorkers chan *workerState` |
| In-flight task lost on crash | Silent task drop | Track currentTask, requeue on crash |
| No task timeout | Worker held indefinitely | `exec.CommandContext` with deadline |
| taskQueue drained on shutdown | Tasks dropped at shutdown | Drain queue before sending SHUTDOWN |
| Caller must call MarkWorkerIdleByPID | Leaky abstraction | Auto-mark idle on result send |
| No task deduplication | Duplicate tasks possible | Add idempotency key check |

---

## Future Improvements

- **Priority queue** — replace `taskQueue chan Task` with a heap-based priority queue for task prioritization
- **Dynamic pool sizing** — shrink pool when idle, grow under load, bounded by min/max config
- **Result streaming** — for long-running commands, stream partial output rather than buffering entire output
- **Binary protocol** — replace line-delimited text with length-prefixed binary for embedded newline support and lower parsing overhead  
- **Metrics integration** — Prometheus counters for task throughput, worker crash rate, queue depth, task latency histogram
- **Multi-machine pool** — replace pipes with gRPC streams; same pool manager interface, distributed workers
