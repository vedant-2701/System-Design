# HTTP Status Codes

## Tags
#http #backend #api-design

---

## Overview

- Numeric codes in HTTP responses communicating outcome category and semantics
- Enable generic middleware (load balancers, circuit breakers, proxies) to make routing/retry decisions **without parsing response bodies**
- Distinguish client errors from server errors — critical for debugging, monitoring, and alerting

---

## Categories

| Range | Category | Consumer Action |
|---|---|---|
| 1xx | Informational | Protocol negotiation in progress |
| 2xx | Success | Request fulfilled |
| 3xx | Redirection | Client must follow up |
| 4xx | Client Error | Fix the request |
| 5xx | Server Error | Retry, escalate, alert |

---

## Key Codes by Category

### 1xx — Informational

**100 Continue**
- Client sends large body; first asks "will you accept this?"
- Server responds 100 → client sends body
- Prevents wasting bandwidth on a body the server will reject (e.g. 413 Payload Too Large)

**101 Switching Protocols**
- Response to `Upgrade: websocket` header
- Connection transitions from HTTP to WebSocket protocol
- Required handshake for all WebSocket connections

---

### 2xx — Success

**200 OK** — generic success; appropriate for GET responses

**201 Created** — resource created via POST; include `Location` header pointing to new resource
- Do not return 200 for creation — semantically wrong, breaks API consumers

**202 Accepted** — request queued, processing not yet complete
- Used for async workflows (video transcoding, email sending)
- Client should poll or use webhooks for result

**204 No Content** — success with no body
- Used for DELETE, or PUT when not returning updated resource
- Client must not expect a body — some clients break if 204 has a body

**206 Partial Content** — response to `Range` request
- Powers video seeking, resumable downloads
- Server returns `Content-Range` header indicating which bytes are included

---

### 3xx — Redirection

**301 Moved Permanently** — cached aggressively by browsers
- Production risk: incorrect 301 cached forever in client — clearing requires user action
- Use for true permanent moves only

**302 Found** — temporary redirect; not cached
- Browser may change POST to GET — use 307 if method must be preserved

**304 Not Modified** — response to conditional request (`If-None-Match`, `If-Modified-Since`)
- No body sent — client uses cached copy
- Core of HTTP validation caching

**307 Temporary Redirect** — preserves HTTP method (POST stays POST)
**308 Permanent Redirect** — preserves HTTP method, cached permanently

**301 vs 307 gotcha**: redirecting a POST form with 301/302 silently converts it to GET. Use 307 for method-preserving temporary redirects.

---

### 4xx — Client Errors

**400 Bad Request** — malformed syntax, invalid JSON, missing required fields

**401 Unauthorized** — **actually means unauthenticated** (historical naming error)
- "I don't know who you are" — client should authenticate

**403 Forbidden** — authenticated but not authorized
- "I know who you are, you lack permission"
- 401 vs 403 distinction is critical — confusing them breaks client auth flows

**404 Not Found** — resource doesn't exist
- Intentionally returned instead of 403 to hide resource existence from attackers

**409 Conflict** — state conflict: duplicate creation, optimistic lock failure, version mismatch

**410 Gone** — resource permanently deleted (unlike 404, signals intentional removal)

**422 Unprocessable Entity** — syntactically valid but semantically invalid
- JSON is parseable but business validation failed; more precise than 400

**429 Too Many Requests** — rate limited; include `Retry-After` header

---

### 5xx — Server Errors

**500 Internal Server Error** — unhandled exception; something unexpected broke

**502 Bad Gateway** — proxy received invalid/garbage response from upstream

**503 Service Unavailable** — server running but refusing requests (overloaded, maintenance)
- Include `Retry-After`; circuit breakers watch for this

**504 Gateway Timeout** — proxy got no response from upstream within timeout window

**502 vs 504 in production**:
- 502 = upstream responded with something invalid
- 504 = upstream didn't respond at all
- Diagnosing 504 spikes → identify which upstream service stopped responding

---

## Production Engineering Scenarios

### Payment Charge Succeeds, DB Write Fails
- Returning 500 → client retries → potential double charge
- Returning 200 → client sees success but no DB record exists
- **Correct design**: write pending DB record first, then charge, then confirm
- If already in bad state: return 500 + use idempotency keys so retry returns original result without double-charging

### Status Codes and Infrastructure
- Circuit breakers open on repeated 503s — they don't read response bodies
- Load balancers route away from servers returning 5xx
- Monitoring alerts fire on 5xx rate thresholds
- This is why correct status codes matter beyond API consumers

---

## Common Mistakes

- Using 200 for creation instead of 201
- Confusing 401 (unauthenticated) with 403 (unauthorized)
- Using 400 for business validation failures instead of 422
- Not including `Retry-After` on 429 and 503
- Using 301 for temporary redirects — cached forever
- Using 302 to redirect POST — silently converts to GET
- Returning 500 for all errors instead of distinguishing 502/503/504

---

## Interview Perspective

- Always distinguish 401 vs 403 — interviewers test this
- Know that 3xx codes have caching behavior differences
- Be able to explain what happens in infrastructure (load balancers, circuit breakers) on 5xx vs 4xx
- Payment/idempotency scenario with 500 vs 200 is a common senior-level question

---

## Revision Summary

- 1xx: protocol negotiation (101 = WebSocket upgrade)
- 2xx: 200 GET, 201 POST creation, 202 async, 204 no body, 206 range
- 3xx: 301 cached permanently, 302 temporary, 304 cache valid, 307/308 preserve method
- 4xx: 400 malformed, 401 unauthenticated, 403 unauthorized, 409 conflict, 422 semantic invalid, 429 rate limited
- 5xx: 500 unhandled, 502 bad upstream response, 503 unavailable, 504 upstream timeout
- 502 = upstream responded badly; 504 = upstream didn't respond
- Infrastructure (circuit breakers, LBs) uses status codes without parsing bodies

---

## Active Recall Questions

1. What is the difference between 401 and 403?
2. Why is 307 preferred over 302 for redirecting POST requests?
3. What's the risk of returning 301 incorrectly?
4. What does a 304 response contain in its body?
5. Distinguish 502 from 504 — what does each tell you about the upstream?
6. A payment charge succeeds but the DB write fails — what status code do you return and why is it hard?
7. Why do circuit breakers care about status codes?

---

## Related Concepts

- [[HTTP Caching Headers]]
- [[HTTPS Request Lifecycle]]
- [[HTTP Authentication Headers]]
- [[Rate Limiting]]
- [[Idempotency]]
