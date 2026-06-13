# lld_framework.md

## Purpose

Define a consistent framework for approaching:
- Low-Level Design (LLD)
- Object-Oriented Design
- Production-Oriented Backend Design
- Machine Coding Rounds
- Design Pattern Problems
- Maintainable Software Architecture

The goal is NOT:
- memorizing design patterns
- writing excessive abstractions
- creating academic UML diagrams only

The goal is:
- clean system modeling
- maintainable architecture
- extensible design
- production-quality thinking
- strong engineering reasoning

---

# Core LLD Philosophy

LLD should optimize for:

1. Maintainability
2. Extensibility
3. Readability
4. Correctness
5. Testability
6. Simplicity
7. Scalability (when relevant)

Avoid:
- overengineering
- unnecessary abstractions
- pattern obsession
- tightly coupled code
- giant god classes

Every abstraction must solve a real problem.

---

# Standard LLD Flow

Always approach LLD problems in this order whenever applicable.

---

# Step 1 — Clarify Requirements

Understand:
- system features
- expected behaviors
- constraints
- edge cases
- scale expectations
- concurrency requirements

Clarify:
- core operations
- user interactions
- business rules
- extensibility expectations

---

# Important Rule

Do NOT jump directly into classes.

First understand:
- the problem
- workflows
- constraints
- responsibilities

---

# Step 2 — Identify Core Entities

Identify:
- domain objects
- business entities
- actors
- responsibilities

Examples:
- User
- Vehicle
- Payment
- Booking
- Ticket
- Notification

Focus on:
- behavior
- responsibilities
- relationships

NOT just data containers.

---

# Step 3 — Define Relationships

Identify:
- composition
- aggregation
- inheritance
- associations
- dependencies

Prefer:
- composition over inheritance

Use inheritance only when:
- true behavioral hierarchy exists

---

# Step 4 — Define Responsibilities

Each class should have:
- clear responsibility
- focused behavior
- limited scope

Follow:

Single Responsibility Principle

Avoid:

* god objects
* utility dumping
* mixed responsibilities

---

# Step 5 — Define Interfaces

Use interfaces when:

* multiple implementations may exist
* extensibility matters
* testing benefits
* dependency inversion helps

Avoid:

* creating interfaces prematurely
* unnecessary abstraction layers

Interfaces should solve real maintainability problems.

---

# Step 6 — Identify Design Patterns

Apply patterns ONLY when justified.

Common useful patterns:

* Strategy
* Factory
* Observer
* Singleton (carefully)
* Decorator
* Adapter
* Command
* Builder

Always explain:

* WHY the pattern helps
* WHAT problem it solves
* WHAT tradeoff it introduces

Avoid pattern-for-pattern’s-sake design.

---

# Step 7 — API / Method Design

Methods should:

* have clear responsibilities
* use meaningful names
* avoid excessive parameters
* expose clean contracts

Discuss:

* validation
* error handling
* concurrency implications
* side effects

---

# Step 8 — Data Flow

Explain:

* object interactions
* request lifecycle
* state transitions
* event flow
* dependency flow

Use:

* sequence-style explanations
* simple diagrams when useful

---

# Step 9 — Concurrency Considerations

Whenever applicable discuss:

* thread safety
* shared state
* synchronization
* locking
* race conditions
* deadlocks
* async execution

Especially important for:

* schedulers
* queues
* booking systems
* caches
* rate limiters

Concurrency should never be ignored.

---

# Step 10 — Extensibility Design

Discuss:

* how new features can be added
* plugin capability
* configuration flexibility
* future modifications

Strong LLD should tolerate change gracefully.

---

# Step 11 — Error Handling

Design for:

* invalid input
* failure scenarios
* retries
* partial failures
* defensive programming

Avoid:

* silent failures
* fragile APIs

---

# Step 12 — Persistence Considerations

When relevant discuss:

* repositories
* database interaction
* transactions
* consistency
* caching
* state management

Separate:

* domain logic
* persistence logic

---

# Step 13 — Testability

Good LLD should be easy to:

* unit test
* integration test
* mock
* debug

Encourage:

* dependency injection
* interface-driven design
* modular components

---

# Step 14 — Scalability Awareness

Even in LLD discuss:

* memory growth
* object lifecycle
* performance bottlenecks
* concurrency bottlenecks
* resource usage

Avoid unrealistic toy-only thinking.

---

# Step 15 — Refactoring Awareness

Discuss:

* simplification opportunities
* abstraction quality
* maintainability improvements
* coupling reduction

Avoid:

* premature abstraction
* unnecessary complexity

---

# UML Guidance

Use UML only when useful for clarity.

Useful diagrams:

* class diagrams
* sequence diagrams
* component diagrams

Avoid:

* overcomplicated UML
* academic-only modeling

The goal is clarity.

---

# Communication Rules

While explaining LLD:

* explain reasoning incrementally
* justify abstractions
* explain tradeoffs
* explain why alternatives were rejected

Avoid:

* dumping classes immediately
* generating giant codebases prematurely
* introducing patterns without justification

---

# Tradeoff-Centric Thinking

For every abstraction discuss:

* maintainability benefits
* extensibility benefits
* complexity introduced
* testing implications
* debugging implications

Examples:

* interfaces improve extensibility but increase abstraction complexity
* factories improve decoupling but may reduce simplicity

---

# Failure-Oriented Thinking

Regularly evaluate:

* invalid states
* race conditions
* stale state
* retry issues
* synchronization bugs
* inconsistent state transitions

LLD should tolerate incorrect usage safely.

---

# Common LLD Mistakes

Avoid:

* giant god classes
* excessive inheritance
* unnecessary patterns
* tight coupling
* poor naming
* mixed responsibilities
* ignoring concurrency
* weak encapsulation
* exposing internal state carelessly

---

# Real-World Engineering Focus

Prioritize:

* maintainability
* debuggability
* extensibility
* production readability
* operational simplicity

Avoid:

* academic perfectionism
* unnecessary abstraction depth

---

# Machine Coding Expectations

During machine coding rounds:

* prioritize correctness first
* keep architecture clean
* avoid overengineering
* communicate assumptions clearly
* implement incrementally

Prefer:
working clean solutions over overdesigned incomplete systems.

---

# LLD Success Criteria

A strong LLD solution should:

* model the domain clearly
* separate responsibilities properly
* remain extensible
* remain testable
* handle edge cases
* avoid unnecessary complexity
* communicate intent clearly

---

# Final Objective

The goal is to develop the ability to:

* design maintainable software systems
* model real-world domains cleanly
* apply design patterns correctly
* reason about abstractions properly
* write production-quality object-oriented systems
* perform strongly in LLD and machine coding interviews