# Memory Visibility and Happens-Before

## Tags
#concurrency #backend-engineering #java #memory-model #os

---

## Overview
- Code that looks correct in single-threaded context can silently produce wrong results under concurrency
- Two root causes: **compiler reordering** and **CPU cache visibility**
- Happens-Before is the formal model for reasoning about what one thread can see from another
- Without an explicit happens-before relationship, there is **no visibility guarantee** — even if writes appear ordered in source code

---

## The Core Problem

```java
boolean ready = false;
int value = 0;

// Thread A
value = 42;
ready = true;

// Thread B
while (!ready) {}
print(value);  // Can print 0
```

Most engineers expect this to always print 42. It can print 0. Two distinct causes.

---

## Cause 1 — Compiler Reordering

The compiler sees `value = 42` and `ready = true` as two **independent writes**. Neither depends on the other. The compiler is legally allowed to reorder them for optimization:

```
// Compiler-reordered output
ready = true;   ← written first
value = 42;     ← written second
```

From a single-threaded perspective the final state is identical — correct optimization. But Thread B can observe `ready = true` before `value = 42` is written.

The compiler has no obligation to preserve write ordering across threads unless explicitly told to.

---

## Cause 2 — CPU Cache Visibility

Modern CPUs have per-core L1/L2 caches. Writes go to the local cache first.

```
Core 1 (Thread A)          Core 2 (Thread B)
┌─────────────┐            ┌─────────────┐
│ value = 42  │            │ value = 0   │  ← stale cached value
│ ready = true│            │ ready = ?   │
└─────────────┘            └─────────────┘
        ↕                          ↕
┌──────────────────────────────────────────┐
│             Main Memory                  │
│         value = 0   ready = false        │
└──────────────────────────────────────────┘
```

Thread A's write to `value` sits in its L1 cache. Main memory not yet updated. Thread B reads from its own cache or main memory — sees stale value 0. Cache coherency protocols don't guarantee **when** a write becomes visible without explicit synchronization.

---

## Happens-Before — The Formal Model

> If operation A **happens-before** operation B, then all effects of A are **guaranteed visible** to B.

Without a happens-before relationship: zero visibility guarantee, regardless of source code order.

### What Establishes Happens-Before

| Mechanism | Relationship |
|---|---|
| Mutex release → acquire | `unlock()` happens-before `lock()` on same mutex |
| Volatile write → read | volatile write happens-before subsequent volatile read of same variable |
| `Thread.start()` | Code before `start()` happens-before first instruction in new thread |
| `Thread.join()` | All instructions in thread happen-before return from `join()` |
| Static initializer | Class initialization happens-before any thread uses the class |

---

## Fix 1 — Synchronized Block

```java
synchronized(lock) {
    value = 42;
    ready = true;
}

synchronized(lock) {
    if (ready) print(value);
}
```

Lock **release** by Thread A happens-before lock **acquire** by Thread B. Full visibility guaranteed.

---

## Fix 2 — Volatile

```java
volatile boolean ready = false;
int value = 0;

// Thread A
value = 42;
ready = true;   // volatile write — memory barrier here

// Thread B
while (!ready) {}   // volatile read
print(value);       // guaranteed to see 42
```

A volatile write creates a **memory barrier**:
- Prevents compiler from reordering writes across the barrier
- Forces CPU to flush cache to main memory before the volatile write
- All writes before the volatile write are visible after the volatile read

Volatile on `ready` guarantees visibility of `value = 42` as well — because the barrier applies to everything before it.

### Volatile Limitation
Volatile guarantees **visibility** but not **atomicity**. `volatile int counter` with concurrent `counter++` is still a race condition — increment is still 3 instructions.

---

## The Double-Checked Locking Bug

Classic production bug — broken without volatile:

```java
// BROKEN
private static Singleton instance;

public static Singleton getInstance() {
    if (instance == null) {                    // check 1
        synchronized(Singleton.class) {
            if (instance == null) {            // check 2
                instance = new Singleton();    // not atomic
            }
        }
    }
    return instance;
}
```

`instance = new Singleton()` is three steps:
1. Allocate memory
2. Initialize fields
3. Assign reference to `instance`

CPU can reorder steps 2 and 3. Another thread sees non-null `instance` (step 3 done) but reads uninitialized fields (step 2 not yet done).

**Fix**: declare `instance` as `volatile`. Memory barrier after step 3 prevents reordering.

```java
private static volatile Singleton instance;  // fixed
```

---

## Tradeoffs

| | Synchronized | Volatile |
|---|---|---|
| Visibility guarantee | Yes | Yes |
| Atomicity guarantee | Yes (within block) | No |
| Performance | Higher overhead (lock acquisition) | Lower overhead (no lock) |
| Use case | Multiple operations must be atomic | Single flag/reference visibility |

---

## Failure Scenarios
- **Missing volatile on flag** → compiler reorders or CPU caches stale value → threads never see flag update → infinite loop
- **Using volatile for compound operations** → visibility guaranteed, atomicity is not → race condition on `volatile counter++`
- **Double-checked locking without volatile** → partially initialized object visible to other threads → subtle data corruption
- **Assuming sequential source code = sequential execution** → fundamental misunderstanding of modern CPU/compiler behavior

---

## Real-World Usage
- Java `volatile` — shutdown flags, stop signals, lazy initialization
- Java `synchronized` — any critical section requiring both visibility and atomicity
- C++ `std::atomic` / `memory_order` — explicit memory ordering control
- Go — channel operations establish happens-before automatically
- Linux kernel — `smp_wmb()`, `smp_rmb()` explicit memory barriers

---

## Common Mistakes
- Assuming writes in source order = writes in execution order
- Using volatile where atomicity is also needed — still a race condition
- Assuming `synchronized` on different lock objects provides visibility — only the same lock establishes happens-before
- Ignoring memory visibility in "obvious" flag patterns — the most common production concurrency bug

---

## Interview Perspective
- Rarely asked directly but appears disguised as: "why does this code sometimes print wrong values?"
- Double-checked locking is a classic interview problem — must know volatile fixes it and why
- Senior-level question: "what's the difference between volatile and synchronized?"
- Expected understanding: CPU caches, compiler reordering, memory barriers are real effects

---

## Revision Summary
- Source code order ≠ execution order. Compiler and CPU reorder writes for optimization.
- CPU per-core caches mean writes aren't immediately visible to other cores.
- Happens-before: the only formal guarantee of cross-thread visibility.
- Volatile: establishes happens-before via memory barrier. Guarantees visibility, not atomicity.
- Synchronized: establishes happens-before via lock release/acquire. Guarantees both.
- Double-checked locking without volatile → partially initialized object visible → data corruption.
- `volatile counter++` is still a race condition — use AtomicInteger instead.

---

## Active Recall Questions
1. Why can `print(value)` output 0 even though `value = 42` appears before `ready = true` in source code?
2. What two distinct mechanisms cause the memory visibility problem?
3. What does a volatile write actually do at the CPU and compiler level?
4. Why does volatile fix double-checked locking? Walk through exactly what breaks without it.
5. What is the difference between volatile and synchronized in terms of what they guarantee?
6. `volatile int counter; counter++` — is this thread-safe? Why or why not?

---

## Related Concepts
- [[Mutex, Semaphore, Spinlock]]
- [[Compare-and-Swap (CAS)]]
- [[Thread Pools]]
- [[Concurrency Models — Threads vs Event Loop vs Async Await]]
- [[Java Memory Model]]
