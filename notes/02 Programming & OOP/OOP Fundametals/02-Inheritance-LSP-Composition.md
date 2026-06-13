# Inheritance, LSP & Composition over Inheritance

## Tags
#oop #java #lld #solid

---

## Overview
- Inheritance models IS-A relationships — subclass extends parent contract
- LSP defines **when inheritance is valid** — child must fully substitute parent
- Composition models HAS-A relationships — delegate to collaborators
- Most inheritance misuse violates LSP → prefer composition

---

## Inheritance

### When It Is Valid
- True IS-A relationship exists
- Child can **fully honor** parent's contract everywhere
- Substituting child for parent produces no surprises

### When It Breaks — LSP Violation

**Liskov Substitution Principle:**
> If S is a subtype of T, objects of T should be replaceable with objects of S without breaking correctness.

#### Classic Example — Penguin
```java
class Bird {
    public void fly() { ... }
    public void eat() { ... }
}

class Penguin extends Bird {
    public void fly() {
        throw new UnsupportedOperationException(); // breaks contract
    }
}
```

Caller holds `Bird` reference and calls `fly()` — crashes at runtime. Child cannot substitute parent. **LSP violated.**

**Damage:** callers are forced to write defensive `instanceof` checks:
```java
for (Bird b : birds) {
    if (!(b instanceof Penguin)) b.fly(); // polymorphism destroyed
}
```
Every new flightless bird = new `instanceof` check. Codebase rots.

#### Real Java Example — Stack extends ArrayList
```java
class Stack extends ArrayList {
    public void push(Object o) { add(o); }
    public Object pop() { return remove(size()-1); }
}

// Caller can legally do:
stack.add(0, 99);    // inserts at index 0 — violates LIFO
stack.remove(1);     // removes from middle — breaks stack contract
```
`Stack` inherits full `ArrayList` API but cannot honor it consistently. LSP broken. This exists in Java's own history as a design mistake.

---

## Composition over Inheritance

### Core Idea
Instead of extending a class to reuse behavior — hold a reference to it.

```java
// Wrong — Stack IS-A ArrayList?
class Stack extends ArrayList { ... }

// Right — Stack HAS-A storage
class Stack {
    private ArrayList<Object> items = new ArrayList<>();

    public void push(Object o) { items.add(o); }
    public Object pop() { return items.remove(items.size()-1); }
    public boolean isEmpty() { return items.isEmpty(); }
    // ArrayList internals completely hidden
}
```

### Benefits of Composition
- Internal implementation swappable (ArrayList → LinkedList) without caller impact
- Only expose what the contract requires
- No accidental inheritance of irrelevant methods
- LSP naturally satisfied — you only expose what you promise

---

## Fixing LSP Violations — Restructure Hierarchy

```java
// Wrong — rigid hierarchy
class Bird { void fly(); void eat(); }
class Penguin extends Bird { ... } // can't fly

// Right — capability-based interfaces
interface Animal { void eat(); }
interface Flyable { void fly(); }

class Eagle implements Animal, Flyable { ... }
class Penguin implements Animal { ... }       // no fly() promise
class Seaplane implements Flyable { ... }     // not even an animal
```

- Classes depend on the **capabilities they need**, not a rigid type tree
- Adding new types never breaks existing callers
- Open/Closed naturally satisfied — extend by adding types, not modifying callers

---

## Decision Rule

| Question | Answer | Use |
|---|---|---|
| IS-A relationship? Child honors full parent contract? | Yes | Inheritance |
| Sharing behavior across unrelated types? | Yes | Interface + Composition |
| Need to reuse implementation without IS-A? | Yes | Composition |
| Child throws UnsupportedOperationException? | Yes | Stop — redesign |

---

## Tradeoffs

| | Inheritance | Composition |
|---|---|---|
| Code reuse | Direct — no delegation | Requires delegation boilerplate |
| Coupling | Tight — child knows parent internals | Loose — depend on interface |
| Flexibility | Rigid — locked at compile time | Flexible — swap at runtime |
| LSP risk | High | Low |
| Testability | Harder — parent state entangled | Easier — mock collaborators |

---

## Failure Scenarios
- Adding `fly()` to `Bird` and subclassing Penguin → LSP violation, runtime crashes
- `Stack extends ArrayList` → callers bypass LIFO via inherited methods
- Deep inheritance chains → fragile base class problem (changing parent breaks all children)
- Overriding with `throw new UnsupportedOperationException()` → caller can never trust contract

---

## Common Mistakes
- Using inheritance for code reuse alone (no IS-A relationship)
- Overriding methods to throw exceptions instead of fixing hierarchy
- Building deep inheritance chains (>2 levels is a warning sign)
- Forgetting that `instanceof` checks in loops signal broken polymorphism

---

## Interview Perspective
- "Favor composition over inheritance" — explain WHY with LSP
- Always ask: *can child substitute parent everywhere without surprises?*
- Stack/ArrayList is a famous real-world example — use it
- Penguin/Bird is the classic LSP violation — know it cold
- Connect to [[Open Closed Principle]] — good composition enables extension without modification

---

## Revision Summary
- LSP: child must fully substitute parent — no surprises
- `throw UnsupportedOperationException` in override = LSP violation
- Stack extends ArrayList = historical Java design mistake
- Fix LSP violations by restructuring hierarchy using interfaces + composition
- Composition: hold reference, delegate, expose only needed contract
- `instanceof` in loops = sign that polymorphism is broken

---

## Active Recall Questions
1. State LSP in one sentence. Give an example of violation.
2. Why does `Stack extends ArrayList` violate LSP?
3. What is the production consequence of LSP violation?
4. When is inheritance justified over composition?
5. How does fixing LSP violations connect to Open/Closed Principle?
6. Why do `instanceof` chains in loops signal a design problem?

---

## Related Concepts
- [[Encapsulation vs Abstraction]]
- [[SOLID Principles]]
- [[Interface vs Abstract Class]]
- [[Polymorphism — Static vs Dynamic Dispatch]]
- [[Open Closed Principle]]
- [[Design Patterns — Structural]]
