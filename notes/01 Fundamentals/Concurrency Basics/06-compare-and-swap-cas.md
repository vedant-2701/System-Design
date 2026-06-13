# Compare-and-Swap (CAS)

## Tags
#concurrency #backend-engineering #lock-free #distributed-systems #databases

---

## Overview
- Single atomic CPU instruction — read, compare, and write happen as one indivisible operation
- Foundation of lock-free programming — no mutex, no OS involvement, no thread sleeping
- Faster than mutex under low-to-medium contention for simple operations
- Used in atomic counters, lock-free data structures, optimistic locking in databases, distributed coordination

---

## How CAS Works

```
CAS(memory_location, expected_value, new_value):
    if current_value == expected_value:
        current_value = new_value
        return true   // success
    else:
        return false  // value changed — someone else modified it
```

The read-compare-write is **one CPU instruction** — the memory bus is locked for its duration. No other thread can interleave.

---

## Lock-Free Counter with CAS

```java
AtomicInteger counter = new AtomicInteger(0);

void increment() {
    while (true) {
        int current = counter.get();              // read
        int next = current + 1;                   // compute
        if (counter.compareAndSet(current, next)) // verify + write
            return;  // success
        // CAS failed — another thread changed value — retry
    }
}
```

### Why This Beats Mutex for Counters

```
Mutex approach:
  Thread A: acquire lock → increment → release lock
  Thread B: sleeping → wake up (context switch) → acquire lock
  Cost: 2 context switches + kernel syscall

CAS approach:
  Thread A: read 0 → compute 1 → CAS(0→1) succeeds
  Thread B: read 0 → compute 1 → CAS(0→1) fails → read 1 → CAS(1→2) succeeds
  Cost: one retry — no kernel, no context switch, no sleeping
```

Under low contention CAS wins significantly. No thread ever sleeps.

---

## The ABA Problem

### What It Is
CAS checks value equality — not identity of change. A value can change A→B→A between a thread's read and its CAS — the CAS succeeds incorrectly because the value looks unchanged.

```
Initial: counter = 10 (version A)

Thread 1: reads 10, prepares CAS(10 → 30)
Thread 1: preempted before executing CAS

Thread 2: CAS(10 → 20) succeeds  [A → B]
Thread 3: CAS(20 → 10) succeeds  [B → A]

Thread 1: resumes, executes CAS(10 → 30)
          sees 10 == expected 10 → CAS succeeds
          but the world changed twice underneath it
```

For a counter this may be acceptable. For a linked list or complex data structure, this corrupts state.

### Fix — Stamped Reference (Version + Value)

```java
// Java AtomicStampedReference — value + version stamp
AtomicStampedReference<Integer> ref =
    new AtomicStampedReference<>(10, 0);  // value=10, stamp=0

int[] stampHolder = new int[1];
int current = ref.get(stampHolder);
int currentStamp = stampHolder[0];

ref.compareAndSet(
    current,           // expected value
    newValue,          // new value
    currentStamp,      // expected stamp
    currentStamp + 1   // new stamp — always increments
);
```

Thread 1's CAS now fails — value is 10 but stamp is 2, not 0. Intermediate changes are detectable.

This is the same pattern as **optimistic locking** in databases — a version column incremented on every write.

---

## CAS in Databases — Optimistic Locking

Database optimistic locking is CAS at the application level:

```sql
UPDATE orders
SET status = 'processed', version = version + 1
WHERE id = 123 AND version = 5;
-- 0 rows affected → version changed → someone else updated → retry
```

Read version → compute update → verify version unchanged → write. Identical semantics to hardware CAS.

Used when:
- Read-heavy workloads — contention is rare
- Short transactions — retry cost is low
- Pessimistic locking would cause unnecessary blocking

---

## CAS in Distributed Systems — etcd

etcd uses CAS for leader election and distributed coordination:

```
CAS(key, expected_value, new_value)
→ only one node succeeds when multiple try simultaneously
→ foundation for distributed locks, leader election
```

---

## Tradeoffs

