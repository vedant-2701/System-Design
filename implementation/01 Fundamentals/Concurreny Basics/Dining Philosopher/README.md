# Dining Philosophers — Implementation README

## Problem Statement

Five philosophers sit at a round table. Each alternates between thinking and eating.
To eat, a philosopher needs both the fork to their left and the fork to their right.
There are exactly 5 forks — one between each adjacent pair.

The problem demonstrates the core challenge of concurrent resource sharing:
multiple threads competing for multiple shared resources in a pattern that
naturally produces deadlock without explicit prevention.

---

## Why This Problem Matters

Dining philosophers is not an academic curiosity. The same structure appears in:

- **Database transactions**: two transactions each waiting for a lock the other holds
- **Microservice calls**: service A holds resource X and waits for Y; service B holds Y and waits for X
- **Thread pools**: workers waiting on each other's results in circular dependency
- **OS resource allocation**: processes competing for memory, I/O, and CPU simultaneously

Understanding how to solve it structurally — not just in simulation — builds the
reasoning required to prevent deadlock in production systems.

---

## The Naive Failure

Every philosopher picks up their left fork first, then waits for their right fork.

```
P0 holds F0 → waiting for F1
P1 holds F1 → waiting for F2
P2 holds F2 → waiting for F3
P3 holds F3 → waiting for F4
P4 holds F4 → waiting for F0   ← cycle closes
```

All four Coffman conditions are satisfied simultaneously:
- **Mutual exclusion**: a fork can only be held by one philosopher
- **Hold and wait**: each holds one fork while waiting for another
- **No preemption**: no one can steal a fork
- **Circular wait**: the chain closes into a cycle

Result: permanent deadlock.

---

## Solution Chosen — Lock Ordering (Break Circular Wait)

Assign each fork a number 0–4. All philosophers acquire forks in **ascending order**.

| Philosopher | Left Fork | Right Fork | Acquires First | Acquires Second |
|---|---|---|---|---|
| P0 | F0 | F1 | F0 | F1 |
| P1 | F1 | F2 | F1 | F2 |
| P2 | F2 | F3 | F2 | F3 |
| P3 | F3 | F4 | F3 | F4 |
| P4 | F4 | F0 | **F0** | **F4** ← reversed |

P4 acquires F0 first — the lower-numbered fork — not F4. This breaks the cycle:

```
P0 holds F0 → waiting for F1
P1 holds F1 → waiting for F2
P2 holds F2 → waiting for F3
P3 holds F3 → waiting for F4
P4 waiting for F0            ← holds nothing, no cycle
```

P4 blocks but does not hold any resource. The circular wait condition is broken.
P0 eventually eats, releases F0. P4 acquires F0, then F4, eats, releases both.

---

## Why This Approach

Three main solutions exist:

| Solution | Mechanism | Starvation Risk | Implementation Complexity |
|---|---|---|---|
| Lock ordering | Break circular wait | Low | Low |
| Semaphore limit (N-1) | Allow at most 4 philosophers at table | Low | Low |
| Atomic acquisition | Hold all or hold none | **High** | Medium |

Lock ordering was chosen because:
- It directly maps to the most practical deadlock prevention technique used in production
- Database engines, OS kernels, and networking stacks all use lock ordering internally
- It has no retry overhead unlike atomic acquisition
- It is simple to implement, reason about, and audit

---

## Design Overview

```
Fork (ReentrantLock / sync.Mutex)
  └── Models one physical fork — lock state IS fork state
  └── No boolean flag needed — the primitive handles synchronization

Philosopher (Runnable / Goroutine)
  └── Owns lock ordering logic — DiningTable doesn't need to know it
  └── firstFork always lower-numbered
  └── Lifecycle: THINKING → HUNGRY → EATING → DONE

DiningTable
  └── Wires forks to philosophers by adjacency only
  └── Manages thread/goroutine lifecycle via ExecutorService / WaitGroup
  └── Timeout-protected shutdown — anomalies become failures, not hangs

PhilosopherState (enum)
  └── Explicit state transitions for observability and debugging
```

### Key Responsibility Allocation

DiningTable knows: which forks are physically adjacent to each philosopher.
Philosopher knows: which adjacent fork to acquire first (the ordering rule).

These responsibilities are deliberately separated. DiningTable models the table layout.
Philosopher models the concurrent behavior. Neither needs to understand the other's concern.

---

## Why Fork is a Lock, Not a Boolean

A boolean flag requires a separate synchronization mechanism:

```java
// BROKEN — race condition between read and write
if (!forkTaken[i]) {
    forkTaken[i] = true;  // another thread can read between these two lines
}
```

A `ReentrantLock` / `sync.Mutex` handles both state and synchronization in one primitive.
The lock being acquired IS the fork being picked up. No extra layer needed.

---

## Java vs Go — Conceptual Differences

