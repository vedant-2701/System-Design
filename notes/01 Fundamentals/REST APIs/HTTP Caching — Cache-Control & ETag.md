# HTTP Caching — Cache-Control & ETag

## Tags
#rest #api-design #backend #caching #scaling #http

---

## Overview

- HTTP caching reduces origin server load by serving responses from intermediate stores (browser, CDN, proxy, Redis)
- Two mechanisms: **Cache-Control** (time-based directives) and **ETag** (content-based validation)
- Caching always trades **consistency** for **performance** — the fundamental tension
- Correct cache design requires understanding the full request path and each layer's behavior

---

## Cache Hierarchy

```
Client (Browser / Mobile App)
        ↓ miss
CDN Edge Cache (Cloudflare, Fastly, Akamai)
        ↓ miss
Server-Side Cache (Redis)
        ↓ miss
Database
```

Each layer absorbs requests before hitting the next. Goal: most requests resolved at the top layers.

**CDN critical constraint:** Only caches responses that are **identical across users**. User-specific responses (`/orders`, `/profile`) must use `private` directive — CDN must not cache them. Violating this leaks one user's data to another.

---

## Cache-Control Directives

### Visibility
- `public` — any cache may store this: browser, CDN, proxy. Use for shared resources (product pages, static assets).
- `private` — only the end client (browser) may cache. CDN must not store. Use for user-specific responses.

### Time-Based
- `max-age=N` — cache is fresh for N seconds. After expiry, client must revalidate.
- `s-maxage=N` — like `max-age` but applies only to shared caches (CDNs). Overrides `max-age` for CDNs.

**Combining for different TTLs per layer:**
```
Cache-Control: public, s-maxage=86400, max-age=60
```
CDN caches 24 hours (can be purged programmatically). Client caches 60 seconds (cannot be forced to invalidate).

### Revalidation
- `no-cache` — **does NOT mean "don't cache."** Cache the response, but always revalidate with the server before serving. Server may respond 304 (use cache) or 200 (fresh content). Saves bandwidth, doesn't save server load.
- `no-store` — **never store this response anywhere.** Use for sensitive data: payment details, auth tokens, PII.
- `must-revalidate` — once stale, must revalidate before serving. Do not serve stale content even if origin is unreachable.

### Stale Serving
- `stale-while-revalidate=N` — after `max-age` expires, serve stale content immediately for N more seconds while fetching fresh content in background. Eliminates latency spike at cache expiry.

```
Cache-Control: max-age=60, stale-while-revalidate=30
```
After 60s: serve stale immediately + kick off background revalidation. Next request gets fresh content. Zero user-visible latency on cache miss.

- `stale-if-error=N` — serve stale content if origin returns error. Graceful degradation when origin is down.

---

## ETag — Content-Based Validation

Solves: "I have a cached response, is it still valid?" without transferring the full body.

### Flow

**First request:**
```http
GET /products/123

200 OK
ETag: "abc123hash"
Cache-Control: max-age=60
{ "id": 123, "price": 499 ... }
```
Client stores response + ETag.

**After cache expires — conditional request:**
```http
GET /products/123
If-None-Match: "abc123hash"
```

**If content unchanged:**
```http
304 Not Modified
(no body — bandwidth saved)
```
Client uses cached response. Only headers transferred.

**If content changed:**
```http
200 OK
ETag: "xyz789hash"
{ "id": 123, "price": 599 ... }
```
Fresh response with new ETag.

### ETag Types

- **Strong ETag** `"abc123"` — byte-for-byte identical. Precise. Expensive to compute for large responses.
- **Weak ETag** `W/"abc123"` — semantically equivalent, not byte-identical. Used when gzip or minor formatting differences shouldn't invalidate.

### Last-Modified — Timestamp Alternative

```http
Last-Modified: Mon, 15 Jan 2024 10:30:00 GMT

If-Modified-Since: Mon, 15 Jan 2024 10:30:00 GMT
```

**ETag preferred over Last-Modified:** timestamps have 1-second granularity. Two changes within one second are invisible to Last-Modified. ETags catch every change regardless of timing.

---

## Cache Invalidation — The Hard Problem

"There are only two hard things in computer science: cache invalidation and naming things."

### TTL-Based (Accept Stale Window)
- Set `max-age` to acceptable staleness tolerance
- Simple. No coordination needed.
- Data may be stale for up to `max-age` seconds after a change
- Best for: data that changes predictably or where staleness is tolerable

### Purge on Write
- When data updates: write to DB → immediately purge cache entries at CDN and Redis
- Accurate: no stale window after successful purge
- Failure modes: purge fails silently → stale CDN cache persists for full TTL
- Requires: purge retry logic, fallback TTLs, alerts on purge failure rate

### Surrogate Keys / Cache Tags (Advanced)
- Tag cached responses with metadata: `Cache-Tag: product:123`
- On update: purge everything tagged `product:123` in one atomic operation
- Supported by Fastly, Cloudflare
- Powerful for complex invalidation (one product update invalidates product page, category page, search results)

---

## Endpoint Decomposition by Volatility

When one resource has fields with different change rates, split into separate endpoints with different cache profiles.

**Example: Product with stable content + volatile stock:**

