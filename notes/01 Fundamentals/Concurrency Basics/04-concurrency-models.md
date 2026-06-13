# Concurrency Models — Threads vs Event Loop vs Async/Await

## Tags
#concurrency #backend-engineering #performance #nodejs #async

---

## Overview
- Three models for keeping CPUs productive while waiting on I/O
- They solve the same problem differently: what does a thread do while waiting 50ms for a DB response?
- Choosing the wrong model for the workload type causes catastrophic throughput failure
- Most production systems use **hybrid architectures** — not a single model

---

## The Core Problem
```
Thread makes DB call → waits 50ms → CPU core sits idle
50ms = ~150,000,000 CPU instructions wasted
```
Concurrency models exist to fill that idle time with productive work.

---

## Model 1 — Threads

### How It Works
Multiple OS threads. While Thread A waits for I/O, Thread B runs on CPU.

### Characteristics
- True parallelism across multiple cores
- Each thread has its own stack (~1MB–8MB)
- OS scheduler manages execution
- Blocking I/O is fine — other threads run

### When to Use
- CPU-bound work — image processing, video transcoding, computation
- True parallelism needed across cores
- Short-lived blocking acceptable

### Critical Limit
- 10,000 threads = potentially 10GB stack memory
- Context switching overhead grows with thread count
- Does not scale for I/O-bound workloads with high concurrency

---

## Model 2 — Event Loop

### How It Works
Single thread. Never blocks. I/O operations register callbacks — execution continues immediately.

```
Request arrives
    ↓
Register DB callback → continue immediately
    ↓
[other events processed]
    ↓
DB response arrives → callback executes
```

```javascript
// Node.js — non-blocking
db.query("SELECT ...", (result) => {
    response.send(result);  // runs when response arrives
});
// execution continues here immediately
```

### Why It Scales for I/O-Bound Work
- Single thread — zero context switching overhead
- Memory per connection: ~KB (callback/closure) vs ~MB (thread stack)
- OS notifies event loop when I/O completes (epoll/kqueue)
- One thread handles 10,000 concurrent connections

### The Catastrophic Failure Mode
CPU-bound work blocks the event loop thread:
```javascript
app.get('/compute', (req, res) => {
    const result = heavyComputation();  // blocks for 5 seconds
    res.send(result);
});
// ALL other requests queue for 5 seconds
```
The single thread is fully occupied. Server freezes. Fix: offload CPU work to worker threads.

### When to Use
- I/O-bound workloads — HTTP servers, API gateways, real-time connections
- High connection concurrency with mostly waiting
- Never for CPU-heavy computation

---

## Model 3 — Async/Await

### What It Is
Syntactic sugar over the event loop model. Makes async code look synchronous without blocking.

```python
async def handle_request():
    result = await db.query("SELECT ...")  # suspends here
    return result                           # resumes when ready
```

### How It Works
When execution hits `await`:
1. Current coroutine **suspends** — saves its state in user space
2. Event loop picks another ready coroutine to run
3. When awaited operation completes, original coroutine **resumes**

No OS thread blocked. No context switch. Suspension is user-space only.

### Parallelism — Language Dependent
| Runtime | True Parallelism? | Notes |
|---|---|---|
| Python asyncio | No | GIL limits to one thread |
| Node.js | No | Single-threaded JS runtime |
| Go (goroutines) | Yes | Multiplexed across OS threads by runtime |
| Java virtual threads | Yes | Project Loom — cheap threads with real parallelism |

### When to Use
- Same as event loop — I/O-bound, high concurrency
- Cleaner code than callback-based event loop (no callback hell)
- Not a substitute for threads on CPU-bound work

---

## Comparison

| | Threads | Event Loop | Async/Await |
|---|---|---|---|
| Concurrency unit | OS Thread | Callback/Event | Coroutine |
| Memory per connection | ~1MB | ~KB | ~KB |
| Blocking behavior | Blocks OS thread | Never blocks | Suspends coroutine |
| CPU-bound work | Good | **Catastrophic** | **Catastrophic** |
| I/O-bound work | Good | Excellent | Excellent |
| True parallelism | Yes | No | Runtime-dependent |
| Code complexity | Simple | Callback hell | Simple (looks sync) |

---

## Hybrid Architecture — Production Reality

Most production services combine models:

```
┌─────────────────────────────────────┐
│   Async HTTP Server (Event Loop)    │  ← receives connections cheaply
│         ↓                           │
│   Async/Await Coroutines            │  ← I/O: DB, Redis, Kafka calls
│         ↓                           │
│   CPU Thread Pool (N = cores)       │  ← CPU-bound: validation, compute
└─────────────────────────────────────┘
```

Examples:
- **I/O-bound API** (auth service, CRUD): async/await throughout
- **CPU-bound service** (image resize, video transcode): thread pool for processing, async HTTP layer for receiving
- **Node.js with CPU work**: `worker_threads` for computation, event loop for I/O
- **Python with CPU work**: `asyncio.run_in_executor()` offloads to thread pool

---

## Failure Scenarios
- **CPU work in event loop** → entire server freezes per request → catastrophic for all concurrent users
- **Thread-per-connection for I/O-bound** → memory exhaustion at scale → OOM under high concurrency
- **Missing await** → coroutine returns immediately without waiting → silent correctness bug
- **Python async with CPU work** → GIL still blocks → asyncio doesn't help → need multiprocessing

---

## Real-World Usage
- **Node.js** — event loop for web servers (Express, Fastify) — handles massive I/O concurrency
- **Go** — goroutines with real parallelism — standard for high-throughput services
- **Python FastAPI** — async/await with uvicorn — I/O-bound APIs
- **Java Netty** — event loop for non-blocking I/O
- **Java Project Loom** — virtual threads — cheap blocking with real parallelism

---

## Common Mistakes
- Running CPU-bound work on the event loop thread
- Assuming async/await gives parallelism in Python (GIL prevents it)
- Using threads for 10,000 concurrent I/O-bound connections
- Not offloading CPU work to worker threads in Node.js
- Mixing blocking I/O calls inside async coroutines — defeats the model

---

## Interview Perspective
- Classic question: "design a high-concurrency HTTP server — threads or event loop?"
- Must distinguish CPU-bound vs I/O-bound and choose accordingly
- Hybrid architecture answer shows senior-level thinking
- "What's wrong with running image processing in Node.js event loop?" → blocks all other requests
- Go vs Node.js comparison: Go has true parallelism, Node.js is single-threaded

---

## Revision Summary
- Threads: true parallelism, expensive memory, good for CPU-bound
- Event loop: single thread, never blocks, excellent for I/O-bound, catastrophic for CPU-bound
- Async/await: coroutines, user-space suspension, same use case as event loop
- CPU-bound → thread pool sized to core count
- I/O-bound → async/await or event loop
- Production = hybrid: async HTTP layer + CPU thread pool for compute
- Go/Java virtual threads give async-level efficiency with real parallelism

---

## Active Recall Questions
1. Why does an event loop handle 10,000 concurrent I/O connections efficiently on a single thread?
2. What happens when CPU-bound work runs inside a Node.js event loop?
3. Does Python async/await give you parallelism? Why or why not?
4. Design the concurrency model for: (a) auth API with 3 DB calls per request, (b) image resize service at 1000 RPS
5. What is the difference between a coroutine suspending and an OS thread blocking?
6. Why do Go goroutines scale better than Java threads (pre-Loom)?

---

## Related Concepts
- [[Thread Pools]]
- [[Amdahl's Law]]
- [[Memory Visibility and Happens-Before]]
- [[Backpressure]]
- [[Non-Blocking I/O]]
