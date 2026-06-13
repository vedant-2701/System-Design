# Thread-Safe Bounded Blocking Queue

## Problem Statement

A concurrent system with producers generating work faster than consumers can process it
needs a buffer. Without bounding that buffer, memory grows unboundedly — a slow consumer
under sustained load causes an OOM crash. The queue must also coordinate blocking and
unblocking of producers and consumers efficiently without busy-waiting.

---

## Design Overview

```
Producers                    Queue                    Consumers
─────────                ─────────────               ─────────
Thread A ──► offer() ──► [T1][T2][T3] ──► take() ──► Worker 1
Thread B ──► offer() ──► [bounded    ] ──► take() ──► Worker 2
Thread C ──► BLOCKS  ──► [capacity=3 ]               Worker 3
              (full)
```

### Core Invariants
- At most `capacity` items exist in the queue at any time
- A producer blocks (or times out) when the queue is full
- A consumer blocks (or times out) when the queue is empty
- No items are lost, duplicated, or reordered (FIFO)
- Shutdown is graceful — existing items drain; new items are rejected

---

## Why Two Semaphores + Mutex (Java)

Three concurrency primitives with distinct responsibilities:

| Primitive | Initialized To | Acquired By | Meaning |
|---|---|---|---|
| `availableSlots` | capacity | Producers | "I claim one empty slot" |
| `availableItems` | 0 | Consumers | "I claim one filled slot" |
| `mutex` | 1 (binary) | Both | "I have exclusive array access" |

**Critical acquisition order** (deadlock prevention via lock ordering):
```
Producer: availableSlots → mutex → insert → release mutex → release availableItems
Consumer: availableItems → mutex → remove → release mutex → release availableSlots
```

Reversing this order (mutex before semaphore) causes deadlock: a producer holding the
mutex blocks on `availableSlots` (queue full), while the consumer needs the mutex to
remove an item. Neither can proceed.

### Why Not ReentrantLock + two Conditions?

Option A (Lock + Conditions) is equally correct. Semaphores were chosen because:
- Each semaphore directly *represents* the resource it models (slot count, item count)
- Responsibilities are separated: semaphores handle coordination, mutex handles exclusion
- Easier to reason about for producers/consumers independently

---

## Why Buffered Channel (Go)

Go's buffered channel is not merely a convenience — it *is* the bounded blocking queue:

```go
ch := make(chan T, capacity)

// Producer blocks when full:
ch <- item

// Consumer blocks when empty:
item := <-ch
```

The Go runtime handles all synchronization internally, more efficiently than user-space
semaphores because the scheduler has full context about which goroutines are waiting.

Explicit semaphores are not needed because the channel semantics already encode:
- `len(ch)` = current items (equivalent to `availableItems` permits)
- `cap(ch) - len(ch)` = empty slots (equivalent to `availableSlots` permits)

What we add on top of the channel:
- Shutdown signaling via a `done` channel (closed on shutdown)
- `select` statements to react to either data availability *or* shutdown
- `sync.Once` to ensure `close(done)` happens exactly once (closing a closed channel panics)

---

## API Design Decisions

### Why Both `put/take` and `offer/poll`?

`put()` (indefinite block) is dangerous in production: if the consumer side crashes, the
producer thread blocks forever, holding stack memory (~1MB) doing nothing.

`offer(timeout)` is the recommended path. On timeout the caller can:
- Increment a `queue.rejected` metric
- Write to a dead-letter store
- Return HTTP 429 to upstream callers (backpressure propagation)

Both are exposed because some internal systems legitimately want indefinite blocking.
The interface makes the tradeoff explicit.

### Why Reject Null Items?

`poll()` returning `null` means timeout elapsed. If null items were permitted,
callers couldn't distinguish "got null item" from "timeout". Null is used as a
sentinel — items must never be null.

### Why `AtomicBoolean` for Shutdown (Java) / `atomic.Bool` (Go)?

Shutdown state is read on every `put()` call by potentially thousands of threads.
A full mutex for this read would create unnecessary contention. An atomic boolean
provides visibility without locking — the write happens once, reads happen constantly.

---

## Exponential Backoff Producer

Retry logic without backoff creates a retry storm: all rejected producers hammer the
queue simultaneously, making the congestion worse.

```
Attempt 1: wait 50ms
Attempt 2: wait 100ms
Attempt 3: wait 200ms
Attempt 4: wait 400ms
Attempt 5: give up → log warning → propagate failure upstream
```

