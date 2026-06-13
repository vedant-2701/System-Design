# Dependency Inversion & OOP Violations in Real Code

## Tags
#oop #java #lld #solid #backend

---

## Overview
- Dependency Inversion Principle (DIP): depend on abstractions, not concrete implementations
- Most real-world OOP violations stem from tight coupling, missing abstractions, and mixed responsibilities
- DIP enables testability, extensibility, and replaceability
- Violations compound — tight coupling usually brings SRP and OCP violations with it

---

## Dependency Inversion Principle (DIP)

> High-level modules should not depend on low-level modules.
> Both should depend on abstractions.
> Abstractions should not depend on details.
> Details should depend on abstractions.

### Violation
```java
class OrderService {
    private MySQLDatabase db = new MySQLDatabase();       // concrete dependency
    private EmailNotifier notifier = new EmailNotifier(); // concrete dependency

    public void placeOrder(Order order) {
        db.save(order);
        notifier.send("Order placed: " + order.getId());
    }
}
```

### What breaks
- `OrderService` knows about MySQL and Email specifically — not abstractions
- Switching to Postgres requires modifying `OrderService`
- Adding SMS alongside Email requires modifying `OrderService`
- Unit testing requires real MySQL and real email — no mocking possible
- Every change to `MySQLDatabase` or `EmailNotifier` risks breaking `OrderService`

---

## All Violations in the Above Code

### Violation 1 — Dependency Inversion (DIP)
Depends on `MySQLDatabase` and `EmailNotifier` — concrete classes, not abstractions.

### Violation 2 — Open/Closed Principle (OCP)
`OrderService` must be **modified** every time a new database or notification channel is added.
A well-designed class is **open for extension, closed for modification**.

### Violation 3 — Single Responsibility Principle (SRP)
`OrderService` has three responsibilities:
- Order business logic
- Database interaction coordination
- Notification delivery coordination

Changes to notification logic should never require touching order placement logic.

### Violation 4 — No Dependency Injection
Dependencies created internally → hardwired → **untestable**.
No way to substitute MockDatabase or MockNotifier without modifying `OrderService`.

---

## Correct Design

### Step 1 — Define Abstractions
```java
interface Database {
    void save(Order order);
}

interface NotificationService {
    void send(String message);
}
```

### Step 2 — Inject Dependencies
```java
public class OrderService {
    private final Database db;
    private final List<NotificationService> notifiers;

    public OrderService(Database db, List<NotificationService> notifiers) {
        this.db = db;
        this.notifiers = notifiers;
    }

    public void placeOrder(Order order) {
        db.save(order);
        notifiers.forEach(n -> n.send("Order placed: " + order.getId()));
    }
}
```

### Step 3 — Implement Concretions
```java
class MySQLDatabase   implements Database { ... }
class PostgresDatabase implements Database { ... }
class EmailNotifier   implements NotificationService { ... }
class SMSNotifier     implements NotificationService { ... }
class PushNotifier    implements NotificationService { ... }
```

### Result
```java
// Production
OrderService svc = new OrderService(
    new MySQLDatabase(),
    List.of(new EmailNotifier(), new SMSNotifier())
);

// Test — no real DB, no real email
OrderService svc = new OrderService(
    new MockDatabase(),
    List.of(new MockNotifier())
);
```

Add Postgres → only `PostgresDatabase` changes.
Add WhatsApp → only `WhatsAppNotifier` is new.
`OrderService` never changes. OCP satisfied.

---

## Dependency Injection Patterns

| Style | Example | When to Use |
|---|---|---|
| Constructor Injection | `new OrderService(db, notifier)` | Preferred — explicit, testable, immutable |
| Setter Injection | `service.setDatabase(db)` | Optional dependencies, reconfigurable |
| Interface Injection | Implement `DatabaseAware` | Rare — framework-specific |

**Constructor injection is preferred** — dependencies are explicit, object is always in valid state, no setter needed.

---

## Connection Resource Management in OrderService

`OrderService` should **not** manage database connection lifecycle.
That responsibility belongs to the `Database` implementation or connection pool.

```java
class MySQLDatabase implements Database {
    private final ConnectionPool pool;

    public void save(Order order) {
        try (Connection conn = pool.getConnection()) { // deterministic cleanup
            // execute query
        }
    }
}
```

`OrderService` only calls `db.save()` — has no knowledge of connections, pools, or cleanup.
Separation of responsibilities enforced through abstraction layers.

---

## Architecture Flow

```text
OrderService (high-level — business logic)
    ↓ depends on
Database interface (abstraction)
NotificationService interface (abstraction)
    ↓ implemented by
MySQLDatabase, PostgresDatabase (low-level — details)
EmailNotifier, SMSNotifier (low-level — details)
```

High-level never imports low-level directly. Both depend on the middle abstraction layer.

---

## Tell, Don't Ask — Connected Principle

DIP pairs with **Tell, Don't Ask**:
- Don't ask `db` for its connection state, check it yourself, then call methods
- Tell `db` to save the order — let it handle its own internals
- Caller knows the contract (`Database.save()`), not the implementation

---

## Tradeoffs

| Benefit | Cost |
|---|---|
| Testability — mock any dependency | More interfaces and classes to maintain |
| Extensibility — add implementations freely | Indirection adds cognitive load |
| OCP satisfied — no modification for extension | DI wiring can become complex at scale |
| SRP preserved — each class one responsibility | DI containers (Spring) add framework dependency |

---

## Failure Scenarios
- Tight coupling → any change to `MySQLDatabase` breaks `OrderService`
- No DI → cannot test `placeOrder()` without hitting real infrastructure
- No abstraction → switching databases requires rewriting business logic
- Mixed responsibilities → notification bug requires touching order logic

---

## Common Mistakes
- Creating dependencies with `new` inside service classes
- Using `static` utility classes as hidden dependencies (untestable)
- Depending on concrete classes instead of interfaces in method signatures
- Forgetting that adding a new channel means modifying the service (OCP violation)
- Not noticing that resource management leaked into business logic layer

---

## Interview Perspective
- DIP is one of the most commonly tested SOLID principles in LLD rounds
- Always identify: what are the dependencies? Are they abstractions or concretions?
- Interviewers look for: *can you unit test this without infrastructure?*
- `List<NotificationService>` for multiple channels shows extensibility thinking
- Connect DIP to testability — interviewers value this connection strongly

---

## Revision Summary
- DIP: high-level depends on abstractions, low-level implements abstractions
- Tight coupling = modification required for every new implementation = OCP violated
- DI via constructor = preferred — explicit, testable, immutable
- `List<NotificationService>` → add channels without touching OrderService
- SRP: each class one reason to change — OrderService only changes for business logic
- Resource management belongs in implementation layer, not business layer
- Tell, Don't Ask: call `db.save(order)`, don't manage connection manually

---

## Active Recall Questions
1. What is the Dependency Inversion Principle in one sentence?
2. List all OOP violations in the naive `OrderService` example.
3. Why does tight coupling make unit testing impossible?
4. How does constructor injection differ from setter injection? Which is preferred and why?
5. Why use `List<NotificationService>` instead of a single `NotificationService`?
6. Where should database connection lifecycle management live — in `OrderService` or `MySQLDatabase`?
7. How does DIP connect to Open/Closed Principle?

---

## Related Concepts
- [[SOLID Principles]]
- [[Encapsulation vs Abstraction]]
- [[Interface vs Abstract Class]]
- [[Inheritance, LSP, Composition]]
- [[Object Lifecycle — Java and Go]]
- [[Design Patterns — Creational]]
- [[Clean Architecture]]