| Concern | Java | Go |
|---|---|---|
| Synchronization primitive | `ReentrantLock` | `sync.Mutex` |
| Reentrancy | Yes — same thread can lock twice | **No** — deadlocks itself |
| Ownership enforcement | Yes — only acquirer can unlock | **No** — any goroutine can unlock |
| Concurrency unit | `Thread` (OS thread) | Goroutine (M:N scheduled) |
| Lifecycle management | `ExecutorService` + `awaitTermination` | `sync.WaitGroup` |
| Timeout pattern | `awaitTermination(N, TimeUnit)` | `select` + `time.After` |
| Loop variable capture | Not applicable | **Must pass as parameter** — closure bug |

### Go-Specific Gotcha: Closure Variable Capture

```go
// BROKEN — all goroutines capture the same p (last value)
for _, p := range philosophers {
    go p.run(&wg)
}

// CORRECT — capture value at launch time
for _, p := range philosophers {
    philosopher := p        // new variable per iteration
    go philosopher.run(&wg)
}
```

This is one of the most common Go concurrency bugs. The loop variable `p` is
shared across all iterations. By the time goroutines execute, `p` has the last
value. Assigning to a new local variable captures the correct value.

---

## Alternatives Considered

### Atomic Acquisition (Hold All or Nothing)
```
if try_acquire(leftFork) and try_acquire(rightFork):
    eat()
else:
    release_all()
    retry()
```
Rejected because: two neighbors can alternate eating, permanently locking out
the philosopher between them. Highest starvation risk of the three solutions.
Also requires a retry loop — introduces livelock risk without randomized backoff.

### Semaphore Limit (Allow N-1 Philosophers at Table)
```
semaphore = 4   // at most 4 can try to eat simultaneously
```
Valid solution. Rejected in favor of lock ordering because lock ordering is more
instructive — it directly applies the Coffman condition analysis, and the same
pattern is used in real systems for precisely the same reason.

### Chandy-Misra (Request/Reply Protocol)
Philosophers request forks from neighbors; holders must give them up after eating.
Guarantees no starvation — the fairest solution. Rejected for this implementation
because it requires significantly more complexity (message passing, dirty/clean fork
state, request queuing). Worth implementing separately as an advanced exercise.

---

## Edge Cases Handled

| Edge Case | Handling |
|---|---|
| Thread interrupted mid-simulation | Interrupt flag restored, philosopher exits cleanly |
| Simulation timeout (potential hang) | `awaitTermination` + `shutdownNow` in Java; `time.After` in Go |
| Wrong meals argument | Parsed with validation, falls back to default |
| Single meal | Tested explicitly — same code path, no special handling needed |

---

## Testing Strategy

| Test | What It Validates | Why |
|---|---|---|
| `NoDeadlock` | Simulation completes within timeout | Core correctness claim |
| `CorrectMealCount` | Every philosopher eats exactly N meals | Starvation detection |
| `StabilityAcrossRuns` | 10 repeated runs all pass | Non-deterministic bug coverage |
| `MutualExclusion` | No two philosophers hold same fork simultaneously | Race condition detection |
| `FinalStateIsDone` | All philosophers reach DONE state | Lifecycle correctness |
| `LockOrdering` | firstFork.id < secondFork.id for all philosophers | Structural guarantee |

### Why Repeated Runs Matter

Concurrency bugs are timing-dependent. A race condition that occurs 1% of the time
passes in a single run 99% of the time. Running 10 times raises detection probability
to ~10%. Running 100 times: ~63%. The Go race detector (`go test -race`) provides
stronger guarantees by instrumenting every memory access at runtime — always use it.

```bash
# Go — run with race detector
go test -race -v ./...

# Java — run tests
javac -cp junit-platform.jar *.java && java -jar junit-platform.jar
```

---

## Production Considerations

**Starvation is still possible.** Lock ordering prevents deadlock but does not
guarantee fairness. A philosopher can theoretically lose the fork race repeatedly
under adversarial scheduling. In production, fairness is addressed by:
- `ReentrantLock(true)` in Java — FIFO acquisition order
- Priority aging — gradually increase scheduling priority of waiting threads

**Observability matters.** The `PhilosopherState` enum and structured logging exist
precisely so that a debugging session can answer: "which philosopher is stuck and
on which fork?" In production, this maps to: "which thread is blocked and on which lock?"
Structured logs with philosopher ID and fork ID make this answerable in seconds.

**Timeout is a safety net, not a feature.** The 30-second shutdown timeout in
`DiningTable` should never fire with correct deadlock prevention. If it does fire
in a real system, it signals a bug — log it at SEVERE, not WARN. Silent hangs
become invisible outages.

---

## Future Improvements

- Implement Chandy-Misra for starvation-free guarantee
- Add metrics: meals per philosopher, average wait time per fork, max wait time
- Add configurable philosopher count (current implementation hardcodes 5)
- Implement the semaphore-based solution as an alternative for comparison
- Add a deliberately broken version (no lock ordering) to demonstrate deadlock for educational purposes