Cap at `maxDelayMs` to prevent unbounded wait. In Go, `select + time.After` is preferred
over `time.Sleep` — context cancellation interrupts the wait immediately rather than
sleeping through it.

---

## Alternatives Considered

### Unbounded Queue
Rejected: slow consumers under sustained load → unbounded memory growth → OOM crash.
Backpressure cannot propagate upstream without a bounded structure.

### Lock-Free Queue (CAS-based)
Correct but complex. CAS-based queues require careful ABA handling and are significantly
harder to reason about. The semaphore approach is correct, readable, and fast enough for
all practical throughput requirements. Optimize only when profiling proves it necessary.

### Single Lock (synchronized everything)
Simpler but serializes producers and consumers unnecessarily. A producer inserting and
a consumer removing could proceed concurrently — they only conflict on the same array
slot, not on the operation itself. Two semaphores enable this concurrency.

---

## Java vs Go — Conceptual Mapping

| Concept | Java | Go |
|---|---|---|
| Bounded buffer | `ArrayDeque` + `Semaphore(capacity)` | `make(chan T, capacity)` |
| Block on full | `availableSlots.acquire()` | `ch <- item` (select) |
| Block on empty | `availableItems.acquire()` | `item := <-ch` (select) |
| Mutual exclusion | `Semaphore(1)` (fair mutex) | Implicit in channel runtime |
| Shutdown signal | `AtomicBoolean` + `IllegalStateException` | `close(done)` channel |
| Timeout | `tryAcquire(timeout, unit)` | `context.WithTimeout` + `select` |
| Idempotent shutdown | Manual flag check | `sync.Once` + `close(done)` |
| Generics | `BoundedQueue<T>` | `BoundedQueue[T any]` |

Key difference: Go's channel encapsulates what Java requires three primitives for.
The Go implementation is shorter not because it's less correct, but because the
runtime provides more built-in concurrency primitives.

---

## Complexity

| Operation | Time | Notes |
|---|---|---|
| `put` / `offer` | O(1) | Semaphore acquire + array append |
| `take` / `poll` | O(1) | Semaphore acquire + array remove |
| `size` | O(1) | Semaphore permit count / channel length |
| `shutdown` | O(1) | Atomic write / channel close |

Space: O(capacity) — bounded by design.

---

## Edge Cases Handled

| Case | Handling |
|---|---|
| Queue full, producer calls `put` | Blocks until consumer removes an item |
| Queue empty, consumer calls `take` | Blocks until producer inserts an item |
| `offer` timeout elapses | Returns false / ErrQueueFull — no item inserted |
| `poll` timeout elapses | Returns null / ErrQueueEmpty — no item removed |
| `put` after `shutdown` | Throws `IllegalStateException` / returns `ErrQueueShutdown` |
| `take` after `shutdown` (items remain) | Returns remaining items, then shutdown error |
| Null item inserted | Rejected immediately — null is used as sentinel for timeout |
| Zero/negative capacity | Constructor rejects it — fail fast |
| Thread interrupted during block | `InterruptedException` propagated — thread interrupt status preserved |
| Multiple `shutdown()` calls | Idempotent — `sync.Once` / `AtomicBoolean` CAS |

---

## Production Considerations

**Observability**: instrument these metrics:
- `queue.size` (gauge) — track sustained fullness
- `queue.offer.timeout` (counter) — indicates consumer lag
- `queue.rejected` (counter) — signals need to scale consumers or increase capacity
- `queue.put.latency` (histogram) — p99 latency under backpressure

**Sizing**: apply Little's Law:
```
Required capacity = throughput (items/s) × acceptable latency (s)
Example: 1000 items/s × 0.5s acceptable buffer = 500 capacity
```

**Graceful shutdown sequence** (application level):
```
1. Stop accepting new work (shutdown queue)
2. Wait up to 30s for consumers to drain (awaitTermination)
3. If still running → force shutdown (shutdownNow / context cancel)
4. Log remaining undrained items as dropped — alert on this
```

---

## Future Improvements

- **Priority queue**: replace `ArrayDeque` with `PriorityQueue` — higher-priority items
  dequeued first. Requires `T extends Comparable` in Java, `T` with ordering in Go.
- **Multiple queues with work stealing**: idle consumers steal from busy queues —
  improves throughput when work is unevenly distributed.
- **Metrics integration**: pass a metrics registry to the constructor — emit size,
  wait time, and rejection rate without caller instrumentation.
- **Batch operations**: `drainTo(collection, maxItems)` for bulk consumer throughput.
