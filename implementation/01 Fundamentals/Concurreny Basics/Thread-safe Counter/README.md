# Thread-Safe Counter

## Problem Statement

A counter shared across multiple threads/goroutines appears trivial but is a canonical concurrency problem. The operation `counter++` is not atomic — it compiles to three CPU instructions. Under concurrent access, threads interleave these instructions and lose updates silently.

This appears in production as:
- Request counters in rate limiters
- Metric collection (request counts, error counts, latency histograms)
- Inventory deduction systems
- Sequence ID generation

The goal is to implement a counter that produces the correct value regardless of how many threads access it concurrently.

---

## Design Overview

Three implementations with increasing sophistication:

```
UnsafeCounter    — demonstrates the race condition; not for production
MutexCounter     — correct; uses OS-backed mutual exclusion
AtomicCounter    — correct; lock-free via CPU-level CAS instruction
```

All three implement the same `Counter` interface. The interface exposes **behavior only** — increment, decrement, get, reset. The synchronization mechanism is hidden from callers. Callers never call `lock()` or `unlock()` — that would break encapsulation and create deadlock risk.

---

## Why This Approach Was Chosen

### Interface-first design
The `Counter` interface allows swapping implementations without changing callers. In tests, you can inject `MutexCounter` or `AtomicCounter` and verify both behave identically. In production, you pick the implementation appropriate for your contention profile.

### Three implementations instead of one
A single "correct" implementation would hide the reasoning. The unsafe version exists to make the race condition observable and testable. Understanding why it breaks is more valuable than just knowing what to use.

### `long` / `int64` instead of `int`
At 50,000 requests per second, a 32-bit int overflows in ~12 hours. A 64-bit value overflows in ~292 billion years. Production counters always use 64-bit integers.

---

## Implementation Details

### MutexCounter

```
lock acquired
  value++ (read-modify-write — now serialized)
lock released
happens-before established → next acquirer sees updated value
```

The `finally` block in Java / `defer` in Go is critical. Without it, an exception or panic inside the critical section leaves the lock permanently acquired. Every subsequent caller blocks forever — effective deadlock.

`get()` also acquires the lock. Without it, on 32-bit JVMs, reading a `long` is not atomic — the JVM may read the high 32 bits from one write and the low 32 bits from another, producing a corrupted value (torn read). On 64-bit hardware this is unlikely to manifest, but the Java and Go memory models do not guarantee safe reads without synchronization.

### AtomicCounter

```
atomic.AddInt64(&value, 1)
  → LOCK XADD instruction (x86)
  → CPU locks memory bus for this one instruction
  → No OS involvement. No goroutine sleep. No context switch.
```

`AtomicLong.incrementAndGet()` in Java and `atomic.AddInt64` in Go are both JVM/compiler intrinsics — they compile directly to a single CPU instruction. The overhead is ~1–5 ns per operation vs ~50–100 ns for a mutex under contention.

### GetAndReset — CAS loop

```go
for {
    current := atomic.LoadInt64(&c.value)
    if atomic.CompareAndSwapInt64(&c.value, current, 0) {
        return current
    }
    // retry if another goroutine changed value between Load and CAS
}
```

A naive `read then StoreInt64(0)` would lose increments that occur between the read and the store. The CAS loop guarantees: if the value changed between our read and our write, we retry with the new value. This is the correct pattern for "collect metrics since last poll" use cases.

---

## Alternatives Considered

### `synchronized` keyword (Java) / `sync.RWMutex` (Go)

`synchronized` in Java and `sync.Mutex` in Go are equivalent in correctness. `ReentrantLock` / `sync.Mutex` were chosen for clarity — the lock/unlock boundary is explicit and readable. `synchronized` is fine for simple cases.

`sync.RWMutex` allows multiple concurrent readers. For a counter where `get()` is called as frequently as `increment()`, an `RWMutex` could improve read throughput. However, `AtomicCounter` is strictly better for this use case — no lock at all. `RWMutex` is more appropriate for data structures with expensive reads.

### `LongAdder` (Java) / Striped counter (Go)

`LongAdder` (Java 8+) stripes the counter across multiple cells — each thread increments its own cell, and `sum()` adds them all. Under extreme contention (thousands of threads), `LongAdder` significantly outperforms `AtomicLong` because threads don't compete for the same memory location.

**Tradeoff**: `LongAdder.sum()` is not a consistent snapshot — concurrent modifications can cause it to return a value between two states. Acceptable for metrics, wrong for exact inventory counts or sequence IDs.

Not included here because it adds complexity that isn't justified for the typical counter use case.

### Volatile (Java only)

`volatile long value` fixes the visibility problem (stale cached values) but not the atomicity problem (`value++` is still three instructions). A common mistake. `volatile` is necessary but not sufficient for a thread-safe counter.

---

## Complexity Analysis

