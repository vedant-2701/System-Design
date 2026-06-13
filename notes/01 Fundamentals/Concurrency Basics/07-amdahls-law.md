# Amdahl's Law

## Tags
#concurrency #scalability #performance #distributed-systems #hld

---

## Overview
- Defines the **theoretical maximum speedup** achievable by parallelizing a workload
- Sequential portions of a program impose a hard ceiling on parallelism — no amount of hardware escapes it
- Applies to threads, servers, partitions, and distributed systems at every level of scale
- Key insight: find the sequential fraction → break it into independent parallel units

---

## The Formula

```
Speedup = 1 / (S + (1 - S) / N)

S = fraction of work that is strictly sequential
N = number of processors/cores/nodes
(1 - S) = fraction that can be parallelized
```

### Example: 10% Sequential, 90% Parallelizable

| Cores (N) | Speedup |
|---|---|
| 1 | 1x |
| 10 | 5.26x |
| 100 | 9.17x |
| 1,000 | 9.91x |
| ∞ | **10x** |

Maximum speedup = `1 / S = 1 / 0.10 = 10x`. Infinite cores cannot exceed this.

### Why There's a Ceiling
Even with infinite cores, the sequential 10% still runs on one core. Total time never drops below that sequential portion. Adding cores only reduces the parallel fraction toward zero — it cannot reduce the sequential fraction.

---

## Diminishing Returns

```
Speedup
  10x │                                    ●●●●●●●
      │                              ●●●●●
      │                        ●●●●
   5x │                  ●●●
      │           ●●●
      │      ●●
   1x │●
      └──────────────────────────────────────────
      1   2   4   8  16  32  64  128  ∞    Cores
```

Most gains come from the first few cores. Doubling cores from 512→1024 gives negligible improvement if sequential fraction dominates.

---

## Real Speedup Is Worse Than Amdahl Predicts

Amdahl assumes parallelization is free. It isn't. Actual overhead:

- Thread creation and destruction
- Lock contention on shared resources
- **Cache invalidation across cores** — when one core writes, others invalidate their cached copy
- Data partitioning and result merging
- Synchronization barriers — threads waiting at checkpoints

```
Actual Speedup = 1 / (S + (1 - S) / N + overhead(N))
```

Overhead grows with N. At some point, adding threads **decreases** throughput. This is why oversizing a CPU-bound thread pool hurts performance.

---

## Amdahl's Law in Distributed Systems

The sequential fraction doesn't just appear in code. It appears in **architecture**. Wherever serialization exists, Amdahl's ceiling applies.

### Database Write Bottleneck
```
Reads  → replicas → horizontally scalable
Writes → single leader → serialized
```
If 30% of traffic is writes through one leader: `Maximum speedup = 1 / 0.30 = 3.3x`
Adding read replicas indefinitely cannot overcome the write bottleneck.
**Fix**: shard into multiple leaders — each owns a partition → write bottleneck is partitioned.

### Kafka Partitions
```
1 partition  → 1 consumer  → sequential processing
10 partitions → 10 consumers → parallel processing
```
Partition count directly determines throughput ceiling. Partition count is the N in Amdahl's formula.

### Global Lock in Application Code
```java
synchronized(globalLock) {
    // 5% of request processing
}
// 95% outside the lock
```
`Maximum speedup = 1 / 0.05 = 20x`
One global lock caps scaling at 20x regardless of how many servers you add.
**Fix**: reduce lock granularity — each user/order/SKU gets its own lock rather than one global lock.

### Inventory System Example
```
Before: global inventory lock → S = 0.20 → ceiling = 5x
After: per-SKU lock → S per SKU is tiny → near-linear scaling across SKUs
```

---

## Identifying the Sequential Fraction in Practice

| System Component | Sequential Bottleneck | Fix |
|---|---|---|
| DB with single leader | All writes serialized | Sharding — multiple leaders |
| Global application lock | All requests serialized through lock | Per-entity locks, CAS |
| Single Kafka partition | All messages processed in order | More partitions |
| Single-threaded queue consumer | All jobs sequential | Parallel consumers |
| Coordinator service | All nodes check in → single point | Partitioned coordination |

---

## Gustafson's Law — The Counterpoint

Amdahl assumes **fixed problem size**. In reality, adding hardware often means handling **more work**, not the same work faster.

- **Amdahl**: fixed workload → what's the speedup ceiling?
- **Gustafson**: growing workload → parallel systems scale well if sequential fraction stays constant in absolute time

Both laws are useful:
- Amdahl → "how fast can I process this batch with more cores?"
- Gustafson → "how much more traffic can I handle with more servers?"

---

## Tradeoffs

| | |
|---|---|
| Insight | Hard ceiling on scaling — prevents over-investment in hardware |
| Insight | Identifies where to focus optimization — the sequential fraction |
| Limitation | Ignores parallelization overhead — real speedup is worse |
| Limitation | Fixed problem size assumption — Gustafson's Law complements it |

---

## Failure Scenarios
- **Ignoring sequential fraction** → adding servers/cores with no throughput gain → wasted infrastructure cost
- **Global lock undetected** → system appears to scale but hits invisible ceiling → only discoverable under load testing
- **Single Kafka partition for high-throughput topic** → consumer becomes bottleneck → partition key design critical
- **Oversized CPU thread pool** → coordination overhead exceeds parallelism gains → throughput decreases

---

## Real-World Usage
- **Kafka partition design** — partition count chosen based on required consumer parallelism
- **Database sharding decisions** — shard when single-leader write throughput becomes the bottleneck
- **Thread pool sizing** — cores+1 for CPU-bound (more threads = more overhead, not more speed)
- **Load testing** — Amdahl explains why throughput plateaus before servers are fully utilized

---

## Common Mistakes
- Assuming more hardware always solves throughput problems
- Optimizing the parallel 90% while ignoring the sequential 10% bottleneck
- Not measuring which fraction of the system is sequential before scaling
- Partitioning without understanding which operations still serialize (e.g. cross-shard transactions)

---

## Interview Perspective
- Often embedded in system design: "you add 10x more servers but throughput only 3x — why?"
- Answer: sequential bottleneck — identify it (single DB leader, global lock, single partition) and break it
- Demonstrates senior-level thinking: quantify the ceiling, explain the architectural fix
- Common in Kafka/database design discussions — partition count justification

---

## Revision Summary
- `Speedup = 1 / (S + (1-S)/N)`. Sequential fraction S sets the hard ceiling.
- `Maximum speedup = 1/S`. Infinite cores can't beat it.
- Parallelization overhead makes real speedup worse than formula predicts.
- In distributed systems: single leader, global locks, single partitions are all sequential fractions.
- Fix: shard the sequential bottleneck into independent parallel units.
- Gustafson's Law: for growing workloads, scaling is more useful than Amdahl's fixed-problem analysis suggests.

---

## Active Recall Questions
1. A program is 20% sequential. What is the maximum possible speedup with infinite cores?
2. You add 5x more servers and get only 2x throughput improvement. What does Amdahl's Law tell you?
3. Your order service has a global lock protecting 15% of request processing. What's the scaling ceiling and how do you break it?
4. Why does oversizing a CPU-bound thread pool decrease throughput?
5. How does Kafka's partition model express Amdahl's Law?
6. What does Gustafson's Law say that Amdahl's doesn't account for?

---

## Related Concepts
- [[Thread Pools]]
- [[Concurrency Models — Threads vs Event Loop vs Async Await]]
- [[Compare-and-Swap (CAS)]]
- [[Sharding and Partitioning]]
- [[Kafka — Topics, Partitions, Consumer Groups]]
- [[Database Replication]]
