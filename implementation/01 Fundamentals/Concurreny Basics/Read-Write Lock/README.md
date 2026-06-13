# Read-Write Lock — Implementation Notes

## Problem Statement

A plain mutex serialises all access to shared state — readers block each other even though
concurrent reads are safe. Under read-heavy workloads this unnecessarily collapses throughput.

A Read-Write Lock solves this by allowing:
- **Multiple concurrent readers** — as long as no writer holds the lock
- **Exactly one writer** — exclusive, no readers or other writers simultaneously

Real-world use cases: shared caches, configuration stores, in-memory indexes,
connection registries — anything with significantly more reads than writes.

---

## The Starvation Problem

A naïve RW Lock implementation allows new readers to jump ahead of waiting writers.
If reads arrive continuously, a writer may wait indefinitely — **writer starvation**.

The fix: once a writer registers intent (`waitingWriters++`), new readers block
even if no writer currently holds the lock. The writer drains existing readers,
then acquires. Only after the writer releases do blocked readers proceed.

---

## State Model

Three fields drive all acquisition decisions:

```
activeReaders  int     — goroutines/threads currently reading
waitingWriters int     — goroutines/threads queued to write
isWriting      bool    — a writer currently holds the lock
```

**Acquisition rules derived directly from state:**

```
Reader acquires when:  !isWriting && waitingWriters == 0
Writer acquires when:  !isWriting && activeReaders == 0
```

These two conditions are the exact loop guards in both implementations.

---

## Why `while` Not `if`

Threads waiting on a condition variable can wake up spuriously — the JVM and POSIX
both explicitly permit this. A spurious wakeup means a thread wakes up even though
no signal was sent and the condition may not be satisfied.

Using `if` would proceed immediately on any wakeup:
```java
// WRONG — if spurious wakeup occurs, isWriting may still be true
if (isWriting || waitingWriters > 0) condition.await();
activeReaders++;
```

Using `while` re-checks the guard on every wakeup:
```java
// CORRECT — always re-evaluate after waking
while (isWriting || waitingWriters > 0) condition.await();
activeReaders++;
```

Both Java and Go implementations use `while`/`for` loops.

---

## Signal vs Broadcast (Signal vs SignalAll)

**When a reader releases (`unlockRead`):**
- Only one writer can proceed — `signal()` / `Signal()` is correct
- Broadcasting would wake all waiting writers; only one acquires, rest re-sleep — wasteful

**When a writer releases (`unlockWrite`) with no writers queued:**
- All blocked readers can now proceed simultaneously — `signalAll()` / `Broadcast()` required
- Signalling only one reader would leave others sleeping indefinitely

**When a writer releases with writers queued:**
- Signal one waiting writer — writer preference policy
- New readers remain blocked until the writer queue empties

---

## Interrupt Safety (Java)

In `lockWrite`, `waitingWriters` must be decremented whether the thread acquires
the lock or is interrupted while waiting:

```java
waitingWriters++;
try {
    while (isWriting || activeReaders > 0)
        writerReady.await();   // InterruptedException thrown here
} finally {
    waitingWriters--;          // executes on both normal exit AND exception
}
isWriting = true;
```

If `waitingWriters` is not decremented on interrupt, it leaks — new readers
are permanently suppressed even though no writer is actually waiting. This is
a silent correctness bug that only manifests as reader starvation under load.

The interrupt test (`interruptedWriterDoesNotCorruptWaitingWritersCount`) validates this.

---

## Java vs Go — Conceptual Mapping

| Concept | Java | Go |
|---|---|---|
| Mutual exclusion primitive | `ReentrantLock` | `sync.Mutex` |
| Condition variable | `Condition` (from `lock.newCondition()`) | `sync.Cond` (from `sync.NewCond(&mu)`) |
| Wait | `condition.await()` | `cond.Wait()` |
| Wake one | `condition.signal()` | `cond.Signal()` |
| Wake all | `condition.signalAll()` | `cond.Broadcast()` |
| Timed wait | `condition.awaitNanos(nanos)` | No direct equivalent — goroutine + select |
| Interrupt handling | `InterruptedException` from `await()` | No direct equivalent — use context |

Both languages expose the same underlying primitives — the logic is identical.
The difference is ergonomics and error handling model.

---

## Alternatives Considered

### Using `synchronized` (Java) / `sync.Mutex` (Go) directly
Would work for a basic lock but provides no condition variable, making it
impossible to efficiently wait for specific state transitions. Spinning on
an atomic check would waste CPU — the classic spinlock tradeoff.

### Using `java.util.concurrent.locks.ReentrantReadWriteLock`
The JDK provides a production-grade RW Lock implementation. In real systems,
use that. This implementation exists to expose the underlying mechanics explicitly.
The JDK implementation adds: lock downgrading, optional fairness mode,
`tryLock` with interruption, and performance optimisations not present here.

### Using `sync.RWMutex` (Go)
Same reasoning — prefer the standard library in production. `sync.RWMutex`
is carefully optimised and tested. This implementation demonstrates the mechanics.

---

## Tradeoffs of This Implementation

| | |
|---|---|
| Writer preference policy | Writers are preferred over readers when both are waiting. Prevents writer starvation. Can cause reader starvation under sustained write pressure — an acceptable tradeoff for write-heavy workloads. |
| Single condition variable (Go) | One `sync.Cond` wakes all waiters on `Broadcast`. Slightly less efficient than two separate conditions for readers and writers. Java uses two `Condition` objects for this reason. |
| No lock downgrading | A writer cannot atomically downgrade to a reader without releasing and re-acquiring. Adding this increases complexity significantly. |
| No reentrance | A goroutine/thread that holds the read lock and tries to acquire the write lock will deadlock. Reentrant RW locks require per-thread lock tracking. |

---

## Production Considerations

**Prefer standard library implementations in production:**
- Java: `java.util.concurrent.locks.ReentrantReadWriteLock`
- Go: `sync.RWMutex`

**When to consider a custom implementation:**
- Custom fairness policies not provided by the standard library
- Need to instrument lock contention metrics (waiting time, queue depth)
- Need lock-level observability for debugging latency spikes

**Observability addition:**
The `Snapshot()` / `snapshot()` method provides lock state for debugging.
In production, emit these as metrics on a slow path (e.g., every N acquisitions)
to detect contention spikes without adding overhead to every acquire/release.

---

## Running Tests

### Go
```bash
cd go
go test -v -race ./...
```
The `-race` flag enables Go's race detector — validates that the lock
actually prevents data races under concurrent access.

### Java
```bash
cd java
mvn test
```
Requires JDK 21+. Tests use JUnit 5 with `@Timeout` annotations to
catch deadlocks and starvation automatically.

---

## File Structure

```
rwlock/
├── go/
│   ├── rwlock.go          # Implementation
│   ├── rwlock_test.go     # Tests (run with -race)
│   └── go.mod
└── java/
    ├── pom.xml
    └── src/
        ├── main/java/com/rwlock/
        │   ├── ReadWriteLock.java              # Interface
        │   └── StarvationFreeReadWriteLock.java # Implementation
        └── test/java/com/rwlock/
            └── StarvationFreeReadWriteLockTest.java
```
