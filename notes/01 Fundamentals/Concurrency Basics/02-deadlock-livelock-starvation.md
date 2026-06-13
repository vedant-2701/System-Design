# Deadlock, Livelock, Starvation

## Tags
#concurrency #backend-engineering #os #distributed-systems

---

## Overview
- Three distinct concurrency failure modes — commonly conflated in interviews
- All three result in threads failing to make expected progress
- Distinguished by: whether threads are blocked, whether CPU is busy, and whether the system makes any progress at all

| | Progress? | Threads Blocked? | CPU Busy? |
|---|---|---|---|
| Deadlock | No | Yes | No |
| Livelock | No | No | Yes |
| Starvation | Partial | No | Yes |

---

## Deadlock

### What It Is
Two or more threads permanently blocked — each waiting for a resource the other holds. No thread can ever proceed.

```
Thread A: holds Lock 1, waiting for Lock 2
Thread B: holds Lock 2, waiting for Lock 1
→ Neither can proceed. Ever.
```

### Coffman Conditions
All four must hold simultaneously for deadlock to occur. Break **any one** to prevent it.

| Condition | Meaning |
|---|---|
| Mutual Exclusion | Resources cannot be shared |
| Hold and Wait | Thread holds one resource while waiting for another |
| No Preemption | Resources cannot be forcibly taken |
| Circular Wait | Circular chain of threads each waiting on the next |

### Prevention Strategies

**Break Hold and Wait — Atomic Acquisition:**
Acquire all required resources upfront or release all and retry. Thread never holds a partial set.
```python
if try_acquire(R1) and try_acquire(R2):
    # do work
    release(R1, R2)
else:
    release_all()
    backoff_and_retry()
```
Limitation: must know all required resources upfront.

**Break Circular Wait — Lock Ordering:**
Enforce a global acquisition order. All threads acquire locks in the same numbered sequence.
```
Rule: always acquire Lock 1 before Lock 2, never reverse
Thread A: Lock1 → Lock2 ✓
Thread B: Lock1 → Lock2 ✓ (blocks at Lock1, not circular)
```
Most practical fix. Used extensively in OS kernels, databases, networking stacks.

**Detection + Recovery (Database approach):**
Allow deadlock to occur. Detect using a wait-for graph (cycle = deadlock). Kill one transaction, let the other proceed. Rolled-back transaction retries.
PostgreSQL and MySQL use this approach. Valid when deadlocks are rare and retry is cheap.

---

## Livelock

### What It Is
Threads are actively running and responding to each other but making no forward progress. CPU is busy. Work is not happening.

```
Thread A detects conflict → backs off → retries
Thread B detects conflict → backs off → retries
Both retry simultaneously → conflict again → repeat indefinitely
```

Analogy: two people in a corridor both stepping aside in the same direction repeatedly.

### Why It's Harder to Detect Than Deadlock
- CPU usage appears normal or high
- Threads are not blocked — they're "working"
- No OS-level signal that something is wrong
- Requires application-level monitoring to detect

### Fix
Randomized exponential backoff — threads retry at different times, breaking the lockstep.

```python
import random, time
base_delay = 0.1
for attempt in range(max_retries):
    if try_operation():
        break
    time.sleep(base_delay * (2 ** attempt) + random.uniform(0, 0.1))
```

---

## Starvation

### What It Is
One or more threads never get scheduled even though the system is making overall progress. Not a deadlock — other threads are completing work.

```
Thread A (high priority) — always wins lock acquisition
Thread B (low priority) — always loses, waits indefinitely
System makes progress overall, but Thread B never runs
```

### Common Causes
- Unfair scheduling policies
- Priority inversion — low-priority thread holds resource needed by high-priority thread
- Biased lock implementations that favor certain threads

### Fix
- **Fair scheduling** — FIFO queues for lock acquisition (Java `ReentrantLock(true)` — fair mode)
- **Priority aging** — gradually increase priority of waiting threads over time
- **Priority inheritance** — temporarily elevate priority of lock-holding thread to prevent inversion

---

## Failure Scenarios
- **Wrong lock order in two different code paths** → both paths work individually, deadlock only under specific timing → hard to reproduce
- **Retry logic without jitter** → livelock under high concurrency → system appears busy but throughput is zero
- **Priority-based thread pools without aging** → low-priority background jobs starve indefinitely under sustained load

---

## Real-World Usage
- **Deadlock detection**: PostgreSQL, MySQL — wait-for graph cycle detection, automatic victim selection
- **Lock ordering**: Linux kernel acquires locks in strict global order across subsystems
- **Livelock in distributed systems**: Two services retrying conflicting writes simultaneously without jitter — solved by exponential backoff with jitter (used by AWS, Kafka retry logic)
- **Starvation**: Java `synchronized` keyword does not guarantee fairness — `ReentrantLock(true)` required for FIFO ordering

---

## Common Mistakes
- Confusing livelock with deadlock — livelock has busy CPU, deadlock does not
- Thinking "just add retries" fixes everything — retries without jitter create livelock
- Not enforcing consistent lock ordering across all code paths — one inconsistency is enough for deadlock
- Assuming deadlock is always reproducible — timing-dependent, often only surfaces under load

---

## Interview Perspective
- Always asked: distinguish deadlock vs livelock vs starvation — use the progress/blocked/CPU table
- Coffman conditions are expected knowledge — and which one to break for a given scenario
- "Two threads, two locks, occasional hang" → deadlock → lock ordering is the clean fix
- Follow-up: "how do databases handle deadlock?" → detection via wait-for graph, not prevention
- Livelock question often framed as: "retries aren't working, system is busy but making no progress"

---

## Revision Summary
- Deadlock: circular wait, all threads blocked, CPU idle. Break one Coffman condition.
- Livelock: threads active, no progress, CPU busy. Fix with randomized backoff.
- Starvation: partial progress, one thread never runs. Fix with fair scheduling or priority aging.
- Lock ordering (break circular wait) is the most practical deadlock prevention in production.
- Databases prefer deadlock detection + rollback over prevention.
- Retry without jitter → livelock. Always add jitter to retry logic.

---

## Active Recall Questions
1. What are the four Coffman conditions? Which is easiest to break in practice?
2. System is at high CPU but throughput is zero. Deadlock or livelock? How do you tell?
3. You have two threads and two database connections. System hangs at 0% CPU. Which failure mode, and how do you fix it?
4. Why does retry logic without jitter cause livelock under high concurrency?
5. How does PostgreSQL handle deadlock? Why is this better than strict prevention for databases?
6. What is priority inversion? How does priority inheritance fix it?

---

## Related Concepts
- [[Mutex, Semaphore, Spinlock]]
- [[Thread Pools]]
- [[Compare-and-Swap (CAS)]]
- [[Distributed Locks]]
- [[Retry with Exponential Backoff and Jitter]]
