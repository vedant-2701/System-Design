# REST APIs — Master Index

## Tags
#rest #api-design #backend #index

---

## Domain Overview

REST APIs is a foundational backend engineering domain covering API design principles, contract management, data access patterns, caching, and observability. Every backend service exposes APIs — understanding how to design them correctly at scale is non-negotiable.

---

## Notes in This Domain

- [[REST APIs — Core Concepts]] — REST vs HTTP, Richardson Maturity Model, HTTP method semantics, idempotency, uniform interface, HATEOAS
- [[API Versioning Strategies]] — URL/header/query/media-type versioning, backward compatible evolution, sunset policy
- [[Pagination — Cursor vs Offset]] — offset correctness problems, cursor design, opaque tokens, composite keys
- [[REST API Error Response Design]] — error schema, error code taxonomy, retryable flag, 404 vs 403 security, no internal leakage
- [[HTTP Caching — Cache-Control & ETag]] — cache hierarchy, Cache-Control directives, ETag conditional requests, invalidation strategies, endpoint decomposition
- [[OpenAPI & Swagger]] — spec-first vs code-first, contract testing, SDK generation, mock servers

---

## Key Cross-Cutting Themes

**Idempotency runs through everything:**
- HTTP method semantics (GET/PUT idempotent, POST/PATCH not guaranteed)
- Error response design (retryable flag for consumer retry decisions)
- Pagination (cursor is position-stable under concurrent writes, offset is not)
- Caching (GET requests safe to cache because idempotent)

**The caching-consistency tension:**
- Every caching decision is a tradeoff between freshness and performance
- Endpoint decomposition by volatility resolves this without accepting a single global TTL
- Cache-Control directives express this intent to every layer in the hierarchy

**Contract stability:**
- API versioning, OpenAPI contract testing, and backward compatible evolution all serve the same goal: clients don't break unexpectedly
- Breaking changes are expensive — the more distributed your consumers, the more expensive

---

## Implementation Checklist (Pending)

- [ ] Idempotency key middleware (POST /orders with deduplication)
- [ ] Cursor-based pagination on a list endpoint
- [ ] Error response middleware with request_id propagation
- [ ] Cache-Control headers on product endpoints with endpoint decomposition
- [ ] OpenAPI spec for a sample service

---

## Related Domains

- [[gRPC & Protocol Buffers]] — REST alternative for internal services
- [[WebSockets]] — bidirectional, stateful alternative to REST for real-time
- [[Authentication & Authorization]] — JWT, OAuth, API key patterns
- [[Rate Limiting]] — token bucket, sliding window, distributed rate limiting
- [[Distributed Tracing]] — request_id / trace ID propagation across services
- [[DNS]] — how API traffic reaches your servers
- [[HTTP & HTTPS]] — the protocol REST is built on

---

## Quick Revision — Most Tested Interview Topics

1. REST vs HTTP — REST is constraints on top of HTTP, not the protocol itself
2. Idempotency — state not response code. DELETE is idempotent. PATCH not guaranteed.
3. Richardson Maturity Level 2 vs 3 — Level 2 is production standard. Level 3 (HATEOAS) rarely implemented.
4. URL versioning vs header versioning — cacheability vs REST correctness tradeoff
5. Offset vs cursor — offset breaks under writes, slow at depth. Cursor is correct and O(log n).
6. Error design — code (stable), retryable (retry signal), request_id (tracing), never expose internals
7. no-cache vs no-store — revalidate always vs never store
8. ETag → 304 Not Modified (not 302, which is a redirect)
9. 404 over 403 for unauthorized resource access — prevents resource enumeration
10. Spec-first OpenAPI + contract testing — prevents spec drift and breaking changes in production
