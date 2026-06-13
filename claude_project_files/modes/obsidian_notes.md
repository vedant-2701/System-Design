# obsidian_notes.md

## Purpose

Generate clean, structured, Obsidian-friendly notes optimized for:
- long-term retention
- fast revision
- conceptual clarity
- engineering reasoning
- interview preparation
- internal linking

These notes are NOT transcripts of conversations.

The goal is:
- concise depth
- high signal density
- easy retrieval
- scalable knowledge organization

---

# Core Principles

Notes must be:
- concept-focused
- concise but deep
- technically accurate
- highly scannable
- linked to related concepts
- optimized for revision
- optimized for future retrieval

Avoid:
- excessive prose
- conversational filler
- motivational text
- repeated explanations
- unnecessary storytelling

---

# One Concept = One Note

Each note should focus on ONE primary concept.

Good:
- Redis Persistence
- Consistent Hashing
- Circuit Breaker
- Kafka Consumer Groups
- Token Bucket Rate Limiter

Bad:
- Entire Distributed Systems
- Everything About Redis
- Complete HLD Notes

---

# Markdown Formatting Rules

Use:
- proper markdown headings
- bullet points
- tables when useful
- code blocks
- concise sections

Avoid:
- giant paragraphs
- deeply nested formatting
- excessive emojis
- decorative markdown

---

# Required Note Structure

Use this structure whenever applicable.

---

# Title

```md
# Topic Name
```

---

# Metadata Section

```md
## Tags
#distributed-systems #database #hld
```

Only include relevant minimal tags.

Avoid tag explosion.

---

# Concept Overview

```md
## Overview
```

Include:

* concise definition
* high-level purpose
* core idea

Maximum:
3–6 concise bullet points.

---

# Why It Exists

```md
## Why It Exists
```

Explain:

* what problem it solves
* why previous/simple approaches fail
* what engineering need created it

Focus on engineering motivation.

---

# Internal Working

```md
## Internal Working
```

Explain:

* architecture
* flow
* mechanisms
* algorithms
* execution model

Prefer:

* step-by-step explanations
* flows
* ordered lists

---

# Architecture / Flow

```md
## Architecture Flow
```

Use:

* ASCII diagrams
* flow diagrams
* component interaction

Example:

```text
Client
  ↓
Load Balancer
  ↓
API Service
  ↓
Redis Cache
  ↓
Database
```

Keep diagrams readable and compact.

---

# Key Components

```md
## Key Components
```

List major components/subsystems with short explanations.

---

# Tradeoffs

```md
## Tradeoffs
```

ALWAYS include:

* benefits
* drawbacks
* scaling limitations
* operational complexity
* consistency implications
* cost implications if relevant

Use comparison tables when appropriate.

---

# Scaling Considerations

```md
## Scaling Considerations
```

Discuss:

* bottlenecks
* scaling strategies
* throughput implications
* concurrency concerns
* replication/sharding considerations
* caching implications

---

# Failure Scenarios

```md
## Failure Scenarios
```

Discuss:

* what breaks
* failure modes
* operational risks
* mitigation strategies
* recovery behavior

This section is VERY important for distributed systems.

---

# Real-World Usage

```md
## Real-World Usage
```

Include:

* companies/systems using similar ideas
* production use cases
* engineering relevance

Avoid fake or speculative examples.

---

# Common Mistakes

```md
## Common Mistakes
```

Include:

* beginner misconceptions
* interview mistakes
* implementation pitfalls
* scalability mistakes

---

# Interview Perspective

```md
## Interview Perspective
```

Include:

* common interview questions
* expected tradeoff discussions
* commonly tested areas
* mistakes candidates make

---

# Implementation Notes

```md
## Implementation Notes
```

Include:

* practical engineering considerations
* coding concerns
* testing concerns
* concurrency concerns
* debugging considerations

Include concise code snippets only when valuable.

---

# Related Concepts

```md
## Related Concepts
```

Use Obsidian links heavily.

Example:

```md
- [[Redis Replication]]
- [[Distributed Caching]]
- [[Cache Invalidation]]
- [[CAP Theorem]]
```

Internal linking is VERY important.

---

# Revision Summary

```md
## Revision Summary
```

Include:

* 5–10 concise revision bullets
* high-signal recall points only

This section should optimize rapid revision.

---

# Active Recall Questions

```md
## Active Recall Questions
```

Generate:

* conceptual questions
* tradeoff questions
* scaling questions
* debugging questions

Example:

```md
1. Why is AOF slower than RDB?
2. What happens if Redis crashes during fsync?
3. Why can cache invalidation become difficult at scale?
```

---

# Code Formatting Rules

When including code:

* keep snippets concise
* focus on concepts
* avoid giant implementations
* prefer readability over completeness

Use proper markdown code blocks.

Example:

```python
cache[key] = value
```

---

# Comparison Formatting Rules

When comparing concepts:
prefer markdown tables.

Example:

| Feature         | Redis | Memcached |
| --------------- | ----- | --------- |
| Persistence     | Yes   | No        |
| Data Structures | Many  | Limited   |

---

# Linking Rules

Always add relevant internal links.

Prefer:

```md
[[Concept Name]]
```

Over:
plain text references.

This improves long-term knowledge graph quality.

---

# Compression Rules

The note should:

* preserve important insights
* preserve tradeoffs
* preserve architecture reasoning

But should NOT:

* become a transcript
* include unnecessary repetition
* include every conversational detail

Compress intelligently without losing engineering meaning.

---

# Accuracy Rules

Before generating notes:

* verify technical correctness
* verify tradeoffs
* verify architecture logic
* verify terminology
* avoid speculative claims

Do not oversimplify critical distributed systems concepts.

---

# Output Rules

Generate ONLY markdown.

Do NOT:

* add conversational intros
* add explanations outside notes
* add motivational content

The output should be directly usable inside Obsidian without cleanup.