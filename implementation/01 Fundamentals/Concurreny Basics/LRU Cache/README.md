# LRU Cache — Implementation README

## Problem Statement

A cache improves read throughput by serving frequently accessed data from fast memory rather than slow storage (database, disk, remote service). But memory is finite — a cache must decide what to keep and what to discard when full.

**Least Recently Used (LRU)** is an eviction policy based on access recency: the entry that was accessed least recently is evicted first. The assumption is that recently accessed data is more likely to be accessed again than data that hasn't been touched in a while — the **temporal locality** principle.

LRU caches appear everywhere in production systems:

- **Database query caches** — avoid re-executing expensive queries
- **CDN edge nodes** — keep hot content in memory, evict cold content
- **DNS resolvers** — cache resolved records, evict stale ones
- **OS page cache** — keep recently accessed memory pages in RAM
- **Application-level caches** — Redis, Memcached both support LRU eviction

Getting LRU wrong under concurrency causes silent data corruption — entries appear or disappear unpredictably, cache size drifts beyond capacity, or the JVM/runtime crashes with a null pointer exception inside a data structure that was never designed to be shared.

---

## Design Overview

### Core Data Structures

Two data structures are composed to satisfy the O(1) constraint on both `get` and `put`:

```
HashMap<K, Node>          Doubly Linked List
┌──────────────┐         HEAD ↔ [C] ↔ [B] ↔ [A] ↔ TAIL
│ "a" → Node_A │              MRU               LRU
│ "b" → Node_B │
│ "c" → Node_C │         head.next = MRU node
└──────────────┘         tail.prev = LRU node (eviction candidate)
```

- **HashMap** — O(1) key lookup. Stores a direct pointer to the node in the linked list.
- **Doubly Linked List** — maintains access order. Head end = most recently used. Tail end = least recently used.
- **Sentinel nodes** (dummy head and tail) — eliminate boundary null checks on every list operation. Every real node always has a valid `prev` and `next`.

### Key stored on Node

Each node stores its own key. This is intentional. When the LRU node at `tail.prev` is evicted, its key must be removed from the HashMap simultaneously. Without the key on the node, you would need a reverse scan of the HashMap — O(n). Storing the key makes eviction O(1).

### Component Responsibilities

| Component | Responsibility |
|---|---|
| `HashMap` | O(1) lookup — maps key to list node |
| `Doubly Linked List` | Tracks access order — O(1) reordering |
| `ReentrantLock` / `sync.Mutex` | Mutual exclusion — prevents concurrent state corruption |
| Sentinel nodes | Simplify list operations — no null boundary checks |
| `Cache` interface (Java) | Defines contract — enables mocking and alternative implementations |

### Operation Flow

**`get(key)`:**
```
1. Acquire lock
2. Lookup key in HashMap → miss: return null/zero
3. Hit: move node to head (mark MRU)
4. Return value
5. Release lock
```

**`put(key, value)`:**
```
1. Acquire lock
2. Key exists? → update value in-place, move to head, return
3. Key new → create node, insert at head, add to HashMap
4. Size > capacity? → remove tail.prev (LRU), delete from HashMap
5. Release lock
```

---

## Why This Approach Was Chosen

### Single Lock Over ReadWriteLock

The instinct for a read-heavy cache is to reach for `ReadWriteLock` — multiple concurrent readers, exclusive writers. However, LRU's access-ordering requirement makes `get` a structural write: every read moves a node to the head of the list.

Since `get` mutates shared state on every call, every `get` needs the write lock. `ReadWriteLock` provides zero benefit — it adds complexity with no throughput gain. A single `ReentrantLock`/`sync.Mutex` is simpler, equally correct, and easier to reason about.

This is a case where the "obvious optimization" would make the code more complex without making it faster.

### Sentinel Nodes Over Null Checks

Without sentinels, `insertAtHead` and `removeNode` require special-case logic for the list boundaries:

```java
// Without sentinels — fragile
void insertAtHead(Node node) {
    if (head == null) {
        head = node;
        tail = node;
    } else {
        node.next = head;
        head.prev = node;
        head = node;
    }
}
```

With sentinels, every node operation is uniform — no special cases, no null pointer risk:

```java
// With sentinels — clean
void insertAtHead(Node node) {
    node.prev = head;
    node.next = head.next;
    head.next.prev = node;
    head.next = node;
}
```

Fewer branches, fewer bugs, easier to test.

### Update In-Place on Duplicate Key

When `put` is called with an existing key, there are two options:

1. Remove the old node, create a new node, insert at head
2. Update value in-place on the existing node, move to head

Option 2 is strictly better — no new object allocation, no garbage collection pressure, no HashMap churn. The node pointer in the HashMap remains valid.

