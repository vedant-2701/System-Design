# Question

> You're designing GET /products/123. The product has a name, description, price, and stock count. Price and name change rarely — maybe once a week. Stock count changes constantly — every purchase decrements it.
> Question: How would you design the caching strategy for this endpoint? Should you cache it at all? If yes, what Cache-Control headers, what TTL, and how do you handle the stock count problem specifically?

---

# Answer

Excellent. All three approaches are real production patterns. You've independently derived endpoint decomposition, edge-side includes, and conditional caching. That's strong reasoning.

Let me evaluate each precisely.

---

**Approach 1 — no-cache on the full response**

Correct that it validates on every request rather than serving stale. But you identified the right concern yourself — it's not efficient here.

The problem: `no-cache` still hits your server on every request for revalidation. At 50k req/s, you're making 50k revalidation checks per second even if 99% return 304. You've reduced bandwidth but not server load. For a product changing once a week, you're doing 604,800 revalidation round trips to catch one change. The cost-benefit is poor.

`no-cache` is better suited for resources that change unpredictably but frequently — like a user's notification count. For weekly-changing data, a longer TTL with purge-on-write is more efficient.

---

**Approach 2 — endpoint decomposition**

This is the correct production answer. Split stable and volatile data into separate endpoints with different caching strategies.

```
GET /products/123
Cache-Control: public, max-age=86400, s-maxage=604800
→ name, description, price (stable, CDN-cacheable)

GET /products/123/availability  
Cache-Control: public, max-age=10, stale-while-revalidate=5
→ stock_count, in_stock (volatile, short TTL)
```

Client makes two requests. Stable data comes from CDN edge — zero server load. Availability hits your servers but with a short TTL that's acceptable for stock count accuracy.

This is exactly what large e-commerce platforms do. Amazon separates product detail caching from inventory availability. The product page loads fast from CDN. Stock availability is a separate lightweight call with its own caching behavior.

**What you got slightly wrong** — you said `no-store` for the dynamic endpoint. That's too aggressive. `no-store` means don't cache anywhere, ever. Even 10 seconds of caching on stock count dramatically reduces load. If stock count is off by 10 seconds, that's usually acceptable — you handle overselling at the order confirmation layer anyway, not at the browse layer.

---

**Approach 3 — server-driven template with edge assembly**

You've described **Edge Side Includes (ESI)** — a real standard supported by Varnish, Fastly, and some CDNs. The CDN assembles the final response from cached fragments and dynamic server-fetched fragments at the edge.

```html
<!-- CDN assembles this -->
<esi:include src="/products/123/static"/>   <!-- cached 7 days -->
<esi:include src="/products/123/stock"/>    <!-- fetched fresh -->
```

This is powerful but operationally complex. You need a CDN that supports ESI, you need to design your backend to serve fragments, and debugging becomes harder because the response assembly happens at the edge, not in your application. Teams only reach for this when simpler approaches break down at extreme scale.

---

**The approach you didn't mention — purge on write:**

```
// When price updates
PATCH /products/123
→ Update database
→ Purge /products/123 from CDN immediately
→ Redis cache invalidated
→ Next request fetches fresh data, repopulates cache
```

With `s-maxage=604800` and purge-on-write, you get week-long CDN caching with immediate invalidation on change. This is cleaner than short TTLs for data that changes rarely but must be accurate when it does change.

The risk — purge can fail. Your write succeeded but purge failed. Now you have stale CDN cache with no TTL expiry for a week. Production systems handle this with purge retries, fallback TTLs, and alerts on purge failure rates.

---

**The complete production strategy for this endpoint:**

```
GET /products/123
→ name, description (truly stable — change requires content team action)
   Cache-Control: public, s-maxage=604800, max-age=86400
   ETag: hash of name+description
   Purge on content update

GET /products/123/price
→ price (changes occasionally, business-critical accuracy)
   Cache-Control: public, s-maxage=3600, max-age=60
   Purge on price update

GET /products/123/availability
→ stock_count, in_stock
   Cache-Control: public, max-age=10, stale-while-revalidate=5
   No purge needed — TTL is short enough
```

Three endpoints, three caching profiles, each tuned to its volatility and accuracy requirements.

---