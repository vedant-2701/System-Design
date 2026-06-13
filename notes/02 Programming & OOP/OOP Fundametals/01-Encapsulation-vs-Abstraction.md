# Encapsulation vs Abstraction

## Tags
#oop #java #backend #lld

---

## Overview
- Both are OOP pillars but solve **different problems** — commonly conflated in interviews
- **Encapsulation** — protect and control access to internal state
- **Abstraction** — hide implementation complexity, expose only capability
- Encapsulation answers: *who can touch this data?*
- Abstraction answers: *what can this thing do?*

---

## Encapsulation

### Core Idea
Bundle state + behavior into one unit and **restrict direct state access** through access modifiers.

### What It Protects
- Internal state from invalid mutations
- Business invariants from being bypassed
- Implementation from leaking to callers

### Correct Example
```java
public class BankAccount {
    private double balance;
    private List<Transaction> ledger;

    public void deposit(double amount) {
        if (amount <= 0) throw new IllegalArgumentException();
        balance += amount;
        ledger.add(new Transaction("DEPOSIT", amount));
    }

    public void withdraw(double amount) {
        if (amount > balance) throw new InsufficientFundsException();
        balance -= amount;
        ledger.add(new Transaction("WITHDRAWAL", amount));
    }

    public double getBalance() { return balance; }
    // NO setBalance()
}
```

### Common Mistake — Anemic Encapsulation
```java
// Field is private but protection is meaningless
private double balance;
public void setBalance(double b) { this.balance = b; }  // anyone can corrupt state
```
Private field + public setter = **no real encapsulation**. Business logic leaks to callers. Object becomes a dumb data bag.

### Tell, Don't Ask Principle
- Wrong: ask for state → check it externally → mutate it
- Right: tell the object what to do → let it enforce its own rules

---

## Abstraction

### Core Idea
Hide *how* something works. Expose only *what* it can do via interfaces or abstract types.

### What It Enables
- Caller is decoupled from implementation
- Swap implementations without changing callers
- Program to contracts, not concrete types

### Example
```java
public interface PaymentService {
    void deposit(double amount);
    void withdraw(double amount);
}

// Caller never knows if this is BankAccount, CryptoWallet, etc.
PaymentService service = new BankAccount();
```

---

## Key Distinction

| | Encapsulation | Abstraction |
|---|---|---|
| **About** | Protecting internal state | Hiding implementation |
| **Mechanism** | `private` + controlled methods | Interfaces, abstract classes |
| **Question** | Who can touch this data? | What can this do? |
| **Enforced by** | Access modifiers | Type system, contracts |
| **Benefit** | Invariant enforcement | Decoupling, replaceability |

---

## Engineering Consequence
- Encapsulation only → protected but **tightly coupled** (caller knows concrete type)
- Abstraction only → decoupled but **state unprotected**
- Both together → protected state + decoupled callers = production-quality design

---

## Common Mistakes
- Using getter/setter for every field (anemic encapsulation)
- Conflating encapsulation with abstraction in interviews
- Forgetting `setBalance()` is a design smell even with validation
- Thinking `private` field alone = encapsulation

---

## Interview Perspective
- Always distinguish the two clearly — most candidates conflate them
- Use BankAccount example: `private balance` = encapsulation, `PaymentService` interface = abstraction
- Interviewers test: *"if you add a setter, is encapsulation preserved?"* — answer is no
- Connect to [[Tell Don't Ask]] and [[Dependency Inversion Principle]]

---

## Revision Summary
- Encapsulation = protect state via access control + domain operations
- Abstraction = hide implementation via interfaces/abstract types
- Setter on private field = anemic encapsulation, not real protection
- Tell don't ask: object enforces its own invariants
- Both are needed — encapsulation without abstraction = tightly coupled protection

---

## Active Recall Questions
1. What is the difference between encapsulation and abstraction?
2. Why is `private balance + setBalance()` not true encapsulation?
3. What principle does `tell, don't ask` relate to?
4. How does abstraction enable swapping implementations?
5. Can you have encapsulation without abstraction? What breaks?

---

## Related Concepts
- [[SOLID Principles]]
- [[Dependency Inversion Principle]]
- [[Interface vs Abstract Class]]
- [[Tell Don't Ask]]
- [[Anemic Domain Model]]