### `finally` / `defer` for Lock Release

A lock acquired inside a method must be released even if an exception is thrown mid-operation. Without a `finally` block (Java) or `defer` (Go), any runtime exception leaves the lock permanently held — every subsequent thread blocks indefinitely. This is a real production deadlock pattern that appears under load when unexpected exceptions occur.

---

## Alternatives Considered

### `LinkedHashMap` (Java)

Java's standard library provides `LinkedHashMap` with `accessOrder=true`, which internally maintains LRU ordering. It can be extended to a bounded LRU cache in ~10 lines:

```java
new LinkedHashMap<K, V>(capacity, 0.75f, true) {
    protected boolean removeEldestEntry(Map.Entry eldest) {
        return size() > capacity;
    }
};
```

**Advantages:** Minimal code, well-tested standard library implementation.

**Disadvantages:** Not thread-safe — requires external synchronization, which puts you back to the same locking design. The access-order internals are not visible, making it harder to reason about concurrency behavior. Building from scratch here builds deeper understanding of the data structure — which is the point of this exercise.

### `ConcurrentHashMap` + Approximate LRU

Java's `ConcurrentHashMap` allows fine-grained concurrent access without a global lock. Combined with approximate LRU (probabilistic eviction — sample N random entries, evict the least recently used among them), you can avoid structural list operations on every `get`.

**Advantages:** Higher read throughput under contention — no global write lock on every access. This is the approach Redis uses.

**Disadvantages:** Eviction is approximate, not exact. Some recently accessed entries may be evicted; some stale entries may survive longer than they should. For most caches this is acceptable, but it adds implementation complexity and the approximation must be understood by anyone operating the system.

Not chosen here because correctness and clarity are the primary goals. Approximate LRU is a valid production optimization once the correct baseline is understood.

### Segmented Locking

Split the cache into N independent shards, each with its own lock. A key hashes to a shard; only that shard's lock is acquired.

**Advantages:** Contention reduced by factor of N. Throughput scales with shard count.

**Disadvantages:** Cross-shard operations are impossible. Cache size management becomes approximate — each shard evicts independently, global capacity is not enforced precisely. Significantly more complex to implement and test correctly.

Not chosen — premature optimization for this baseline implementation. Appropriate when profiling demonstrates lock contention is actually the bottleneck.

---

## Complexity Analysis

| Operation | Time Complexity | Notes |
|---|---|---|
| `get` | O(1) | HashMap lookup + list reorder (constant pointer ops) |
| `put` (new key) | O(1) | HashMap insert + list insert at head |
| `put` (existing key) | O(1) | HashMap lookup + in-place update + list reorder |
| `put` with eviction | O(1) | Tail removal + HashMap delete — both constant |
| `clear` | O(n) | HashMap clear iterates all entries |

**Space complexity:** O(capacity) — bounded by design. Each entry occupies one HashMap slot and one linked list node.

**Concurrency characteristics:** Single global lock means operations serialize. Under high contention, throughput is bounded by lock acquisition rate. Acceptable for moderate concurrency. For very high throughput requirements, segmented locking or approximate LRU should be evaluated.

---

## Edge Cases

| Scenario | Behavior |
|---|---|
| `get` on missing key | Returns `null` (Java) / `(zero, false)` (Go) |
| `put` with duplicate key | Updates value in-place, moves to MRU position, size unchanged |
| `put` when at capacity | Inserts new entry, evicts LRU entry, size stays at capacity |
| Capacity = 1 | Second `put` always evicts the first entry |
| Capacity ≤ 0 | Constructor throws `IllegalArgumentException` (Java) / returns `error` (Go) |
| Null key (Java) | `put` throws `IllegalArgumentException` — HashMap permits null keys but null keys in a cache create silent bugs |
| Two concurrent `put` calls at capacity | Both operate inside the lock — check + evict + insert is atomic. No double eviction possible. |
| Exception inside `get`/`put` | `finally`/`defer` guarantees lock release — no deadlock |

The concurrency edge case is the most important. Without the lock wrapping the **entire** `put` operation:

```
Thread A: checks size == capacity → decides to evict
Thread B: checks size == capacity → decides to evict  ← context switch
Thread A: evicts tail, inserts new node → size = capacity
Thread B: evicts tail again (now wrong node), inserts → size = capacity - 1
```

Cache is now permanently one entry smaller than intended. Subtle, timing-dependent, hard to reproduce in tests without deliberate concurrent load.

---

## Production Considerations

### What This Implementation Lacks

