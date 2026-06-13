# Object Lifecycle — Java & Go

## Tags
#oop #java #go #jvm #memory #concurrency

---

## Overview
- Object lifecycle = from class loading to garbage collection
- Java and Go both have GC — but resource cleanup mechanisms differ significantly
- Java: `AutoCloseable` + try-with-resources — deterministic cleanup
- Go: `defer` — scope-based deterministic cleanup
- Neither language should rely on GC-driven cleanup for critical resources

---

## Java Object Lifecycle

```text
1. Class Loading      — JVM loads bytecode, resolves dependencies
2. Static Init        — static blocks run in order, exactly once
3. Memory Allocation  — heap memory reserved for object
4. Initialization     — constructor runs, fields set
5. Active Use         — object referenced, methods called
6. Unreachable        — no references point to object
7. GC Collection      — GC reclaims heap memory
```

### Static Initializer Blocks
```java
class Example {
    static { System.out.println("First");  }  // runs first on class load
    static { System.out.println("Second"); }  // runs second — LIFO order
}
```
- Runs **exactly once** per JVM — when class is first loaded
- Multiple blocks allowed — run **top to bottom**
- Runs before any constructor or instance method

---

## Why `finalize()` is Broken

```java
// DO NOT DO THIS
protected void finalize() {
    conn.close(); // relies on GC timing
}
```

| Problem | Consequence |
|---|---|
| No timing guarantee | GC runs at JVM discretion — could be never |
| No execution guarantee | JVM exit may skip finalizers entirely |
| Two GC cycles required | Finalizable objects live longer — GC pressure |
| Exceptions silently swallowed | Cleanup failure is invisible |
| Resurrection risk | `this` assigned to static field in finalizer → memory leak |

**`finalize()` deprecated in Java 9, scheduled for removal.**

Tying critical resource cleanup to step 7 (GC) means you don't control when it happens.

---

## Correct Java Resource Cleanup — AutoCloseable

```java
public class DatabaseConnection implements AutoCloseable {
    private Connection conn;

    public DatabaseConnection(String url) throws SQLException {
        this.conn = DriverManager.getConnection(url);
    }

    public void executeQuery(String sql) { ... }

    @Override
    public void close() throws SQLException {
        if (conn != null && !conn.isClosed()) conn.close();
    }
}
```

```java
// try-with-resources — close() called deterministically on block exit
try (DatabaseConnection db = new DatabaseConnection(url)) {
    db.executeQuery("SELECT * FROM orders");
} // close() guaranteed here — normal exit OR exception
```

- Cleanup tied to **scope boundary**, not GC
- Works correctly even when exception is thrown inside the block
- Multiple resources closed in **reverse order** of declaration

---

## Go Object Lifecycle

Go also has GC (concurrent, tricolor mark-and-sweep, low-latency). But cleanup is handled differently.

```text
1. Variable declared  — compiler performs escape analysis
2. Allocation         — stack (local) or heap (escaping) based on escape analysis
3. Active Use         — variable in scope, accessible
4. Scope exit         — defer runs (LIFO), stack variables reclaimed
5. Unreachable        — heap objects awaiting GC
6. GC Collection      — GC reclaims heap memory
```

---

## Go Escape Analysis — Stack vs Heap

Compiler automatically decides allocation location — developer does not control this explicitly.

```go
func createUser() *User {
    u := User{Name: "Alice"} // escapes — returned pointer → heap
    return &u
}

func processLocally() {
    u := User{Name: "Bob"} // does not escape → stack (cheaper, no GC pressure)
    fmt.Println(u.Name)
}
```

- Stack allocation: cheaper, automatically reclaimed on function return
- Heap allocation: GC managed, needed when value outlives its scope
- Understanding escape analysis helps write **GC-pressure-friendly** Go code

---

## Go Resource Cleanup — defer

`defer` guarantees a function call runs **when the enclosing function returns** — regardless of normal return or panic.

```go
func processOrder(orderID string) error {
    db, err := sql.Open("postgres", connString)
    if err != nil { return err }
    defer db.Close()  // guaranteed — even if panic occurs

    rows, err := db.Query("SELECT * FROM orders WHERE id = $1", orderID)
    if err != nil { return err }
    defer rows.Close() // also guaranteed

    return nil
}
```

### defer Stack — LIFO Execution
```go
func example() {
    defer fmt.Println("First")   // runs third
    defer fmt.Println("Second")  // runs second
    defer fmt.Println("Third")   // runs first
}
// Output: Third → Second → First
```
LIFO mirrors natural resource unwinding — open A then B → close B then A.

---

## Critical defer Pitfall — Loop Scope

