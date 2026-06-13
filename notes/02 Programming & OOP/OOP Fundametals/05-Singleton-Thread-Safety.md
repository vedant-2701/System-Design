# Singleton Pattern — Thread Safety

## Tags
#oop #java #concurrency #lld #design-patterns

---

## Overview
- Singleton ensures **only one instance** of a class exists in the JVM
- Naive implementation breaks under concurrency — two threads can create two objects
- Multiple correct implementations exist — each with different tradeoffs
- `volatile` + `synchronized` together are required for DCL — neither alone is sufficient

---

## Why Naive Singleton Breaks

```java
public class ConnectionPool {
    private static ConnectionPool instance;

    private ConnectionPool() { }

    public static ConnectionPool getInstance() {
        if (instance == null) {
            instance = new ConnectionPool(); // two threads can reach here simultaneously
        }
        return instance;
    }
}
```

**Race condition:** Thread A and Thread B both check `instance == null`, both see `null`, both create objects. Singleton broken.

---

## Fix 1 — Double-Checked Locking (DCL)

```java
public class ConnectionPool {
    private static volatile ConnectionPool instance;

    private ConnectionPool() { }

    public static ConnectionPool getInstance() {
        if (instance == null) {                    // first check — no lock (performance)
            synchronized (ConnectionPool.class) {
                if (instance == null) {            // second check — with lock (safety)
                    instance = new ConnectionPool();
                }
            }
        }
        return instance;
    }
}
```

### Why `volatile` is needed
- CPU cores have L1/L2 caches — without `volatile`, a thread may read stale cached `instance` and never see another thread's write
- `volatile` guarantees **visibility** — reads go to main memory, writes flush immediately
- `volatile` also prevents **instruction reordering**

### Why instruction reordering matters
`instance = new ConnectionPool()` is NOT atomic. JVM breaks it into:
```text
1. Allocate memory
2. Initialize object (run constructor)
3. Assign address to instance variable
```
JVM/CPU can reorder steps 2 and 3:
```text
1. Allocate memory
3. Assign address    ← reordered — instance is non-null but uninitialized
2. Initialize object
```
Another thread hits first `if (instance == null)` → sees non-null → gets **half-constructed object** → crash.

`volatile` enforces **happens-before** — all writes before `instance` assignment are visible to any thread that reads `instance` after.

### Why `synchronized` alone is not enough
`synchronized` ensures mutual exclusion but does NOT prevent instruction reordering. Half-constructed object bug still possible without `volatile`.

### Why first null check exists
Avoids acquiring lock on every call after initialization — significant performance optimization under high concurrency.

---

## Fix 2 — Initialization-on-Demand Holder (Bill Pugh)

```java
public class ConnectionPool {
    private ConnectionPool() { }

    private static class Holder {
        private static final ConnectionPool INSTANCE = new ConnectionPool();
    }

    public static ConnectionPool getInstance() {
        return Holder.INSTANCE;
    }
}
```

### Why this works without `volatile` or `synchronized`
- Static inner classes are loaded **lazily** — only when first referenced
- JVM class loading is **inherently thread-safe** (guaranteed by JVM spec)
- `INSTANCE` is initialized exactly once during class loading
- Zero explicit synchronization — JVM handles it

### Limitation
Cannot pass **caller-controlled runtime arguments** to constructor. Fixed parameters only (can read from system properties as workaround).

---

## Fix 3 — Enum Singleton (Recommended by Effective Java)

```java
public enum ConnectionPool {
    INSTANCE;

    private final int poolSize;

    ConnectionPool() {
        this.poolSize = Integer.parseInt(
            System.getProperty("DB_POOL_SIZE", "10")
        );
    }

    public int getPoolSize() { return poolSize; }
}

// Usage
ConnectionPool.INSTANCE.getPoolSize();
```

### Why Enum is the strongest Singleton
- Thread safe — JVM guarantees enum instances are initialized once
- **Serialization safe** — enum instances are inherently serialization-proof (other singletons break on deserialization)
- **Reflection safe** — cannot break via `getDeclaredConstructor().setAccessible(true)`
- Zero boilerplate

### Reflection attack on DCL Singleton
```java
Constructor<ConnectionPool> c = ConnectionPool.class.getDeclaredConstructor();
c.setAccessible(true);
ConnectionPool second = c.newInstance(); // breaks Singleton — Enum prevents this
```

### Limitation of Enum
Cannot accept **caller-controlled constructor arguments** at runtime. Parameters must come from environment or config — not from caller.

---

## Comparison

| Approach | Thread Safe | Lazy | Serialization Safe | Reflection Safe | Complexity |
|---|---|---|---|---|---|
| Naive | No | Yes | No | No | Low |
| Synchronized method | Yes | Yes | No | No | Low |
| Double-Checked Locking | Yes | Yes | No | No | High |
| Holder Pattern | Yes | Yes | No | No | Low |
| Enum | Yes | Yes | **Yes** | **Yes** | Very Low |

---

## When to Use Each

| Need | Use |
|---|---|
| Caller-controlled constructor args | DCL |
| Simplest correct implementation | Holder Pattern |
| Serialization + Reflection safety | Enum |
| Legacy Java (pre-5) | Synchronized method |

---

## Failure Scenarios
- Naive singleton under concurrent load → multiple instances → inconsistent shared state
- DCL without `volatile` → half-constructed object → NullPointerException or corrupt state
- Singleton broken via deserialization → add `readResolve()` method if not using Enum
- Singleton broken via reflection → use Enum or add guard in constructor

---

## Common Mistakes
- Using `synchronized` alone in DCL (missing `volatile`)
- Using `volatile` alone in DCL (missing `synchronized`)
- Forgetting second null check inside `synchronized` block
- Not knowing Holder Pattern or Enum Singleton in interviews
- Calling Enum constructor with runtime caller args (not possible)

---

## Interview Perspective
- DCL is a very common interview question — explain both `volatile` and `synchronized` roles separately
- Instruction reordering explanation separates strong candidates — know it cold
- Enum Singleton is the "correct" answer most interviewers expect as the best option
- Holder Pattern shows deep JVM knowledge — use it to stand out
- Always mention serialization and reflection vulnerabilities of DCL

---

## Revision Summary
- Naive Singleton breaks under concurrency — two threads create two objects
- DCL needs both `volatile` (visibility + ordering) and `synchronized` (mutual exclusion)
- `volatile` prevents instruction reordering — half-constructed object bug
- Holder Pattern: lazy, thread-safe via JVM class loading guarantee, zero synchronization
- Enum: safest — thread safe, serialization safe, reflection safe
- Enum cannot take caller-controlled constructor arguments

---

## Active Recall Questions
1. Why does naive Singleton break under concurrency?
2. Why is `volatile` alone not enough for DCL?
3. Why is `synchronized` alone not enough for DCL?
4. What is instruction reordering and how does `volatile` prevent it?
5. Why does Holder Pattern work without explicit synchronization?
6. How does Enum prevent reflection attacks?
7. When would you use DCL over Enum Singleton?

---

## Related Concepts
- [[Concurrency Basics]]
- [[Volatile and Happens-Before]]
- [[Static vs Instance Members]]
- [[Object Lifecycle — Java]]
- [[Design Patterns — Creational]]
- [[JVM Internals]]