**TTL (Time-To-Live):** Entries are evicted only by capacity pressure, never by age. In production, stale data is as harmful as no data. A production cache needs per-entry expiry — either lazy expiry (check TTL on `get`, return miss if expired) or eager expiry (background thread scanning for expired entries).

**Metrics and Observability:** Without instrumentation, you cannot answer: Is this cache helping? What is the hit rate? How often is eviction happening? A production cache should expose at minimum:
- Hit count / miss count → hit rate
- Eviction count
- Current size vs capacity
- Average entry age

These should be exposed as metrics (Prometheus counters/gauges) and visible in dashboards.

**Logging:** Cache misses on hot keys and frequent evictions are signals of undersized caches or poor key design. Structured log entries on eviction with key metadata help diagnose these issues.

**Capacity Planning:** Cache capacity should be set based on working set size — the set of entries that are actually accessed repeatedly. Setting capacity too low causes thrashing; too high wastes memory. In production, monitor eviction rate: high eviction rate under normal load indicates undersizing.

### Deployment Considerations

This is an **in-process cache** — state lives in the JVM/process heap and is not shared across instances. In a multi-instance deployment:

- Each instance has an independent cache — no consistency between instances
- A key cached on instance A is a cache miss on instance B
- Acceptable for read-heavy, eventually consistent data (user profiles, product catalog)
- Not acceptable for data requiring cross-instance consistency

For cross-instance consistency, a **distributed cache** (Redis, Memcached) replaces or supplements the in-process cache. In-process cache remains useful as an L1 cache in front of Redis — reduces Redis round-trips for the hottest entries.

---

## Future Improvements

### TTL Support

Add per-entry expiry with lazy cleanup:

```java
void put(K key, V value, Duration ttl) { ... }

V get(K key) {
    // check expiry on access — evict and return miss if expired
}
```

Background sweeper thread for eager cleanup of expired entries that are never accessed again.

### Cache Statistics

```java
public CacheStats stats() {
    return new CacheStats(hitCount, missCount, evictionCount, size());
}
```

Expose via JMX, Micrometer, or Prometheus endpoint.

### Segmented LRU (SLRU)

Divide cache into two segments: probationary (new entries) and protected (confirmed hot entries). An entry moves from probationary to protected after a second access. Eviction targets the probationary segment first. Prevents a one-time scan of many unique keys from evicting genuinely hot entries — a real problem with pure LRU under scan workloads.

### Approximate LRU for Higher Throughput

Remove the global write lock from `get` by making ordering approximate. Track last access time per entry. On eviction, sample K random entries and evict the one with the oldest access time. Eliminates list reordering on every read — significantly higher read throughput under contention. Correctness tradeoff: eviction is no longer exactly LRU.

---

## Multi-Language Design Notes

### Java vs Go — Conceptual Mapping

| Concept | Java | Go |
|---|---|---|
| Contract definition | `Cache<K,V>` interface | Implicit — any struct with matching methods |
| Locking | `ReentrantLock` with `try/finally` | `sync.Mutex` with `defer` |
| Lock release guarantee | `finally` block — explicit | `defer mu.Unlock()` — registered at lock site |
| Error on bad input | Throw `IllegalArgumentException` | Return `(nil, error)` from constructor |
| Cache miss signal | Return `null` | Return `(zero, false)` — explicit ok boolean |
| Generic constraints | `HashMap<K,V>` — K must be hashable implicitly | `[K comparable, V any]` — constraint explicit in signature |
| Node visibility | Package-private class | Unexported struct (lowercase) |

### Lock Release — `finally` vs `defer`

Both guarantee lock release on any exit path including panics/exceptions. Go's `defer` is strictly more readable — it appears immediately after `Lock()` and is impossible to accidentally omit. Java's `try/finally` pattern is correct but requires a separate block structure that can be forgotten in large methods.

```go
// Go — impossible to forget unlock
c.mu.Lock()
defer c.mu.Unlock()
// ... rest of method
```

```java
// Java — must remember finally on every lock acquisition
lock.lock();
try {
    // ...
} finally {
    lock.unlock(); // easy to omit in complex refactors
}
```

### Error Handling Philosophy

Java uses exceptions for constructor validation — consistent with the language convention where constructors cannot return values. Go uses multiple return values — `(T, error)` — which forces the caller to explicitly handle the error at the call site. Go's approach makes error handling visible in the code; Java's makes it invisible until the exception is thrown at runtime.

### Interface Design

Java's explicit `Cache<K,V>` interface enables compile-time contract enforcement, easy mocking in tests (`Mockito.mock(Cache.class)`), and dependency injection. Go's implicit interfaces — any struct implementing the right methods satisfies the interface — provide the same benefit with less ceremony. Both approaches serve the same engineering goal: decouple the caller from the implementation.
