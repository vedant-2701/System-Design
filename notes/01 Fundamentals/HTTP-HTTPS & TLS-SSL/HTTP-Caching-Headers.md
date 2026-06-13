# HTTP Caching Headers

## Tags
#http #caching #performance #backend #cdn

---

## Overview

- HTTP defines a caching model allowing clients, proxies, and CDNs to store and reuse responses
- Two strategies: **time-based** (don't ask until stale) and **validation-based** (ask if still fresh)
- Correct caching headers are one of the highest-leverage performance optimizations — zero server load for cached hits

---

## Two Caching Strategies

### Strategy 1 — Time-Based (Cache-Control)
Server declares how long the response is fresh. Client serves from cache with **no network request** during freshness window.

### Strategy 2 — Validation-Based (ETags / Last-Modified)
Client asks server "has this changed?" Server returns 304 No Modified (no body) or 200 with new content.

Both strategies are complementary and used together in production.

---

## Cache-Control Directives

```
Cache-Control: public, max-age=86400, stale-while-revalidate=3600, stale-if-error=86400
```

| Directive | Meaning |
|---|---|
| `public` | CDNs and proxies may cache |
| `private` | Only browser may cache (not CDN) |
| `max-age=N` | Fresh for N seconds |
| `no-cache` | Cache it but always revalidate before serving |
| `no-store` | Never write to any cache (sensitive data) |
| `must-revalidate` | Once stale, must revalidate — don't serve stale |
| `stale-while-revalidate=N` | Serve stale while fetching fresh in background |
| `stale-if-error=N` | Serve stale for N seconds if origin is down |
| `immutable` | Resource will never change — skip revalidation entirely |

**`no-cache` vs `no-store`**: `no-cache` does NOT mean "don't cache" — it means cache but always revalidate. `no-store` means never cache at all.

**`stale-while-revalidate`** is critical for UX — user sees instant response while background fetch updates the cache. Eliminates latency spike at cache expiry.

**`stale-if-error`** is critical for resilience — serve stale content during origin outages instead of showing error pages.

---

## Validation Headers

### ETags (Content Fingerprint)

Server response:
```
ETag: "art456-hash123"
```
Client conditional request:
```
If-None-Match: "art456-hash123"
```
Server response if unchanged:
```
HTTP/1.1 304 Not Modified
```

**Strong ETag** `"hash123"` — byte-for-byte identical content required
**Weak ETag** `W/"hash123"` — semantically equivalent but not byte-identical (e.g. compressed vs uncompressed of same content)

Use strong ETags for content validation. Weak ETags for compression variants.

### Last-Modified (Timestamp Fallback)

```
Last-Modified: Wed, 10 Jun 2026 08:00:00 GMT
If-Modified-Since: Wed, 10 Jun 2026 08:00:00 GMT
```

- 1-second granularity — misses sub-second changes
- ETags are more precise; Last-Modified is a fallback for clients that don't support ETags
- Serve both headers — costs nothing, maximizes compatibility

---

## Cache Hierarchy in Production

```
Browser Cache  (private)
      ↓
CDN Edge Cache  (public)
      ↓
API Gateway Cache
      ↓
Application Cache (Redis)
      ↓
Database
```

- `Cache-Control: public` → CDN caches; thousands of users served from edge, origin never sees request
- `Cache-Control: private` → CDN passes through; only user's browser caches
- Per-user data must be `private` — never `public`

---

## Cache Invalidation Patterns

### Purge on Write
- On content update, call CDN purge API
- CDN broadcasts invalidation to all edge nodes
- **Problem**: propagation is not instant — brief inconsistency window
- Use for: product catalogs, articles, content data

### Short max-age + Revalidation
- `max-age=60` instead of `max-age=86400`
- Staleness bounded by design — no purge needed
- Tradeoff: more origin requests

### Surrogate Keys / Cache Tags
```
Cache-Tag: article-456, author-789, category-tech
```
- Tag responses with logical identifiers
- On update, purge by tag — invalidates all URLs sharing that tag
- Supported by Fastly, Cloudflare
- Powerful for cascading invalidation (delete article → purge article + its comments)

### Cache Warming (Proactive Priming)
- After purge, immediately fetch the resource to warm CDN
- Prevents first real user hitting origin after invalidation

---

## Production Caching Design by Endpoint Type

### Public Static Content (article, author bio)
```
Cache-Control: public, max-age=86400, stale-while-revalidate=3600, stale-if-error=86400
ETag: "hash123"
Last-Modified: Wed, 10 Jun 2026 08:00:00 GMT
Vary: Accept-Encoding
Cache-Tag: article-456
```

### Frequently Changing Public Content (comments)
```
Cache-Control: public, max-age=30, stale-while-revalidate=10, stale-if-error=3600
ETag: "hash456"
Last-Modified: Wed, 10 Jun 2026 08:00:00 GMT
Vary: Accept-Encoding
Cache-Tag: comments-456, article-456
```

### Authenticated Personal Data
```
Cache-Control: private, no-store
```
Never use `public` or any `max-age` on authenticated user-specific data.

### Truly Immutable Assets (versioned JS/CSS)
```
Cache-Control: public, max-age=31536000, immutable
```
Safe only when filename changes on every deploy (e.g. `main.abc123.js`). ID-based content URLs are never truly immutable.

---

## Vary Header

```
Vary: Accept-Encoding
```

Tells CDN: cache separate copies per value of `Accept-Encoding`. Without this, a CDN might serve a gzip-compressed response to a client expecting uncompressed content.

---

## Failure Scenarios

- **Missing `stale-if-error`** — origin outage causes error pages instead of stale-but-valid content
- **`public` on authenticated endpoint** — CDN caches User A's profile; User B receives it → data breach
- **1-year `max-age` without purge** — content update invisible to users for a year if purge fails
- **Wildcard `Cache-Control: no-cache`** everywhere — defeats all caching; origin overloaded
- **Missing `Vary: Accept-Encoding`** — compressed responses served to clients expecting plain text
- **CDN propagation delay on purge** — brief window where stale content is served after update

---

## Common Mistakes

- Using `no-cache` thinking it means "don't cache"
- Setting `max-age=31536000` on content identified by ID (not immutable)
- Omitting `Last-Modified` alongside ETags (free fallback)
- Setting `public` on user-specific endpoints
- Not using `Cache-Tag` — forces URL-level purge instead of logical purge
- Missing `stale-if-error` — poor resilience during origin outages

---

## Interview Perspective

- Know both strategies: time-based and validation-based
- Explain 304 Not Modified and what body it contains (none)
- Distinguish `no-cache` from `no-store` — interviewers test this
- Cache invalidation is notoriously hard — be able to explain purge propagation delay
- CDN + Cache-Tag is the production answer for content platforms

---

## Revision Summary

- Two strategies: `Cache-Control: max-age` (time-based) and ETags/Last-Modified (validation)
- `public` = CDN can cache; `private` = browser only; `no-store` = never cache
- `no-cache` ≠ don't cache — it means always revalidate
- `stale-while-revalidate` = serve stale instantly, refresh in background
- `stale-if-error` = serve stale during origin outages — critical for resilience
- Strong ETags = byte-identical; Weak ETags = semantically equivalent
- `Vary: Accept-Encoding` required for compression-aware caching
- CDN purge is eventual — brief inconsistency window always exists
- Authenticated endpoints: always `private, no-store`
- Immutable = only safe for content-addressed filenames

---

## Active Recall Questions

1. What is the difference between `no-cache` and `no-store`?
2. What does a 304 response contain? When is it returned?
3. Why is `Cache-Control: public` dangerous on `/users/me`?
4. What is `stale-while-revalidate` and what UX problem does it solve?
5. Why can't you safely use `max-age=31536000` on `/articles/456`?
6. A product price updates at 2pm. CDN has it cached for 24 hours. What are your options?
7. What is the difference between a strong and weak ETag?
8. What does `Vary: Accept-Encoding` tell the CDN?

---

## Related Concepts

- [[HTTP Status Codes]]
- [[HTTPS Request Lifecycle]]
- [[CDN Architecture]]
- [[HTTP Version Evolution]]
- [[Redis Caching]]
