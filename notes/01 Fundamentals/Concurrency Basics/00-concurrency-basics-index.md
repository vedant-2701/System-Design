# Concurrency Basics — Index

## Tags
#concurrency #backend-engineering #index

---

## Overview
Core concurrency primitives, failure modes, and mental models for backend engineering and system design.

---

## Notes in This Topic

| Note | Core Concept |
|---|---|
| [[Mutex, Semaphore, Spinlock]] | Synchronization primitives — ownership, signaling, busy-wait |
| [[Deadlock, Livelock, Starvation]] | Concurrency failure modes — detection, prevention, recovery |
| [[Thread Pools]] | Bounded thread reuse — sizing, backpressure, rejection policies |
| [[Concurrency Models — Threads vs Event Loop vs Async Await]] | Choosing the right concurrency model for CPU vs I/O workloads |
| [[Memory Visibility and Happens-Before]] | Why correct-looking code fails — compiler reordering, CPU caches, memory barriers |
| [[Compare-and-Swap (CAS)]] | Lock-free programming — atomic operations, ABA problem, optimistic locking |
| [[Amdahl's Law]] | Fundamental scaling ceiling — sequential fraction, distributed bottlenecks |

---

## Mental Model — How These Connect

```
Shared resources need protection
        ↓
Mutex → exclusive access with ownership
Semaphore → signaling and resource counting
Spinlock → busy-wait for nanosecond locks
        ↓
Coordination failures
Deadlock → circular wait, permanent block, CPU idle
Livelock → active, no progress, CPU busy
Starvation → partial progress, some threads never run
        ↓
Thread Pools → bound concurrency, reuse threads, enforce backpressure
CPU-bound → cores. I/O-bound → Little's Law
        ↓
Concurrency Models
Threads → true parallelism, expensive
Event Loop → single thread, I/O only, never blocks
Async/Await → coroutines, syntactic sugar over event loop
        ↓
Memory Visibility
Compiler and CPU reorder writes
Happens-Before guarantees cross-thread visibility
Volatile and synchronized enforce ordering
        ↓
CAS → lock-free atomic operations
Read → compute → verify → write atomically
ABA problem → stamp/version needed
        ↓
Amdahl's Law → fundamental scaling ceiling
Sequential fraction determines maximum speedup
Lock granularity, partition count, single leaders
all express this same architectural principle
```

---

## Key Tradeoffs to Remember

| Decision | Consider |
|---|---|
| Mutex vs Spinlock | Lock duration vs context switch cost |
| Mutex vs CAS | Contention level — CAS wins low, mutex wins very high |
| Threads vs Event Loop | CPU-bound vs I/O-bound workload |
| Prevent vs Detect deadlock | Lock ordering (prevent) vs DB rollback (detect + recover) |
| Volatile vs Synchronized | Visibility only vs visibility + atomicity |

---

## Roadmap Status
- [ ] Mutex, Semaphore, Spinlock
- [ ] Deadlock, Livelock, Starvation
- [ ] Thread Pools
- [ ] Async/Await vs Threads vs Event Loop
- [ ] Memory Visibility & Happens-Before
- [ ] Compare-and-Swap (CAS)
- [ ] Amdahl's Law
