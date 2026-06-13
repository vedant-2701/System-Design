# REST API Error Response Design

## Tags
#rest #api-design #backend #observability

---

## Overview

- HTTP status codes alone are insufficient for production APIs — machines and humans need structured error context
- Error responses serve three distinct audiences: frontend clients, backend consumers, and ops engineers
- Well-designed errors reduce support burden, enable correct retry logic, and accelerate debugging
- Error responses are part of the API contract — they must be as stable and versioned as success responses

---

## What Each Stakeholder Needs

| Stakeholder | Needs |
|---|---|
| Frontend developer | Machine-readable code (drive UI logic), field-level validation details (highlight form fields) |
| Backend service consumer | Error code (retry vs abort decision), `retryable` flag, `request_id` for correlation |
| Ops engineer | `request_id` / trace ID (find in distributed traces), timestamp, domain, error path |

---

## Production Error Response Schema

```json
{
  "error": {
    "code": "VALIDATION_FAILED",
    "message": "One or more fields failed validation",
    "details": [
      {
        "field": "quantity",
        "issue": "must be greater than 0",
        "received": -1
      },
      {
        "field": "address.pincode",
        "issue": "invalid format",
        "received": "ABCDE"
      }
    ],
    "retryable": false,
    "request_id": "req_7f3k2m9x",
    "timestamp": "2024-01-15T10:30:00Z",
    "docs_url": "https://api.example.com/docs/errors#VALIDATION_FAILED"
  }
}
```

**Field-by-field rationale:**

- `code` — machine-readable stable identifier. Clients switch on this. **Never switch on `message`** — messages change, codes are contracts.
- `message` — human-readable. For logs and developer debugging only. Never parse programmatically.
- `details` — field-level granularity. Return **all** validation failures at once, not one at a time. Never make clients fix one error, resubmit, discover the next.
- `retryable` — tells consumers whether to retry or abort. Critical for distributed systems.
- `request_id` — unique ID generated at API gateway, flows through entire system as trace ID. The single most important field for production debugging.
- `docs_url` — links directly to documentation for this error code. Reduces support tickets. (Stripe does this.)

---

## Error Code Taxonomy

**Flat codes:**
```
VALIDATION_FAILED
INSUFFICIENT_STOCK
PAYMENT_DECLINED
```

**Hierarchical codes (preferred at scale):**
```
ORDER_VALIDATION_FAILED
ORDER_INSUFFICIENT_STOCK
PAYMENT_CARD_DECLINED
PAYMENT_FRAUD_DETECTED
```

Hierarchical codes allow consumers to handle broad categories or specific cases:

```javascript
if (error.code.startsWith('PAYMENT_')) {
  // route to payment error handler
}
if (error.code === 'PAYMENT_CARD_DECLINED') {
  // show specific card declined UI
}
```

---

## Retryable vs Non-Retryable Errors

Critical distinction for distributed system consumers.

```json
{
  "error": {
    "code": "DOWNSTREAM_TIMEOUT",
    "message": "Payment processor did not respond in time",
    "retryable": true,
    "retry_after_seconds": 5,
    "request_id": "req_7f3k2m9x"
  }
}
```

- `retryable: true` — transient failure, safe to retry with backoff
- `retryable: false` — permanent failure, retrying will not help

**Without this signal:** clients either retry everything (causing retry storms on already-stressed systems) or retry nothing (silent failures on transient errors that would have resolved).

HTTP status codes give rough signal (503 usually retryable, 400 usually not) but insufficient granularity. A 500 can be a transient DB blip or a permanent code bug.

---

## Security: 404 vs 403 for Unauthorized Resources

**Scenario:** User requests `/orders/123` which belongs to another user.

**Return 404, not 403.**

- 403 confirms the resource exists — enables **resource enumeration attacks**. Attacker iterates `/orders/1`, `/orders/2`... and maps existing IDs from 403 vs 404 responses.
- 404 is correct semantically: the resource doesn't exist *for that user*. No existence confirmation.
- GitHub uses this pattern — private repositories return 404 to unauthorized users.

