# Polymorphism — Static vs Dynamic Dispatch

## Tags
#oop #java #jvm #lld

---

## Overview
- Polymorphism = one interface, multiple behaviors
- Two resolution mechanisms: **compile-time (static dispatch)** and **runtime (dynamic dispatch)**
- Overloading → static dispatch. Overriding → dynamic dispatch.
- Static methods are **never polymorphic** — common source of production bugs

---

## Compile-Time Polymorphism — Method Overloading

- Same method name, different parameter types/count
- Compiler resolves the correct method at **compile time** based on declared argument types
- Called **static dispatch** — decision baked into bytecode

```java
class PaymentProcessor {
    public void process(CreditCard card) { ... }
    public void process(UPI upi)         { ... }
    public void process(NetBanking nb)   { ... }
}
```

Compiler looks at argument type → locks in correct `process()` overload at compile time.

---

## Runtime Polymorphism — Method Overriding

- Same method name, same parameters, different implementation in subclass
- JVM resolves the correct method at **runtime** based on actual object type
- Called **dynamic dispatch** — decision made via vtable lookup

```java
class NotificationService {
    public void send(String msg) { System.out.println("Base"); }
}

class EmailService extends NotificationService {
    @Override
    public void send(String msg) { System.out.println("Email: " + msg); }
}

class SMSService extends NotificationService {
    @Override
    public void send(String msg) { System.out.println("SMS: " + msg); }
}

NotificationService svc = new EmailService();
svc.send("Hello");  // prints "Email: Hello" — runtime decision
```

Declared type is `NotificationService` — compiler only knows that.
Actual object is `EmailService` — JVM resolves to `EmailService.send()` at runtime.

---

## vtable — How JVM Resolves Overriding

Every class has a **virtual method table (vtable)** — a table of method pointers.

```text
NotificationService vtable:
  send → NotificationService.send()

EmailService vtable:
  send → EmailService.send()      ← overrides parent pointer

SMSService vtable:
  send → SMSService.send()        ← overrides parent pointer
```

When `svc.send()` is called → JVM looks up vtable of **actual object** → finds correct method pointer → calls it.

---

## Real Engineering Value of Dynamic Dispatch

```java
List<NotificationService> channels = List.of(
    new EmailService(),
    new SMSService(),
    new PushNotificationService()
);

channels.forEach(c -> c.send("Order confirmed"));
```

Add `WhatsAppService` → add to list → **nothing else changes**.
No if-else chains. No instanceof. This is Open/Closed Principle emerging from polymorphism.

---

## Static Methods — NOT Polymorphic

Static methods are resolved at **compile time** based on **declared type** — not actual object type.
This is **method hiding**, not overriding.

```java
class Base  { public static void print() { System.out.println("Base"); } }
class Child extends Base { public static void print() { System.out.println("Child"); } }

Base obj = new Child();
obj.print();  // prints "Base" — compile-time resolution on declared type
```

`Child.print()` **hides** `Base.print()` — both exist independently.
No vtable involved. Declared type wins.

```java
// Instance method — runtime
Base obj = new Child();
obj.instanceMethod();  // resolves to Child.instanceMethod() — dynamic dispatch

// Static method — compile time
obj.staticMethod();    // resolves to Base.staticMethod() — static dispatch
```

**Rule:** Never call static methods on instance references. Call on class name directly.
```java
Base.print();   // clear intent
Child.print();  // clear intent
obj.print();    // misleading — avoid
```

---

## Overloading vs Overriding — Sharp Comparison

| | Overloading | Overriding |
|---|---|---|
| **Parameters** | Different | Same |
| **Resolution** | Compile time | Runtime |
| **Mechanism** | Static dispatch | Dynamic dispatch (vtable) |
| **Inheritance required** | No | Yes |
| **`@Override` annotation** | Invalid | Valid |
| **Return type** | Can differ | Must be same (or covariant) |
| **Polymorphic** | No | Yes |

---

## Method Hiding vs Method Overriding

| | Overriding | Hiding |
|---|---|---|
| **Applies to** | Instance methods | Static methods |
| **Resolution** | Runtime — vtable | Compile time — declared type |
| **Polymorphic** | Yes | No |
| **`@Override`** | Valid | Compiler warning |

---

## Failure Scenarios
- Calling static method on instance reference thinking it's polymorphic → wrong method called silently, no error
- Expecting overloading to dispatch dynamically → compile-time resolution surprises
- Overriding with `throw UnsupportedOperationException` → LSP violation, runtime crashes

---

## Common Mistakes
- Swapping overloading/overriding resolution times in interviews (overloading = compile, overriding = runtime)
- Assuming static methods are polymorphic
- Using `instanceof` chains instead of leveraging polymorphism
- Calling static methods via instance references

---

## Interview Perspective
- Resolution time is a very common interview question — overloading=compile, overriding=runtime
- vtable explanation separates strong candidates from average ones
- Static method hiding is a trap question — know what it prints and why
- Connect polymorphism to Open/Closed Principle — adding types without modifying existing code

---

## Revision Summary
- Overloading = same name, different params = compile-time = static dispatch
- Overriding = same name, same params, subclass = runtime = dynamic dispatch = vtable
- vtable: each class has method pointer table; JVM looks up actual object's vtable at runtime
- Static methods: compile-time resolution, method hiding not overriding, not polymorphic
- `Base obj = new Child(); obj.staticMethod()` → calls Base's method
- Dynamic dispatch enables Open/Closed — add types, don't modify callers

---

## Active Recall Questions
1. What is the difference between static and dynamic dispatch?
2. What is a vtable and how does JVM use it?
3. `Base obj = new Child(); obj.staticMethod()` — what prints and why?
4. Why are static methods not polymorphic?
5. What is method hiding vs method overriding?
6. How does polymorphism connect to Open/Closed Principle?

---

## Related Concepts
- [[Inheritance, LSP, Composition]]
- [[Interface vs Abstract Class]]
- [[SOLID Principles]]
- [[JVM Internals]]
- [[Design Patterns — Behavioral]]
