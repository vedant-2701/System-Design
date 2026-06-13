# Interface vs Abstract Class

## Tags
#oop #java #lld #design-patterns

---

## Overview
- Both define contracts and enable polymorphism — but solve different problems
- **Interface** = capability contract — what a type *can do*
- **Abstract Class** = partial implementation + type identity — what a type *is*
- Java 8 default methods blur the line — but key distinctions remain

---

## Full Comparison

| Feature | Interface | Abstract Class |
|---|---|---|
| Instance fields | No | Yes |
| Constructor | No | Yes |
| Access modifiers on methods | `public` only (pre Java 9) | Any (`private`, `protected`, `public`) |
| Multiple inheritance | Yes — implement many | No — extend only one |
| Concrete methods | Yes (Java 8+ `default`) | Yes always |
| Initialization guarantee | No | Yes — via constructor |
| IS-A relationship | Capability contract | True type hierarchy |
| State management | Cannot | Can — with invariants |

---

## When to Use Interface

Use when:
- Defining a **capability** shared across unrelated types
- Multiple inheritance of behavior is needed
- No shared state required
- You want maximum flexibility for implementors

```java
interface Flyable  { void fly(); }
interface Swimmable { void swim(); }

class Duck     implements Flyable, Swimmable { ... }
class Seaplane implements Flyable, Swimmable { ... }
// Duck and Seaplane share capabilities, not identity
```

---

## When to Use Abstract Class

Use when:
- True type hierarchy with **shared state** exists
- Subclasses need guaranteed initialization (constructor)
- Protected implementation must be shared with access control
- You need to enforce a **behavioral sequence** subclasses cannot break

```java
abstract class DatabaseConnection {
    private String url;           // shared state
    private String credentials;   // shared state
    protected Connection conn;

    public DatabaseConnection(String url, String creds) {
        this.url = url;           // guaranteed initialization
        this.credentials = creds;
    }

    public final void connect() { // sequence enforced — final
        validateCredentials();
        conn = openConnection(url);
        onConnected();
    }

    protected abstract Connection openConnection(String url);
    protected abstract void onConnected();
    private void validateCredentials() { ... } // hidden from subclasses
}
```

Interface cannot enforce this — `default` methods can be overridden, breaking the sequence.

---

## Template Method Pattern

Abstract class enables the **Template Method Pattern** — define algorithm skeleton, subclasses fill steps.

```java
abstract class Logger {
    public final void log(String message) { // skeleton — cannot override
        String formatted = format(message); // step 1
        write(formatted);                   // step 2
    }
    protected abstract String format(String message);
    protected abstract void write(String formatted);
}

class JsonLogger extends Logger {
    protected String format(String msg) { return "{\"msg\":\"" + msg + "\"}"; }
    protected void write(String s) { /* write to file */ }
}
```

- `log()` is `final` — sequence is unbreakable
- Subclasses control *how* each step works, not *which* steps run
- Interface with `default` cannot guarantee this — callers can override

---

## Java 8+ Default Methods — The Blurred Line

```java
interface PaymentService {
    void processPayment(double amount);         // abstract

    default void logPayment(double amount) {    // concrete — optional override
        System.out.println("Processing: " + amount);
    }
}
```

What default methods **cannot** provide:
- Instance fields (no state)
- Constructors (no initialization guarantee)
- Protected/private access modifiers (methods are public)
- Unbreakable behavioral sequences (can be overridden)

Abstract class remains necessary when any of the above are required.

---

## Abstract Class Constructor — Common Misconception

```java
abstract class Animal {
    protected String name;
    public Animal(String name) { this.name = name; }
}

class Dog extends Animal {
    public Dog(String name) { super(name); } // calls Animal constructor
}
```

`super(name)` does **NOT** create a separate `Animal` object.
There is **one object** — `Dog` — on the heap.
The Animal constructor runs to **initialize the Animal portion** of the Dog object.

---

## Tradeoffs

| Decision | Benefit | Cost |
|---|---|---|
| Interface | Multiple inheritance, loose coupling | No state, no initialization guarantee |
| Abstract class | State, constructor, access control, `final` methods | Single inheritance only |
| Default methods | Backward compatibility in interfaces | Can be overridden — no sequence guarantee |

---

## Common Mistakes
- Defaulting to abstract class when interface suffices (increases coupling)
- Using interface when shared state/initialization is needed
- Thinking Java 8 default methods make abstract class obsolete — they don't
- Saying "abstract class gets instantiated" — wrong, only its constructor runs within subclass object

---

## Interview Perspective
- Interviewers ask: *"When would you use abstract class over interface?"* — answer with state, constructor, and `final` method enforcement
- Template Method Pattern is the canonical abstract class use case — know it cold
- Java 8 default methods are a common follow-up — explain what they still cannot do
- Connect to [[Template Method Pattern]] and [[SOLID Principles]]

---

## Revision Summary
- Interface = capability contract, multiple inheritance, no state
- Abstract class = type identity, shared state, constructor, `final` enforcement
- Java 8 default methods blur but don't eliminate the distinction
- Template Method = abstract class enforcing algorithm skeleton — interface cannot guarantee sequence
- Abstract class constructor runs inside subclass object — no separate object created
- Rule: interface for *can-do*, abstract class for *is-a + shared skeleton*

---

## Active Recall Questions
1. What can abstract class do that interface still cannot after Java 8?
2. Why can't interface enforce a method execution sequence?
3. What is Template Method Pattern and why does it require abstract class?
4. Does `super()` create a new object? What actually happens?
5. When would you use interface over abstract class for a payment system?
6. What changed in Java 8 for interfaces and what are its limits?

---

## Related Concepts
- [[Encapsulation vs Abstraction]]
- [[Inheritance, LSP, Composition]]
- [[Template Method Pattern]]
- [[SOLID Principles]]
- [[Polymorphism — Static vs Dynamic Dispatch]]
- [[Design Patterns — Behavioral]]
