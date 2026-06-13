# Producer-Consumer Queue

A thread-safe, bounded producer-consumer queue implemented in Java and Go.
Covers semaphores, backpressure, graceful shutdown, and concurrency correctness.

---

## Problem Statement

Multiple producer threads generate items. Multiple consumer threads process them.
A shared buffer decouples production rate from consumption rate.

Three constraints must hold simultaneously:
- **Bounded**: buffer never exceeds capacity N — producers must block when full
- **Backpressure**: producers experience real blocking pressure, not silent drops
- **Correct shutdown**: consumers drain remaining items before exiting — no items lost

---

## Design Overview

```
Producer 1 ──┐
Producer 2 ──┤──→ [ BoundedBuffer (capacity N) ] ──→ Consumer 1
Producer 3 ──┘                                   └──→ Consumer 2
                                                  └──→ Consumer 3
```

### Core components

| Component | Responsibility |
|---|---|
| `BoundedBuffer` | Circular array + semaphores + mutex. Enforces capacity and blocks correctly. |
| `Producer` | Pulls from a supplier, pushes to buffer with timeout-based backpressure. |
| `ConsumerWorker` | Pulls from buffer, passes to handler. Exits on poison pill (Java) or context cancel (Go). |
| `ProducerConsumerSystem` | Orchestrates thread lifecycle, shutdown sequencing, stats logging. |

---

## Critical Invariant — Semaphore Before Lock

The most important ordering rule in the entire implementation:

```
// CORRECT
semaphore.acquire()   // may block — outside the lock
lock.lock()
  // mutate buffer
lock.unlock()
semaphore.release()

// DEADLOCK
lock.lock()           // lock held
semaphore.acquire()   // blocks here — lock never released — other threads can't proceed
```

Acquiring a lock before a semaphore means the thread holds the lock while potentially
blocking indefinitely. Other threads cannot acquire the lock to make progress
(e.g. insert an item). The system deadlocks.

**Rule: never block while holding a lock.**

---

## Two-Semaphore Pattern

One semaphore tracks available spaces (producers acquire).
One semaphore tracks available items (consumers acquire).
They are complementary: `spaces + items = N` at all times.

```
Initial state (capacity = 3):
  spaces = 3  ░░░  (3 empty slots)
  items  = 0       (0 items)

After 2 puts:
  spaces = 1  ░    (1 empty slot)
  items  = 2  ██   (2 items)

After 1 take:
  spaces = 2  ░░   (2 empty slots)
  items  = 1  █    (1 item)
```

---

## Shutdown Strategy

### Java — Poison Pill
```
1. Set shutdown flag → producers stop immediately (put() returns false)
2. Insert N poison pills into the buffer (one per consumer)
3. Each consumer dequeues a poison pill → exits its loop
4. All items already in the buffer are consumed before the pill arrives
```

Guarantees in-flight items are drained. Requires knowing consumer count at shutdown time.

### Go — Context Cancellation
```
1. cancel() called → context cancelled across all goroutines
2. Producers: Put() select on ctx.Done() → exit cleanly
3. Consumers: Take() select on ctx.Done() → exit cleanly
4. sync.WaitGroup confirms all goroutines exited
```

Idiomatic Go. Simpler than poison pills.
Trade-off: items remaining in buffer at cancellation time are NOT drained.
For drain-on-shutdown, cancel producers first, wait for buffer to empty, then cancel consumers.

---

## Why Array, Not Linked List

Buffer capacity is fixed and known at construction time.

| | Circular Array | Linked List |
|---|---|---|
| Per-item allocation | None — slots pre-allocated | One node object per item |
| GC pressure | None | High under sustained load |
| Cache locality | Excellent — contiguous memory | Poor — scattered heap nodes |
| Memory overhead | Exactly N slots | N nodes + pointer per node |

For bounded buffers with known capacity: array always wins.

---

## Language Design Differences

| Concern | Java | Go |
|---|---|---|
| Semaphore | `java.util.concurrent.Semaphore` | Buffered channel of `struct{}` |
| Mutex | `ReentrantLock(fair=true)` | `sync.Mutex` |
| Shutdown signal | Poison pill (null sentinel) | `context.Context` cancellation |
| Observability | `AtomicLong` counters | `sync/atomic` int64 |
| Type safety | Generics (`BoundedBuffer<T>`) | Generics (`BoundedBuffer[T]`) |
| Thread/goroutine | `ExecutorService` + `Runnable` | `go func()` + `sync.WaitGroup` |

### Channels as Semaphores (Go)

A buffered channel of capacity N behaves exactly like a counting semaphore initialized to N:
- `ch <- struct{}{}` (send) = acquire (blocks when channel is full)
- `<-ch` (receive) = release (blocks when channel is empty)

This is idiomatic Go — no explicit semaphore type needed.

---

## Alternatives Considered

### Java `LinkedBlockingQueue` directly
Java's standard library provides `LinkedBlockingQueue` which implements this pattern internally.
We implement from scratch here to understand the semaphore mechanics directly.
In production Java, prefer `ArrayBlockingQueue` (bounded, array-backed, lower overhead).

### Go channels directly
A single buffered channel (`make(chan T, N)`) implements bounded producer-consumer natively in Go.
We implement with explicit semaphores + mutex to demonstrate the underlying mechanics
and to support the two-semaphore pattern explicitly.
In production Go, a buffered channel is usually the right answer.

### Read-Write Lock instead of Mutex
Considered but rejected. The buffer hot path has no pure reads — every operation
(insert or remove) modifies head, tail, or buffer contents. A read-write lock
provides no benefit and adds complexity.

---

## Edge Cases

| Case | Handling |
|---|---|
| `put(null)` | `IllegalArgumentException` thrown immediately (Java) |
| Capacity ≤ 0 | `IllegalArgumentException` / error at construction |
| Interrupt during block | `InterruptedException` propagated, interrupt flag restored |
| Shutdown before start | `put()` returns false immediately; `take()` returns poison pill |
| Consumer handler throws | Error logged and counted; consumer continues (not killed) |
| Supplier throws | Error logged and counted; producer continues (not killed) |

---

## Production Considerations

**Dead-letter queue**: handler errors currently log and continue.
Production systems should route failed items to a DLQ for inspection and retry.

**Metrics**: `producedCount`, `rejectedCount`, `consumedCount`, `errorCount` expose
queue health. Wire these to Prometheus counters in production.

**Rejection rate as backpressure signal**: sustained high `rejectedCount` means
consumers are too slow — scale out consumers or reduce producer rate.

**Fair locks**: both Java (`ReentrantLock(true)`) and Go mutex are fair enough
for this use case. Java's fair semaphore prevents producer starvation under
heavy consumer load.

---

## Running the Code

### Go
```bash
cd go
go test -v -race ./...          # all tests with race detector
go test -run TestConcurrent -v  # specific test
```

### Java
```bash
cd java
mvn test                        # requires JUnit 5 in pom.xml
# or with your IDE's test runner
```

---

## Further Improvements

- **Priority queue**: replace circular array with a heap — highest-priority items consumed first
- **Metrics endpoint**: expose buffer depth, rejection rate, throughput as Prometheus metrics
- **Dynamic consumer scaling**: spin up additional consumers when buffer depth exceeds a threshold
- **Distributed extension**: replace in-memory buffer with Redis list or Kafka topic — same producer/consumer contract, distributed backing store
