# Mutex, Semaphore, Spinlock

## Tags
#concurrency #backend-engineering #os #lld

---

## Overview
- Three primitives for coordinating access to shared resources
- Solve the same core problem — preventing race conditions — but in fundamentally different ways
- Choosing the wrong one introduces correctness bugs or performance collapse
- A **race condition** occurs when a multi-step operation (read → modify → write) is interrupted by another thread between steps

---

## Mutex

### What It Is
- Mutual Exclusion lock — only one thread holds it at a time
- All other threads attempting to acquire it **sleep** until it is released
- Has strict **ownership** — only the acquiring thread can release it

### Internal Working
```
Thread A acquires lock → enters critical section
Thread B tries to acquire → blocked (sleeps)
Thread A releases lock → Thread B wakes up → acquires lock
```

### Why `counter += 1` Needs a Mutex
At CPU level this is 3 instructions — not 1:
1. Load value from memory into register
2. Add 1 to register
3. Write register back to memory

Two threads can both read the same value before either writes back → lost increment.

### Tradeoffs
| | |
|---|---|
| Benefit | Correct mutual exclusion with ownership enforcement |
| Cost | Threads sleep → context switch overhead (~1,000–10,000 ns) |
| Risk | Deadlock if multiple mutexes acquired in wrong order |

---

## Semaphore

### What It Is
- Integer counter — threads decrement to acquire, increment to release
- **No ownership** — any thread can signal (release) it
- Binary semaphore (0/1) vs counting semaphore (0 to N)

### Key Distinction From Mutex
| | Mutex | Semaphore |
|---|---|---|
| Ownership | Yes — acquirer must release | No — any thread can signal |
| Value | Binary | Integer 0 to N |
| Primary use | Protecting critical section | Signaling + resource counting |

### Using Binary Semaphore as Mutex — The Danger
Any thread can signal it. Thread B can release a semaphore Thread A acquired, allowing Thread C to enter the critical section while Thread A is still inside it. Mutual exclusion guarantee is broken.

### Counting Semaphore Use Case
```
semaphore = 10   # 10 DB connections available
thread acquires → semaphore = 9
thread releases → semaphore = 10
semaphore = 0 → new threads block
```
Models bounded resource pools (DB connections, HTTP connections).

### Tradeoffs
| | |
|---|---|
| Benefit | Models resource counting and inter-thread signaling |
| Benefit | Flexible — producer/consumer coordination |
| Risk | No ownership enforcement → easy to signal incorrectly |
| Risk | Misuse as mutex creates subtle correctness bugs |

---

## Spinlock

### What It Is
- Thread busy-waits in a loop instead of sleeping
- Continuously checks "is the lock free?" using CPU cycles
- No OS involvement, no context switch

```python
while lock.is_held():
    pass  # burn CPU cycles checking
# lock free — acquire
```

### When Sleeping Is Worse Than Spinning
- Context switch costs ~5,000–10,000 ns
- If lock is held for 50 ns — sleeping costs 100x more than just waiting
- Spinlock wins when critical section duration << context switch cost

### Tradeoffs
| | |
|---|---|
| Benefit | Zero context switch overhead for very short waits |
| Benefit | Usable in interrupt handlers (cannot sleep) |
| Cost | Burns CPU cycles — wasteful under high contention |
| Cost | Effectively single-core only — on one core, holder can't run to release |
| Risk | Multiple threads spinning simultaneously → CPU throughput collapse |

### When to Use Each
| Scenario | Choice |
|---|---|
| Short critical section (ns), multi-core | Spinlock |
| Longer critical section (µs+) | Mutex |
| Kernel interrupt handler | Spinlock |
| Resource counting / signaling | Semaphore |
| Simple shared counter at high RPS | Neither — use [[Compare-and-Swap (CAS)]] |

---

## Failure Scenarios
- **Using binary semaphore as mutex** → external thread signals it → mutual exclusion violated
- **Spinlock on single-core** → holder never gets CPU to release → infinite spin
- **High contention spinlock** → all cores burning cycles → system throughput collapse
- **Mutex under high contention** → thundering herd on lock release → context switch storm

---

## Real-World Usage
- OS kernels use spinlocks for very short critical sections where sleeping is prohibited
- Java `synchronized` and `ReentrantLock` implement mutex semantics
- `java.util.concurrent.Semaphore` used for connection pool bounding
- Linux `pthread_mutex_t` — mutex with ownership enforcement

---

## Common Mistakes
- Using binary semaphore where mutex is needed — loses ownership guarantee
- Using spinlock for long critical sections — CPU waste
- Ignoring that `counter += 1` is not atomic — assuming single-line = single instruction
- Spinlock on a single-core machine — deadlocks effectively

---

## Interview Perspective
- Always asked: "difference between mutex and semaphore" — answer with **ownership**, not just binary vs counting
- Follow-up: "why can't you use binary semaphore as mutex?" — any thread can release it
- Spinlock question: "when is busy-waiting better than sleeping?" — short critical sections, interrupt handlers, multi-core
- Often tested: recognizing race conditions in simple code snippets

---

## Revision Summary
- Mutex = exclusive access + ownership. Only acquirer can release.
- Semaphore = no ownership. Any thread signals. Models counting and signaling.
- Binary semaphore ≠ mutex. Lacks ownership — unsafe as mutex.
- Spinlock = busy-wait. No sleep. Fast for nanosecond locks. Wastes CPU under contention.
- For atomic counters at scale → CAS, not mutex or spinlock.
- Race condition = read-modify-write interrupted between steps.

---

## Active Recall Questions
1. Why can't a binary semaphore safely replace a mutex?
2. What exactly makes `counter += 1` a race condition at the CPU level?
3. When does a spinlock outperform a mutex? When does it catastrophically underperform?
4. You need to limit concurrent DB connections to 10. Which primitive and why?
5. A kernel interrupt handler needs to protect a small data structure. What do you use and why?

---

## Related Concepts
- [[Compare-and-Swap (CAS)]]
- [[Deadlock, Livelock, Starvation]]
- [[Thread Pools]]
- [[Memory Visibility and Happens-Before]]
- [[Concurrency Models — Threads vs Event Loop vs Async Await]]
