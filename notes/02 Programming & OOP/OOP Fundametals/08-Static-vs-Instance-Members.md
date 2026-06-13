# Static vs Instance Members

## Tags
#oop #java #jvm #concurrency #lld

---

## Overview
- **Static members** belong to the class — shared across all instances, one copy in JVM
- **Instance members** belong to the object — each instance has its own copy
- Static context has no access to instance state — a fundamental JVM constraint
- Static methods are not polymorphic — resolved at compile time on declared type

---

## Core Distinction

```java
class Counter {
    private static int totalCount = 0;  // shared — one copy per JVM
    private int instanceCount = 0;      // per object — each instance has own copy

    public Counter() { totalCount++; }

    public void increment()      { instanceCount++; }
    public int getCount()        { return instanceCount; }
    public static int getTotal() { return totalCount; }
}

Counter a = new Counter();
Counter b = new Counter();
a.increment(); a.increment();
b.increment();

System.out.println(a.getCount());        // 2 — instance state
System.out.println(b.getCount());        // 1 — instance state
System.out.println(Counter.getTotal());  // 2 — shared class state
```

---

## Static Members

### Static Fields
- One copy per JVM — all instances share the same value
- Initialized during class loading — before any constructor runs
- Common uses: constants, counters, shared config, Singleton instance reference

### Static Methods
- Belong to the class — callable without instantiation
- Cannot access instance fields or instance methods directly
- Cannot use `this` reference
- **Not polymorphic** — resolved at compile time based on declared type (method hiding, not overriding)

```java
class MathUtils {
    public static int add(int a, int b) { return a + b; } // stateless utility — correct use of static
}
```

### Static Initializer Blocks
```java
class Config {
    static final Map<String, String> SETTINGS;

    static {
        SETTINGS = new HashMap<>();
        SETTINGS.put("timeout", "30");
        SETTINGS.put("retry", "3");
        // runs once on class load — before any constructor
    }
}
```

---

## Instance Members

### Instance Fields
- Each object has its own copy — independent state
- Initialized during constructor execution
- Lifetime tied to object lifetime — GC'd when object becomes unreachable

### Instance Methods
- Operate on specific object's state via `this`
- Polymorphic — resolved at runtime via vtable
- Can access both instance and static members

---

## When to Use Static

| Use Case | Static? | Reason |
|---|---|---|
| Utility/helper methods (no state) | Yes | No object needed |
| Constants | Yes (`static final`) | Shared, immutable |
| Factory methods | Yes | Create objects without constructing first |
| Singleton instance reference | Yes | Shared across all callers |
| Counters tracking class-level state | Yes | Shared across instances |
| Business logic operating on object state | No | Needs instance context |
| Polymorphic behavior | No | Static methods cannot override |

---

## Concurrency Risk — Static Fields

Static fields are shared across all threads. Without synchronization, concurrent access causes race conditions.

```java
class ConnectionPool {
    private static int activeConnections = 0; // shared — all threads read/write this

    public static void acquire() {
        activeConnections++; // NOT thread safe — read-modify-write is not atomic
    }
}
```

Fix:
```java
private static final AtomicInteger activeConnections = new AtomicInteger(0);

public static void acquire() {
    activeConnections.incrementAndGet(); // atomic — thread safe
}
```

**Rule:** Any mutable static field accessed by multiple threads needs synchronization.

---

## Static vs Instance — Method Resolution

```java
class Base  { public static void staticMethod()   { System.out.println("Base static"); }
              public void instanceMethod() { System.out.println("Base instance"); } }

class Child extends Base {
    public static void staticMethod()   { System.out.println("Child static"); }
    public void instanceMethod() { System.out.println("Child instance"); }
}

Base obj = new Child();
obj.staticMethod();   // "Base static"  — compile-time, declared type wins
obj.instanceMethod(); // "Child instance" — runtime, actual type wins
```

Same reference. Two different resolution mechanisms. One of the most common interview trap questions.

---

## Memory Layout

```text
JVM Method Area (shared):
  ├── Class metadata
  ├── Static fields  ← one copy, shared
  └── Static methods

JVM Heap (per object):
  ├── Object header
  └── Instance fields ← each object has own copy

JVM Stack (per thread):
  └── Local variables, method frames
```

---

## Tradeoffs

| | Static | Instance |
|---|---|---|
| Memory | One copy — efficient | Per object — scales with instances |
| Polymorphism | No — compile-time resolution | Yes — runtime vtable |
| Thread safety | Must synchronize shared mutable state | Usually isolated per object |
| Testability | Harder — global state, no injection | Easier — inject per test |
| Lifecycle | Entire JVM lifetime | Object lifetime |

---

## Common Mistakes
- Mutable static fields without synchronization — race conditions under concurrency
- Calling static methods on instance references — misleading, avoid
- Putting business logic in static methods — prevents polymorphism and testing
- Using static state in web applications — shared across all requests, causes data leakage
- Thinking static method can be overridden — it is hidden, not overridden

---

## Interview Perspective
- Static vs instance resolution is a frequent trap question — know both mechanisms
- Mutable static field in multithreaded context → race condition → common follow-up
- `static final` constant vs `static` mutable field — interviewers distinguish
- Static factory method pattern — explain why static is correct here (no instance needed to create instance)
- Connect to [[Singleton Pattern]] — Singleton instance is a static field

---

## Revision Summary
- Static = class-level, one copy, no `this`, no polymorphism, JVM lifetime
- Instance = object-level, per-object copy, `this` context, polymorphic, object lifetime
- Static method on subclass = method hiding (compile-time), not overriding (runtime)
- Mutable static fields = concurrency risk — use AtomicInteger, volatile, or synchronized
- `Base obj = new Child(); obj.staticMethod()` → calls Base's — declared type wins
- Static initializer runs once on class load, before any constructor

---

## Active Recall Questions
1. What is the difference between static and instance fields in memory?
2. Why can't static methods access instance fields?
3. `Base obj = new Child(); obj.staticMethod()` — what runs? Why?
4. What concurrency risk do mutable static fields introduce?
5. When is a static method the correct design choice?
6. Why is putting business logic in static methods problematic?
7. What is the difference between method hiding and method overriding?

---

## Related Concepts
- [[Polymorphism — Static vs Dynamic Dispatch]]
- [[Singleton Pattern — Thread Safety]]
- [[Concurrency Basics]]
- [[Object Lifecycle — Java and Go]]
- [[JVM Internals]]
- [[Volatile and Happens-Before]]