| | |
|---|---|
| Benefit | No kernel involvement — no sleeping, no context switch |
| Benefit | Lock-free — a thread dying mid-operation doesn't block others |
| Benefit | Very fast under low-to-medium contention |
| Cost | Under high contention → many retries → wasted CPU cycles (retry storm) |
| Cost | ABA problem requires stamped references for complex data structures |
| Cost | Only atomic on a **single memory location** — multi-location updates still need mutex |
| Limit | Complex lock-free data structures are extremely hard to implement correctly |

---

## Hot Key Problem Under Extreme Contention

At 50,000 RPS competing on the same key:

```
Most CAS attempts fail → all threads retry → retry storm
CPU burns on retries → throughput collapses
```

Solutions:
- **Shard the key**: split into N sub-keys, each thread hashes to a shard → contention reduced by N
- **Queue-based serialization**: Kafka partition per key → single consumer → no contention at all
- **Local buffering**: pre-allocate batches locally, reconcile with central store periodically

---

## Failure Scenarios
- **High CAS contention on single key** → retry storm → throughput worse than mutex
- **ABA on linked list without stamp** → node removed and re-added → CAS succeeds on stale state → list corruption
- **Multi-location CAS needed** → using multiple atomic operations separately → not atomic as a group → race condition
- **Spin-retry without backoff under contention** → livelock-like behavior → CPU waste

---

## Real-World Usage
- Java `AtomicInteger`, `AtomicLong`, `AtomicReference` — lock-free counters, references
- Java `AtomicStampedReference` — ABA-safe CAS
- Linux kernel — `cmpxchg` instruction for lock-free internal structures
- Redis `WATCH/MULTI/EXEC` — optimistic locking on Redis operations
- Redis Lua scripts — atomic execution for inventory deduction
- etcd — compare-and-swap for distributed coordination
- Database optimistic locking — version column pattern

---

## Common Mistakes
- Using plain CAS on complex data structures without addressing ABA
- Expecting CAS to scale linearly under extreme contention — retry storms are real
- Assuming CAS works across multiple memory locations — it doesn't atomically
- Not adding backoff to retry loops — wastes CPU, may degrade to livelock

---

## Interview Perspective
- "How do you increment a counter at 50,000 RPS without a mutex?" → AtomicInteger / CAS
- "What is the ABA problem?" → value changes A→B→A, CAS can't detect it — stamp/version fixes it
- "How does database optimistic locking work?" → application-level CAS with version column
- Often connected to inventory systems: "prevent overselling without killing throughput" → CAS + Redis atomic operations
- Hot key follow-up: "CAS on a flash sale item at 100k RPS" → sharding, queue-based serialization

---

## Revision Summary
- CAS: atomic read-compare-write CPU instruction. No mutex. No kernel. No sleep.
- Lock-free counter: read → compute → CAS → retry on failure.
- CAS beats mutex under low-to-medium contention. Loses under extreme contention (retry storm).
- ABA problem: value returns to original, CAS can't detect intermediate changes. Fix: stamp/version.
- Database optimistic locking = application-level CAS with version column.
- CAS operates on one memory location. Multi-location atomicity still needs mutex or transactions.
- Hot key at extreme RPS: shard the key or serialize via queue.

---

## Active Recall Questions
1. Why is CAS faster than a mutex for a shared counter under low contention?
2. Walk through the ABA problem with a concrete example. What data structures are most vulnerable?
3. How does AtomicStampedReference prevent ABA?
4. At 100,000 RPS competing on the same inventory key, CAS starts degrading. Why? What do you do?
5. How does database optimistic locking relate to hardware CAS?
6. Can you use CAS to atomically update two separate memory locations? Why or why not?
7. How would you implement atomic inventory deduction in Redis to prevent overselling?

---

## Related Concepts
- [[Mutex, Semaphore, Spinlock]]
- [[Memory Visibility and Happens-Before]]
- [[Deadlock, Livelock, Starvation]]
- [[Distributed Locks]]
- [[Optimistic vs Pessimistic Locking]]
- [[Transactions and ACID]]