| Operation | UnsafeCounter | MutexCounter | AtomicCounter |
|---|---|---|---|
| Increment | O(1), unsafe | O(1) + lock overhead | O(1) lock-free |
| Get | O(1), unsafe | O(1) + lock overhead | O(1) |
| Throughput under contention | Data race | Degrades with thread count | Degrades only under extreme contention |

### When AtomicCounter degrades

Under extreme contention — thousands of threads competing on the same `AtomicLong` simultaneously — CAS retry loops burn CPU cycles. The retry rate grows with contention. At this scale:
- Use `LongAdder` for metrics (approximate but high throughput)
- Shard the counter across N instances and sum them (reduces contention by N×)
- Queue updates and apply in batch (serializes writes but off the hot path)

---

## Edge Cases

| Scenario | Behavior |
|---|---|
| Decrement below zero | Allowed — counters support negative values |
| `incrementBy(0)` | Error — zero delta is almost always a caller bug |
| `incrementBy(-1)` | Error — use `decrement()` explicitly for intent clarity |
| `reset()` under concurrent increments | Reset races with concurrent writes — documented as best-effort |
| `getAndReset()` under concurrent increments | CAS loop guarantees no increment is lost |
| Counter overflow (int64) | Wraps around — at 50k RPS, overflow takes ~292 billion years |

---

## Testing Approach

### Concurrency tests use a gate pattern

All threads/goroutines block on a `CountDownLatch` (Java) or closed channel (Go) and release simultaneously. This maximizes contention — the worst-case scenario. Without the gate, threads start staggered and the race rarely manifests, making the test unreliable.

### The unsafe counter test expects failure

`TestUnsafeCounter_LosesIncrements` passes when the counter produces the wrong answer. This is intentional — it proves the race condition exists. If it somehow returns the correct value, the race didn't manifest this run and a warning is logged.

### Run with race detector

```bash
# Go
go test -race ./...

# Expected output:
# UnsafeCounter → DATA RACE warnings
# MutexCounter  → no warnings
# AtomicCounter → no warnings
```

The Go race detector instruments every memory access and reports concurrent unsynchronized reads/writes. It is the definitive tool for finding data races. Always run with `-race` in CI.

### Benchmarks

```bash
go test -bench=. -benchmem -cpu=1,2,4,8
```

Expected results on a modern machine:
- `AtomicCounter.Increment` → ~5–15 ns/op
- `MutexCounter.Increment` → ~20–80 ns/op (grows with contention)
- The gap widens significantly as `-cpu` (goroutine count) increases

---

## Production Considerations

### Observability

`MutexCounter` exposes `getQueueLength()` (Java) / `IsLocked()` (Go) for diagnostics. In production, emit this as a metric:

```
counter.lock.queue_length  → rising trend = lock contention problem
counter.operations.total   → monotonically increasing = health signal
```

### Metrics emission pattern

```java
// Correct pattern for metrics collection
AtomicCounter requestCounter = new AtomicCounter();

// Request handler
requestCounter.increment();

// Metrics polling thread (every 10s)
long count = requestCounter.getAndReset();
metricsClient.emit("requests.total", count);
```

`getAndReset()` is preferred over `get()` + `reset()` — the two-call version has a window where an increment lands between `get()` and `reset()`, losing it from the metric.

### Never expose the lock

If callers can acquire the counter's internal lock, they can:
- Hold it indefinitely, blocking all other threads
- Forget to release it on exception, causing deadlock
- Create unintended lock ordering with other locks

The interface pattern prevents this. The lock is private. Callers call `increment()`.

---

## Conceptual Mapping — Java vs Go

| Concept | Java | Go |
|---|---|---|
| Mutex | `ReentrantLock` / `synchronized` | `sync.Mutex` |
| Atomic integer | `AtomicLong` | `atomic.Int64` / `atomic.AddInt64` |
| CAS operation | `AtomicLong.compareAndSet()` | `atomic.CompareAndSwapInt64()` |
| Unlock on panic | `try/finally` | `defer mu.Unlock()` |
| Concurrent test gate | `CountDownLatch` | closed channel |
| Race detector | ThreadSanitizer (external) | `go test -race` (built-in) |

Key difference: Go's race detector is built into the toolchain and runs in CI trivially. Java requires external tooling (ThreadSanitizer, Helgrind) which is rarely used in practice. Go's approach catches races earlier.

---

## Future Improvements

1. **Striped counter**: N internal counters, each thread hashes to one. Reduces contention by N×. Useful when `LongAdder`-level throughput is needed but exact reads are also required.

2. **Rate-limited counter**: wraps `AtomicCounter` with a token bucket — increment only succeeds if rate limit allows. Natural extension for rate limiter implementation.

3. **Prometheus integration**: emit counter value as a `prometheus.Counter` metric automatically. Production counters are rarely standalone — they feed into monitoring systems.

4. **Distributed counter**: use Redis `INCR` command for cross-service counting. Redis is single-threaded — `INCR` is inherently atomic. Adds network latency, removes need for in-process synchronization.
