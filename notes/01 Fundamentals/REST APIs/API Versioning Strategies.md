# API Versioning Strategies

## Tags
#rest #api-design #backend #versioning

---

## Overview

- API versioning manages **breaking changes** without forcing all clients to update simultaneously
- Goal: evolve the API contract while maintaining backward compatibility for existing clients
- Two broad approaches: **explicit versioning** (version in URL or header) and **backward compatible evolution** (avoid versioning as long as possible)
- Every version created is a codepath maintained indefinitely until clients migrate — high long-term cost

---

## What Makes a Change Breaking

| Change | Breaking? | Why |
|---|---|---|
| Add new response field | No | Existing clients ignore unknown fields |
| Remove response field | Yes | Clients reading that field get undefined/error |
| Rename response field | Yes | Same as removal for existing clients |
| Add optional request param | No | Existing clients don't send it — no impact |
| Make optional param required | Yes | Existing clients not sending it now get 400 |
| Change field type (int → string) | Yes | Client type expectations violated |
| Add new endpoint | No | Existing clients unaffected |
| Change enum values | Yes | Client switch statements miss new values |
| Change auth model | Yes | All clients must update auth mechanism |
| Change response shape (restructure) | Yes | Field paths change entirely |

**Key insight:** Non-breaking changes are purely **additive**. Never remove, rename, retype, or tighten existing contracts.

---

## Versioning Strategies

### 1. URL Versioning
```
GET /api/v1/users/123
GET /api/v2/users/123
```
- Most widely used in practice (Stripe, GitHub, Twitter)
- Version is explicit and visible in URLs, logs, and shared links
- **Cache-friendly** — CDNs cache `/v1/` and `/v2/` independently, no special config needed
- Violates REST uniform resource principle — same resource has multiple URLs
- Code duplication grows as versions accumulate

### 2. Header Versioning
```
GET /api/users/123
Accept-Version: v2
Accept: application/vnd.myapi.v2+json
```
- Clean URLs, semantically more RESTful
- **Cache-hostile** — CDNs cache by URL; same URL with different headers should return different responses. Requires `Vary: Accept-Version`, poorly supported by many CDN/proxy layers
- Version invisible in URLs — lost when sharing links or filing bug reports
- Harder to debug — can't determine version from URL alone

### 3. Query Parameter Versioning
```
GET /api/users/123?version=2
```
- Cache-friendly (version in URL)
- Semantically weak — query params are for filtering/sorting, not contract selection
- Rarely used in serious production APIs

### 4. Content Negotiation (Media Type)
```
Accept: application/vnd.github.v3+json
```
- Most semantically correct — negotiating representation format is what `Accept` was designed for
- GitHub uses this
- Operationally complex, nearly invisible to developers
- No broad tooling support

---

## Comparison

| Strategy | Cache-Friendly | REST-Correct | Debuggability | Production Use |
|---|---|---|---|---|
| URL | Yes | No | High | Most common |
| Header | No | Yes | Low | Less common |
| Query Param | Yes | Weak | Medium | Rare |
| Media Type | No | Yes | Low | Rare (GitHub) |

---

## Backward Compatible Evolution — The Preferred Approach

Avoid versioning entirely by making only additive changes.

```json
// Instead of renaming user_name → display_name (breaking):
{
  "user_name": "vedant",      // kept — old clients read this
  "display_name": "vedant"    // added — new clients use this
}
```

Old clients continue working. New clients use new fields. No version bump.

**Where it breaks down — forced to version:**
- Semantic meaning of existing field changes (enum values expand, behavior changes)
- Authentication model changes (API key → OAuth)
- Resource model restructuring (embedded → separate resources)
- Business logic behind endpoint changes producing different results with same inputs

**Mature strategy:** evolve backward compatibly as long as possible. Version only when forced. Announce deprecation with `Sunset` headers. Enforce migration timeline.

---

## Code Management Behind Versions

Two approaches once versioning is necessary:

**Branch by abstraction** — separate v1/v2 handlers in same codebase. Clean routing, but duplication grows fast.

**Transformation layer** — one internal model, transform at API boundary per version. Cleaner long-term, adds complexity.

**Sunset policy** — version lifecycle must be explicit:
- Announce deprecation (add `Deprecated` and `Sunset` response headers)
- Set migration deadline
- Monitor v1 traffic to track client migration progress
- Delete v1 when traffic reaches zero (or enforce deadline)

Teams routinely underestimate the operational cost of maintaining multiple versions indefinitely.

---

## Failure Scenarios

- **No sunset policy:** v1 lives forever, maintenance burden accumulates indefinitely
- **Purge failure on breaking change:** clients not notified, discover breakage in production
- **Header versioning + CDN:** requests with different `Accept-Version` served same cached response — wrong version silently returned
- **Semantic change without version bump:** same field, different behavior — hardest to detect and debug

---

## Scaling Considerations

- URL versioning enables CDN caching per version without configuration — significant at scale
- Header versioning requires `Vary` header support across entire CDN/proxy chain — often breaks silently
- Multiple long-lived versions increase deployment and testing surface area

---

## Interview Perspective

- "How do you handle breaking changes?" — backward compatible evolution first, version only when forced
- "URL vs header versioning tradeoffs?" — cacheability vs REST correctness is the core tension
- "What makes a change breaking?" — must enumerate additive vs destructive changes precisely
- Common mistake: jumping to versioning before considering backward compatible evolution

---

## Related Concepts

- [[REST APIs — Core Concepts]]
- [[HTTP Caching — Cache-Control & ETag]]
- [[OpenAPI & Swagger]]
- [[REST API Error Response Design]]

---

## Revision Summary

- Breaking = removes, renames, retypes, or tightens existing contracts
- Non-breaking = purely additive
- URL versioning: cache-friendly, not REST-correct, most used in production
- Header versioning: REST-correct, cache-hostile, debugging pain
- Prefer backward compatible evolution; version only when structurally forced
- Every version = maintenance burden until clients fully migrate
- Sunset headers + migration timeline mandatory for version lifecycle

---

## Active Recall Questions

1. Why is URL versioning cache-friendly but header versioning is not?
2. Name three changes that are non-breaking and three that are breaking. Explain why for each.
3. What is backward compatible evolution and where does it break down?
4. Why does header versioning require `Vary` headers and why does that cause problems with CDNs?
5. What is a Sunset header and when should you use it?
6. A colleague says "let's just add a new version whenever we change anything." What's wrong with this?