```
GET /products/123
Cache-Control: public, s-maxage=604800, max-age=86400
→ name, description (changes weekly — CDN caches 7 days, purge on update)

GET /products/123/price  
Cache-Control: public, s-maxage=3600, max-age=60
→ price (changes occasionally — CDN caches 1 hour, purge on change)

GET /products/123/availability
Cache-Control: public, max-age=10, stale-while-revalidate=5
→ stock_count, in_stock (changes constantly — short TTL, no purge needed)
```

Client makes 2-3 lightweight requests. Stable data hits CDN edge. Volatile data hits origin with short TTL. Overselling handled at order confirmation, not browse layer.

This is how large e-commerce platforms (Amazon, etc.) separate product detail caching from inventory availability.

---

## Edge Side Includes (ESI)

Advanced pattern: CDN assembles response from cached fragments + dynamic server-fetched fragments at edge.

```xml
<esi:include src="/products/123/static"/>  <!-- cached 7 days at CDN -->
<esi:include src="/products/123/stock"/>   <!-- fetched fresh per request -->
```

Supported by Varnish, Fastly. Powerful but operationally complex — debugging assembly at edge is hard. Only justified at extreme scale when endpoint decomposition is insufficient.

---

## Caching Decision Matrix

| Response Type | Directive | Rationale |
|---|---|---|
| Public static content | `public, max-age=86400, s-maxage=604800` | Stable, shared, CDN-cacheable |
| User-specific response | `private, max-age=60` | Browser only, never CDN |
| Sensitive data | `no-store` | Never cache anywhere |
| Frequently validated | `no-cache` | Cache but always revalidate |
| Volatile shared data | `public, max-age=10, stale-while-revalidate=5` | Short TTL, serve stale while refreshing |
| Infrequent + accurate | `public, s-maxage=3600` + purge on write | Long CDN TTL, programmatic invalidation |

---

## Failure Scenarios

- **`private` on shared resource:** resource not cached at CDN, all requests hit origin
- **`public` on user-specific resource:** CDN serves user A's data to user B — data leak
- **`no-cache` confused with `no-store`:** developer thinks `no-cache` prevents caching — it doesn't, it forces revalidation
- **Purge failure silent:** write succeeds, purge fails, CDN serves stale data for full TTL with no alert
- **Timestamp-only cursor for ETag generation:** two updates within 1 second → ETag unchanged → client serves stale
- **No `stale-while-revalidate`:** cache expiry at high traffic = all requests simultaneously hit origin (thundering herd at TTL boundary)

---

## Scaling Considerations

- CDN absorption can eliminate 80-95% of read traffic for popular shared resources
- `stale-while-revalidate` prevents thundering herd at TTL expiry — critical at high QPS
- 304 responses are dramatically cheaper than 200 at scale — ETag conditional requests save bandwidth and processing
- Short TTL on volatile data + long TTL on stable data via endpoint decomposition is more efficient than moderate TTL on combined endpoint

---

## Real-World Usage

- **Stripe:** `Cache-Control: no-cache, no-store` on all API responses (financial data, must always be fresh)
- **GitHub:** ETags on repository and file endpoints
- **CDN for e-commerce:** product catalog cached at edge, inventory availability served from origin with short TTL
- **Social feeds:** `stale-while-revalidate` for timeline endpoints — serve slightly stale feed instantly, refresh in background

---

## Interview Perspective

- "What's the difference between `no-cache` and `no-store`?" — revalidation vs no storage
- "How would you cache an endpoint with volatile and stable fields?" — endpoint decomposition
- "What does 304 mean?" — Not Modified, use cached response (not a redirect — that's 302)
- "How does ETag work?" — conditional request with `If-None-Match`, server returns 304 or 200 + new ETag
- "How do you invalidate CDN cache immediately?" — purge API + handle purge failures
- Common mistake: confusing `no-cache` with `no-store`; returning 302 instead of 304

---

## Related Concepts

- [[REST APIs — Core Concepts]]
- [[Caching Strategies — Cache-Aside, Write-Through, Write-Behind]]
- [[CDN Architecture]]
- [[Redis as Cache]]
- [[Thundering Herd Problem]]
- [[REST API Error Response Design]]
- [[API Versioning Strategies]]

---

## Revision Summary

- Cache hierarchy: Client → CDN → Redis → DB. Each layer absorbs misses.
- `public` = any cache. `private` = client only. `no-cache` = revalidate always. `no-store` = never store.
- `s-maxage` overrides `max-age` for CDNs — enables different TTLs per layer
- `stale-while-revalidate` eliminates TTL expiry latency spikes
- ETag = content hash. Conditional request with `If-None-Match`. Server returns 304 (unchanged) or 200 (changed).
- ETag preferred over Last-Modified — 1-second timestamp granularity misses rapid changes
- Endpoint decomposition by volatility — separate cache profiles per field change rate
- Cache invalidation: TTL (simple, stale window), purge-on-write (accurate, failure-prone), cache tags (powerful, complex)

---

## Active Recall Questions

1. What is the difference between `no-cache` and `no-store`? Give a use case for each.
2. How does `stale-while-revalidate` prevent thundering herd at cache TTL expiry?
3. Walk through the full ETag flow for a conditional request where the content has NOT changed.
4. Why is ETag preferred over Last-Modified?
5. You have a product endpoint with name (weekly change), price (hourly change), and stock (per-second change). How do you design caching for this?
6. Purge-on-write fails silently. What are the consequences and how do you mitigate?
7. Why must user-specific responses use `private` and never `public`?
8. What status code does a successful conditional ETag revalidation return? (Not 302.)
