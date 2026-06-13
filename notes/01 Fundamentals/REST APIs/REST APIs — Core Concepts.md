# REST APIs — Core Concepts

## Tags
#rest #api-design #backend #http #networking

---

## Overview

- REST (Representational State Transfer) is an **architectural style**, not a protocol — a set of constraints on how to use HTTP
- HTTP is the protocol (mechanics: methods, headers, status codes, request/response)
- REST tells you **how to design** your API on top of HTTP — resource-oriented, uniform interface
- An API using HTTP is not automatically RESTful
- REST is theoretically protocol-agnostic but always used with HTTP in practice
- Core idea: model your system as **resources** (nouns), operate on them using HTTP verbs — not procedure calls (verbs in URLs)

---

## REST vs RPC

| Dimension | REST | RPC (gRPC, etc.) |
|---|---|---|
| Orientation | Resource (nouns) | Procedure (verbs) |
| URL style | `/users/123` | `/getUser` |
| HTTP method | Carries semantic meaning | Usually POST for everything |
| Best fit | CRUD-heavy, resource-centric | Operation-heavy, streaming |
| Caching | Native via GET | Harder |

---

## Richardson Maturity Model

Practical scale measuring how RESTful an API is.

```
Level 0 — One endpoint, POST everything (SOAP style)
Level 1 — Multiple resource URLs, still POST only
Level 2 — Resources + correct HTTP verbs + status codes  ← most production APIs
Level 3 — Level 2 + HATEOAS (hypermedia controls in responses)
```

- **Most production APIs (Stripe, GitHub, Twitter) live at Level 2** and correctly call themselves REST
- Level 3 is theoretically complete REST per Fielding's definition but rarely implemented

---

## HTTP Methods — Semantics & Idempotency

| Method | Semantic | Idempotent | Safe | Cacheable |
|---|---|---|---|---|
| GET | Fetch resource | Yes | Yes | Yes |
| POST | Create resource | No | No | No |
| PUT | Full replace | Yes | No | No |
| PATCH | Partial update | Not guaranteed | No | No |
| DELETE | Remove resource | Yes (state, not response) | No | No |

**Key distinctions:**
- **Idempotency = same system state**, not same response code. `DELETE /users/123` twice → first returns 204, second returns 404. System state is identical after both.
- **PATCH is not guaranteed idempotent.** `{ "balance": "+100" }` applied 10 times charges 10x. `{ "age": 25 }` applied 10 times is idempotent. Design explicitly.
- **PUT is idempotent by definition** — full replacement always produces the same state.
- **Safety** = no state change. GET and HEAD are safe. Safe implies idempotent.

**Why idempotency matters in distributed systems:**
- Networks fail. Clients retry. At-least-once delivery is the norm.
- Idempotent methods can be retried safely by clients, load balancers, and proxies without side effects.
- POST requires application-level idempotency keys to achieve safe retries — see [[Idempotency Keys]].

---

## Uniform Interface — The Core REST Constraint

REST's most important constraint. Four sub-constraints:

1. **Resource identification** — resources identified by URIs (`/users/123`)
2. **Manipulation through representations** — client manipulates resources via representations (JSON, XML)
3. **Self-descriptive messages** — each message contains enough info to describe how to process it (Content-Type, etc.)
4. **HATEOAS** — hypermedia as the engine of application state (see [[HATEOAS]])

Violating uniform interface = not truly RESTful, even if using HTTP correctly.

---

## HATEOAS

- Responses embed hypermedia controls telling clients what actions are available and where
- Client doesn't hardcode URLs — discovers them from server responses
- Server drives application state through links

```json
{
  "id": 123,
  "status": "processing",
  "_links": {
    "self":    { "href": "/orders/123",        "method": "GET"  },
    "cancel":  { "href": "/orders/123/cancel", "method": "POST" },
    "payment": { "href": "/orders/123/payment","method": "GET"  }
  }
}
```

- `cancel` link only present when order is cancellable — server encodes valid state transitions

**Why production teams skip HATEOAS:**
- Client teams hardcode URLs regardless — the theoretical benefit doesn't materialize
- No standard format (HAL, JSON-LD, Siren, custom `_links` — no consensus)
- Response bloat at high frequency endpoints
- Zero tooling support (OpenAPI, Postman, SDK generators built around Level 2)
- Debugging complexity — response shape changes per state

**Where HATEOAS is genuinely useful:** state-machine-driven resources with complex valid action sets — payments, bookings, order lifecycle.

---

## Failure Scenarios

- **POST-for-everything:** CDN caching impossible, idempotency guarantees lost at protocol level, monitoring/observability degrades (can't distinguish reads from writes in logs)
- **Wrong HTTP method semantics:** clients cannot infer intent, proxies cannot optimize, caches cannot function correctly
- **Ignoring idempotency on PATCH:** retry storms on network failures cause data corruption

---

## Scaling Considerations

- GET requests: horizontally cacheable at CDN, browser, proxy layers — free read scaling
- POST requests: always hit origin — no CDN absorption possible
- Correct method semantics enable the entire caching hierarchy to function; incorrect semantics collapse it

---

## Interview Perspective

- "Is every HTTP API a REST API?" — No. REST requires uniform interface, resource modeling, correct verb semantics
- "What's the difference between REST and RPC?" — Resource-oriented vs procedure-oriented; GET /users/123 vs POST /getUser
- "Why not POST for everything?" — Breaks caching, idempotency guarantees, protocol-level retry safety, observability
- "What level is most production APIs?" — Level 2. Know why Level 3 isn't practical.
- Always distinguish idempotency (state) from response code

---

## Common Mistakes

- Confusing "uses HTTP" with "is RESTful"
- Thinking `DELETE` is not idempotent because the response changes
- Assuming PATCH is always idempotent — it's not guaranteed
- Confusing REST (architectural style) with HTTP (protocol)
- Treating HATEOAS as mandatory for production APIs

---

## Related Concepts

- [[HTTP Methods & Idempotency]]
- [[HATEOAS]]
- [[API Versioning Strategies]]
- [[HTTP Caching — Cache-Control & ETag]]
- [[REST API Error Response Design]]
- [[Pagination — Cursor vs Offset]]
- [[OpenAPI & Swagger]]
- [[gRPC vs REST]]
- [[Idempotency Keys]]

---

## Revision Summary

- REST = constraints on how to use HTTP, not a protocol itself
- Core constraint: uniform interface — resources, not procedures
- Richardson Maturity: Level 0 → 3. Production = Level 2.
- Idempotency is about **state**, not response code
- PATCH not guaranteed idempotent; PUT always is
- HATEOAS: theoretically complete REST, practically skipped due to tooling/client coupling
- POST-for-everything kills caching, idempotency, and observability

---

## Active Recall Questions

1. What is the difference between HTTP and REST? Can you have HTTP without REST?
2. Why is DELETE idempotent even though the second call returns 404?
3. Give an example of a non-idempotent PATCH request and an idempotent one.
4. Why do most production teams skip HATEOAS? Name three concrete reasons.
5. What breaks specifically when you use POST for every operation?
6. At what Richardson Maturity Level do Stripe and GitHub APIs operate? Why don't they go to Level 3?
