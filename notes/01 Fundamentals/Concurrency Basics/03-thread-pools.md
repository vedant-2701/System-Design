# Thread Pools

## Tags
#concurrency #backend-engineering #performance #scalability

---

## Overview
- Fixed set of reusable worker threads — created once at startup, shared across all tasks
- Solves thread explosion: unbounded thread-per-request creates memory exhaustion and scheduler overhead
- Tasks submitted to a bounded queue; workers pull and execute them
- Backpressure enforced via bounded queue + rejection policies

---

## Why Thread-Per-Request Fails at Scale

Creating a new thread per request costs:
- **Stack memory**: ~1MB–8MB per thread (OS default)
- **Kernel registration**: system call overhead per creation
- **Scheduler overhead**: OS manages N threads — context switching grows with N
- **Thread explosion**: 10,000 concurrent requests = potentially 10GB of stack memory

At scale, threads become the bottleneck, not the business logic.

---

## Architecture

```
Incoming Tasks
      ↓
┌─────────────────────────────────────┐
│           Thread Pool               │
│                                     │
│  [Worker 1] [Worker 2] [Worker 3]   │
│                                     │
│  ┌─────────────────────────────┐    │
│  │   Bounded Task Queue        │    │
│  │  [T1] [T2] [T3] [T4] ...   │    │
│  └─────────────────────────────┘    │
└─────────────────────────────────────┘
      ↓ (queue full)
  Rejection Policy
```

---

## Key Parameters

| Parameter | Purpose |
|---|---|
| Core pool size | Minimum threads kept alive, even when idle |
| Max pool size | Upper bound — threads created on demand up to this |
| Queue size | How many tasks can wait — **must be bounded** |
| Keep-alive time | How long extra threads survive when idle |
| Rejection policy | What happens when queue is full |

```java
// Java — production example
new ThreadPoolExecutor(
    10,                              // core pool size
    100,                             // max pool size
    60L, TimeUnit.SECONDS,           // keep-alive
    new ArrayBlockingQueue<>(500),   // bounded queue
    new CallerRunsPolicy()           // rejection policy
);
```

### Rejection Policies
| Policy | Behavior |
|---|---|
| AbortPolicy | Throw exception (default) |
| CallerRunsPolicy | Caller thread executes the task — natural backpressure |
| DiscardPolicy | Silently drop task |
| DiscardOldestPolicy | Drop oldest queued task, retry new one |

---

## Pool Sizing

### CPU-Bound Tasks
```
Optimal threads ≈ number of CPU cores (or cores + 1)
```
More threads than cores → context switching overhead with zero throughput gain. CPU is always the bottleneck.

The `+1` buffer handles minor stalls (GC pause, OS interruption) without leaving a core idle.

### I/O-Bound Tasks
```
Optimal threads = cores × (1 + wait_time / compute_time)
```
Thread spends most time waiting — more threads keep cores productive during waits.

### Little's Law for Sizing
```
Required threads = Throughput (req/s) × Average latency (s)

Example: 100 req/s × 0.2s latency = 20 threads minimum
```

---

## Unbounded Queue — the Hidden DOS Vector

A thread pool with unbounded queue is still vulnerable:
```
Pool: 10 threads, Queue: unlimited
Attacker sends 1,000,000 requests
Queue grows to 1,000,000 tasks
Memory exhausted → OOM crash
```
Production systems **must** use bounded queues with explicit rejection. Rejecting gracefully under overload is intentional design — not failure. This is [[Backpressure]].

---

## Tradeoffs

| | |
|---|---|
| Benefit | Bounded memory — threads created once |
| Benefit | Reduced context switching vs thread-per-request |
| Benefit | Backpressure via bounded queue |
| Cost | Fixed pool may underutilize resources under low load |
| Cost | Wrong sizing kills throughput (too small) or adds overhead (too large) |
| Risk | Unbounded queue silently enables memory exhaustion |

---

## Failure Scenarios
- **Pool too small** → queue fills → tasks rejected → degraded throughput
- **Pool too large for CPU-bound work** → context switching overhead → throughput decreases
- **Unbounded queue** → memory exhaustion under sustained overload
- **All workers blocked on I/O** → new tasks queue indefinitely → latency spikes
- **CallerRunsPolicy under overload** → HTTP server thread processes task → accepts no new connections

---

## Real-World Usage
- Java `ExecutorService` / `ThreadPoolExecutor` — standard production thread pools
- Python `concurrent.futures.ThreadPoolExecutor`
- Nginx worker processes — fixed count, not per-request
- Database connection pools (PgBouncer) — same bounded reuse pattern applied to connections
- Tomcat/Jetty HTTP thread pools — bounded, with queue and rejection

---

## Common Mistakes
- Setting pool size to an arbitrary large number "to handle more load"
- Using unbounded queue — appears fine until sustained overload causes OOM
- Applying CPU-bound sizing to I/O-bound workloads and vice versa
- Not measuring with Little's Law — sizing by intuition instead of math
- Ignoring rejection policies — default AbortPolicy throws exceptions that may be swallowed silently

---

## Interview Perspective
- Pool sizing question almost always appears in system design and backend interviews
- Expected: distinguish CPU-bound vs I/O-bound sizing strategies
- Follow-up: "what happens if the queue fills up?" → bounded queue + rejection policy + backpressure
- Little's Law is impressive to mention — shows quantitative reasoning
- Common trap: "set pool to 200 for more throughput on 8-core CPU-bound service" → explain why this hurts

---

## Revision Summary
- Thread-per-request fails at scale — memory, scheduler overhead, context switching
- Thread pool: fixed workers + bounded queue + rejection policy
- CPU-bound → pool size ≈ cores. I/O-bound → cores × (1 + wait/compute)
- Little's Law: threads needed = throughput × latency
- Unbounded queues are a silent OOM risk — always bound them
- Rejection policy = backpressure mechanism at the pool level

---

## Active Recall Questions
1. Why does thread-per-request fail at 10,000 concurrent connections?
2. You have a CPU-bound transcoding service on an 8-core machine. Team suggests pool size 200. What's wrong and what's correct?
3. What is Little's Law and how do you apply it to thread pool sizing?
4. What happens when a bounded queue fills up? Walk through each rejection policy.
5. Why must production thread pools use bounded queues?
6. An I/O-bound service has 4 cores, each request takes 100ms with 90ms spent waiting on DB. What pool size?

---

## Related Concepts
- [[Mutex, Semaphore, Spinlock]]
- [[Concurrency Models — Threads vs Event Loop vs Async Await]]
- [[Amdahl's Law]]
- [[Backpressure]]
- [[Connection Pooling]]
