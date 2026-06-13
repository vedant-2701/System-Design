# Pagination — Cursor vs Offset

## Tags
#rest #api-design #backend #database #scaling

---

## Overview

- Pagination controls how large datasets are returned in manageable chunks
- Two primary strategies: **offset-based** (page numbers) and **cursor-based** (positional token)
- Choice affects correctness, performance, and UI design — not just implementation complexity
- Decision starts at product design, not API design

---

## Offset-Based Pagination

```sql
SELECT * FROM orders ORDER BY created_at DESC LIMIT 50 OFFSET 490000
```

```
GET /orders?page=9800&limit=50
GET /orders?offset=490000&limit=50
```

**How it works:** skip N records, return next M. Database traverses and discards the first N rows.

**Problems:**

**1. Performance degrades at depth (Late Pagination Problem)**
- `OFFSET 490000` forces DB to scan and discard 490,000 rows even with an index
- Query time grows linearly with offset value
- At extreme depths approaches full table scan behavior

**2. Correctness breaks under concurrent writes**
- New record inserted at top while user reads page 3 → every subsequent page shifts by one → **user skips a record permanently**
- Record deleted while paginating → **duplicate record appears across two pages**
- Offset pagination produces incorrect results in any system with concurrent writes — by design

**When to use:**
- Total record count is small (< 10,000)
- Random access / page jumping is a product requirement
- Data is immutable or writes are extremely infrequent
- UI explicitly shows page numbers

---

## Cursor-Based Pagination

```sql
SELECT * FROM orders 
WHERE (created_at, id) < ('2024-01-15T10:30:00', 456)
ORDER BY created_at DESC 
LIMIT 50
```

```
GET /orders?cursor=eyJpZCI6NDU2fQ==&limit=50
```

**How it works:** encode the position of the last seen record as a cursor. Next query seeks directly to that position using an index.

**Performance:** O(log n) index seek regardless of depth. No scanning, no discarding.

**Correctness:** position-stable under writes. New records inserted don't shift existing cursors.

---

## Cursor Design — What Matters

**ID as cursor** — works only if IDs are monotonically increasing integers.
```
WHERE id > 123456
```

**Timestamp alone — dangerous.** Multiple records can share the same timestamp. Records at the exact boundary timestamp may be skipped or duplicated.

**Composite cursor (timestamp + ID) — correct approach:**
```sql
WHERE (created_at, id) < ('2024-01-15T10:30:00', 456)
```
Timestamp for sort order, ID for tie-breaking. Uniquely identifies position.

**Opaque cursor — production standard (Stripe, GitHub):**
- Base64-encode internal cursor state
- Client treats it as an opaque token, never parses it
- Server free to change cursor internals without breaking clients

```json
{
  "data": [...],
  "next_cursor": "eyJpZCI6NDU2LCJ0cyI6MTcwNTI5MH0=",
  "has_more": true
}
```

---

## Tradeoffs

| Dimension | Offset | Cursor |
|---|---|---|
| Performance at depth | Degrades (O(n) scan) | Stable (O(log n) seek) |
| Correctness under writes | Incorrect (skips/duplicates) | Correct (position-stable) |
| Random access / page jump | Supported | Not supported |
| Total count | Easy (`SELECT COUNT(*)`) | Expensive / approximate |
| Bidirectional navigation | Easy | Complex (prev_cursor needed) |
| UI pattern | Page numbers | Infinite scroll |
| Implementation complexity | Low | Medium |

---

## Cursor Limitations

- **No random access.** Cannot jump to record 5000 directly. Sequential forward navigation only.
- **No total count easily.** `COUNT(*)` on millions of rows is expensive. Cursor APIs typically omit or approximate total counts — which is why infinite scroll UIs pair naturally with cursors.
- **Cursor expiry.** Cursors encoding a timestamp/ID can become invalid if underlying data is reorganized or deleted. Clients need to handle `cursor_expired` errors.
- **Bidirectional complexity.** Backward pagination requires inverted queries and both `next_cursor` and `prev_cursor` in responses. Most APIs support forward only.

---

## Hybrid — Paginated Chapters + Cursor Scroll

Real pattern used in some social feed products. Coarse page boundaries (chapters) with cursor-based scrolling within each chapter. High engineering complexity — maintain two levels of pagination state on client. Only justified when product explicitly requires both behaviors.

---

## Architecture Flow

```
Client requests page 1
  → GET /orders?limit=50
  → DB: SELECT * FROM orders ORDER BY (created_at, id) DESC LIMIT 50
  → Response: { data: [...50 items], next_cursor: "abc123", has_more: true }

Client requests next page
  → GET /orders?cursor=abc123&limit=50
  → Server decodes cursor: { created_at: "2024-01-15", id: 456 }
  → DB: SELECT * FROM orders WHERE (created_at, id) < ('2024-01-15', 456)
        ORDER BY (created_at, id) DESC LIMIT 50
  → Direct index seek — no scanning
```

---

## Failure Scenarios

- **Timestamp-only cursor on high-write system:** multiple records same timestamp at boundary → records silently skipped
- **Cursor without expiry handling:** cursor references deleted record → undefined behavior
- **Using offset on 10M record table:** page 200,000 at 50/page → offset 10,000,000 → query timeout in production
- **Mutable cursor format (not opaque):** clients parse cursor internals → server cannot change cursor implementation without breaking clients

---

## Scaling Considerations

- Offset pagination degrades exactly when you need it most — when the dataset is large
- Cursor pagination performance is independent of dataset size — scales horizontally
- Composite index on `(created_at, id)` is mandatory for cursor queries to be efficient
- `COUNT(*)` for total pages is a separate expensive query — avoid at scale; use approximate counts or remove from API entirely

---

## Real-World Usage

- **Stripe:** opaque base64 cursors, forward pagination only
- **GitHub:** cursor-based for most list endpoints
- **Twitter/X:** timeline uses cursor (since_id, max_id) for position stability under high write volume
- **Facebook Graph API:** cursor-based pagination throughout

---

## Interview Perspective

- "Why is offset pagination incorrect?" — concurrent write correctness, not just performance
- "When would you choose offset over cursor?" — random access, small datasets, page number UI requirement
- "Why is timestamp alone insufficient as a cursor?" — same-timestamp collision at boundary
- "How does Stripe implement cursors?" — opaque base64 tokens
- Common mistake: choosing offset because "it's simpler" without considering correctness implications

---

## Related Concepts

- [[REST APIs — Core Concepts]]
- [[Database Indexing]]
- [[SQL Query Optimization]]
- [[API Versioning Strategies]]

---

## Revision Summary

- Offset: simple, breaks correctness under writes, degrades at depth
- Cursor: correct, O(log n) always, no random access, no total count
- Timestamp alone as cursor is dangerous — use composite (timestamp + ID)
- Production cursors are opaque base64 tokens
- Choice starts at product design: infinite scroll → cursor, page jump → offset
- Cursor expiry must be handled explicitly

---

## Active Recall Questions

1. Why does offset pagination produce incorrect results on a system with concurrent writes?
2. Why is a timestamp-only cursor dangerous? What's the correct cursor design?
3. A client wants to jump to the 500th record. Can cursor pagination support this? What's the solution?
4. Why do cursor-based APIs typically not return total record counts?
5. What is the Late Pagination Problem and why does it occur at the database level?
6. What makes a cursor "opaque" and why does that matter for API evolution?