**When 403 is correct:** resource existence is already public knowledge (admin-only endpoints everyone knows exist).

---

## Security: Never Leak Internals

**Never expose in production error responses:**
```json
// BAD — security vulnerability
{
  "error": "NullPointerException at OrderService.java:234",
  "stack_trace": "at com.example.OrderService.process(OrderService.java:234)..."
}
```

Exposes: stack structure, file paths, library versions, internal class names. Useful for attackers, never for legitimate consumers.

**Log stack traces internally. Never expose externally.**

---

## HTTP Status Code Guidance

| Range | Meaning | Examples |
|---|---|---|
| 2xx | Success | 200 OK, 201 Created, 204 No Content |
| 400 | Client error — bad request | 400 Validation, 401 Auth, 403 Forbidden, 404 Not Found, 409 Conflict, 422 Unprocessable, 429 Rate Limited |
| 500 | Server error — server fault | 500 Internal, 502 Bad Gateway, 503 Unavailable, 504 Timeout |

**Key distinctions:**
- 401 = unauthenticated (no valid credentials). 403 = authenticated but unauthorized (valid credentials, insufficient permissions).
- 404 vs 403 security consideration above.
- 409 Conflict = resource state conflict (duplicate idempotency key, optimistic lock failure).
- 422 Unprocessable = syntactically valid but semantically invalid request.
- 429 = rate limited. Include `Retry-After` header.
- 503 = service unavailable (overloaded or down). Include `Retry-After` if temporary.

---

## Failure Scenarios

- **Switching on message string:** message changes in new version → client logic silently breaks
- **Single validation error returned:** client fixes it, resubmits, discovers next error → poor UX, wasted round trips
- **No `retryable` flag:** consumer retries permanent failures indefinitely → wasted load, retry storm
- **No `request_id`:** production incident → no way to correlate client complaint to server logs
- **Exposing stack traces:** attacker maps internal architecture

---

## Real-World Usage

- **Stripe:** hierarchical error codes, field-level details, `docs_url` per error code, `request_id` on every response
- **GitHub:** 404 for unauthorized private repo access
- **Google APIs:** `status`, `message`, `details` array with typed error messages

---

## Interview Perspective

- "Design an error response schema" — must include code, message, details, request_id, retryable
- "404 or 403 for unauthorized resource?" — always 404. Explain resource enumeration.
- "How do you help consumers decide to retry?" — `retryable` flag, `retry_after_seconds`
- "Why not just return HTTP status codes?" — insufficient granularity for machines, no field-level context, no trace correlation
- Common mistake: returning stack traces, switching on message strings

---

## Related Concepts

- [[REST APIs — Core Concepts]]
- [[HTTP Methods & Idempotency]]
- [[Idempotency Keys]]
- [[Distributed Tracing]]
- [[API Versioning Strategies]]
- [[Rate Limiting]]

---

## Revision Summary

- Three audiences: frontend (field details), backend consumer (retryable + request_id), ops (request_id + timestamp)
- `code` is stable contract. `message` is human-readable only. Never switch on message.
- Return all validation errors at once — never one at a time
- `retryable` flag prevents retry storms and silent failures
- `request_id` is mandatory for production debugging
- 404 over 403 for unauthorized resource access — prevents resource enumeration
- Never expose stack traces externally

---

## Active Recall Questions

1. Why should clients never switch on the `message` field? What should they switch on instead?
2. A service gets 500 on a downstream call. How does it know whether to retry? What field tells it?
3. User requests `/orders/123` owned by another user. Do you return 403 or 404? Why?
4. Why return all validation errors at once instead of the first one?
5. A production incident happens. An ops engineer only has the client's complaint. What single field in the error response lets them find the relevant server logs?
6. What information should never appear in a production error response and why?