```go
// WRONG — all files stay open until function returns
func processFiles(files []string) error {
    for _, file := range files {
        f, err := os.Open(file)
        if err != nil { return err }
        defer f.Close() // deferred to function exit, NOT loop iteration
        // process f
    }
    return nil
}
```

With 1000 files → 1000 file descriptors open simultaneously → OS fd limit hit → crash.

```go
// CORRECT — extract to function, defer scoped to processFile()
func processFiles(files []string) error {
    for _, file := range files {
        if err := processFile(file); err != nil { return err }
    }
    return nil
}

func processFile(path string) error {
    f, err := os.Open(path)
    if err != nil { return err }
    defer f.Close() // scoped to processFile — closes after each iteration
    // process f
    return nil
}
```

**Rule:** `defer` is **function-scoped**, not block-scoped. Inside loops — extract to function or close explicitly.

---

## Go `runtime.SetFinalizer` — Avoid

Same problems as Java's `finalize()`:
- No timing guarantee
- May never run
- Not suitable for critical resources

```go
// Avoid — same problems as Java finalize()
runtime.SetFinalizer(obj, func(o *MyType) { o.Close() })

// Always prefer defer
defer obj.Close()
```

---

## Java vs Go — Resource Management Comparison

| Concept | Java | Go |
|---|---|---|
| GC | Generational, configurable | Concurrent, low-latency, short pauses |
| Stack allocation | Primitives only | Compiler decides via escape analysis |
| Heap allocation | All objects | Escaping values |
| Resource cleanup | `AutoCloseable` + try-with-resources | `defer` |
| Finalizer | `finalize()` — deprecated | `runtime.SetFinalizer` — avoid |
| Concurrency unit | Thread (~1MB fixed stack) | Goroutine (~2KB, grows dynamically) |
| Goroutine/Thread count | Thousands (limited by stack) | Millions (small initial stack) |

---

## Production Consequence — Connection Pool Exhaustion

Failing to close connections deterministically causes:
```text
- Requests timing out
- "Too many connections" database errors
- Threads/goroutines stuck waiting for connections
- Health checks failing
- Cascading service failures
```

Root cause: connections not closed on exception paths, or relying on GC/finalizer for cleanup.

---

## Failure Scenarios
- Java: relying on `finalize()` → connections never closed under low GC pressure → pool exhaustion
- Go: `defer` inside loop → file descriptor exhaustion under large file sets
- Java: exception thrown inside try block without try-with-resources → resource leaked
- Go: panic without `defer` → cleanup skipped

---

## Common Mistakes
- Java: using `finalize()` for DB connection cleanup
- Java: forgetting try-with-resources on exception paths
- Go: putting `defer` inside for loops without extracting function
- Go: assuming `defer` runs at block exit (it's function exit)
- Both: thinking GC will handle critical resource cleanup in time

---

## Interview Perspective
- Java: explain why `finalize()` is broken — timing, no guarantee, two GC cycles, silent exceptions
- Go: explain defer LIFO semantics and the loop pitfall
- Escape analysis in Go — shows deep understanding of memory model
- Connection pool exhaustion is a real production incident scenario — interviewers value this awareness
- Connect to [[Concurrency Basics]] — goroutine stack growth vs thread fixed stack

---

## Revision Summary
- Java lifecycle: class load → static init → allocate → construct → use → unreachable → GC
- `finalize()` — no timing guarantee, may never run, two GC cycles, deprecated Java 9
- `AutoCloseable` + try-with-resources = deterministic Java cleanup
- Go: escape analysis decides stack vs heap — developer does not control explicitly
- `defer` = function-scoped LIFO cleanup — not block-scoped
- `defer` in loop = all cleanup deferred to function return → fd exhaustion
- Fix: extract loop body to function so defer is scoped per iteration
- Never rely on GC (Java or Go) for critical resource cleanup

---

## Active Recall Questions
1. Why is `finalize()` dangerous for database connection cleanup?
2. What are the two GC cycles problem with finalizable objects?
3. How does try-with-resources handle exceptions differently from finally?
4. What is escape analysis in Go and what does it decide?
5. What order do deferred functions execute in? Why?
6. What happens when you put `defer f.Close()` inside a for loop processing 1000 files?
7. How do you correctly handle per-iteration cleanup in Go?
8. What is the goroutine stack size vs Java thread stack size? Why does it matter?

---

## Related Concepts
- [[Singleton Pattern — Thread Safety]]
- [[Concurrency Basics]]
- [[Static vs Instance Members]]
- [[Go Concurrency — Goroutines and Channels]]
- [[JVM Internals]]
- [[Connection Pooling]]
