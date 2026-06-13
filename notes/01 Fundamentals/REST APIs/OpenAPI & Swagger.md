# OpenAPI & Swagger

## Tags
#rest #api-design #backend #tooling #documentation

---

## Overview

- **OpenAPI Specification** — a language-agnostic, machine-readable format (YAML/JSON) for describing REST APIs
- **Swagger** — the tooling ecosystem built around OpenAPI (Swagger UI, Swagger Editor, Swagger Codegen)
- Purpose: make the API contract explicit, versioned, and executable — not just documented in prose
- Without a formal spec, API contracts live in outdated docs, tribal knowledge, and Postman collections that drift from implementation

---

## What OpenAPI Enables

```
OpenAPI Spec (.yaml / .json)
         ↓
┌────────────────────────────────────┐
│  Auto-generated interactive docs  │  (Swagger UI, Redoc)
│  Auto-generated client SDKs       │  (TypeScript, Python, Java, Go)
│  Auto-generated server stubs      │  (implement against spec)
│  Contract testing                 │  (verify implementation matches spec)
│  API gateway configuration        │  (AWS API Gateway, Kong)
│  Mock servers                     │  (frontend dev before backend ready)
└────────────────────────────────────┘
```

---

## Spec-First vs Code-First

### Code-First
- Write implementation → generate spec from annotations/reflection
- Fast to start
- Spec reflects implementation quirks, not intentional API design
- Design decisions made accidentally during coding

### Spec-First (Preferred at Mature Teams)
- Write OpenAPI spec first → generate server stubs → implement against spec
- Forces deliberate API design before a single line of implementation
- Breaking changes visible in spec diff before deployment
- Frontend/mobile can work against mock server while backend is being built
- API design reviewed and approved before costly implementation

**The key insight:** API design is cheap to change in a YAML file. API design is expensive to change once clients are integrated.

---

## Contract Testing — The Real Production Value

The most important capability OpenAPI enables in production.

**Problem:** Implementation drifts from spec over time. A field gets renamed, a status code changes, a required param becomes optional. Clients break.

**Solution:** Contract testing verifies that the running server's actual responses match the OpenAPI spec.

Tools: Dredd, Schemathesis, Pact (consumer-driven variant)

```
OpenAPI Spec  ←→  Running Server Response
         Contract Test
         PASS: implementation matches spec
         FAIL: field missing, wrong type, wrong status code → caught before deployment
```

Without contract testing, breaking changes are discovered when clients fail in production.

---

## Key OpenAPI Concepts

**Paths** — endpoint definitions with HTTP method, parameters, request body, and responses

**Components** — reusable schemas, response objects, parameters. Prevents repetition.

**$ref** — reference to a component. Single source of truth for shared types.

**Tags** — group related endpoints. Reflected in Swagger UI as collapsible sections.

**Security Schemes** — define auth mechanisms (API key, Bearer token, OAuth 2.0 flows)

---

## Minimal Example Structure

```yaml
openapi: 3.0.3
info:
  title: Orders API
  version: 1.0.0

paths:
  /orders/{id}:
    get:
      summary: Get order by ID
      parameters:
        - name: id
          in: path
          required: true
          schema:
            type: integer
      responses:
        '200':
          description: Order found
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/Order'
        '404':
          description: Order not found

components:
  schemas:
    Order:
      type: object
      properties:
        id:
          type: integer
        status:
          type: string
          enum: [pending, processing, delivered, cancelled]
        total:
          type: number
```

---

## Tradeoffs

| Dimension | Benefit | Cost |
|---|---|---|
| Spec-first | Deliberate design, early review | Upfront time before any implementation |
| Code-first | Fast start | Spec reflects implementation bugs, not intent |
| Contract testing | Catches breaking changes pre-deploy | Test setup complexity |
| SDK generation | Clients get typed SDKs automatically | Generated code can be verbose or opinionated |
| Mock servers | Frontend/mobile unblocked | Mock may not reflect real behavior edge cases |

---

## Failure Scenarios

- **Spec drift:** code-first with no contract testing → spec becomes outdated within weeks → documentation misleads consumers
- **No spec:** API contract exists only in developers' heads → onboarding is slow, integration is error-prone, breaking changes are invisible
- **Generated SDK over-pinned:** clients pin to generated SDK version → server evolves → client locked to old behavior
- **Missing error response schemas:** spec documents success paths only → consumers don't know error shapes → build fragile error handling

---

## Real-World Usage

- **Stripe:** spec-first, full OpenAPI spec published, client SDKs generated from spec
- **GitHub:** OpenAPI spec public, used to generate official client libraries
- **AWS API Gateway:** accepts OpenAPI spec to configure endpoints, auth, and validation automatically
- **Redoc / Swagger UI:** standard documentation portals generated from spec

---

## Interview Perspective

- "How do you prevent API documentation from drifting?" — contract testing against OpenAPI spec
- "Spec-first vs code-first?" — spec-first preferred; forces deliberate design, enables early review
- "What's the value of OpenAPI beyond documentation?" — SDK generation, mock servers, contract testing, API gateway config
- Common mistake: treating OpenAPI as documentation-only, ignoring contract testing

---

## Related Concepts

- [[REST APIs — Core Concepts]]
- [[API Versioning Strategies]]
- [[REST API Error Response Design]]
- [[gRPC & Protocol Buffers]]

---

## Revision Summary

- OpenAPI = machine-readable API contract in YAML/JSON
- Swagger = tooling ecosystem around OpenAPI
- Spec-first > code-first for deliberate design and early review
- Contract testing is the most important production use case
- Enables: docs, SDK generation, mock servers, API gateway config
- Without spec: drift, tribal knowledge, breaking changes invisible until production

---

## Active Recall Questions

1. What is the difference between OpenAPI and Swagger?
2. Why is spec-first preferred over code-first? What problem does it prevent?
3. What is contract testing and why does it matter in production?
4. Name four things an OpenAPI spec enables beyond interactive documentation.
5. How does OpenAPI help frontend teams before the backend is ready?
6. What happens to API contracts without a formal spec in a team of 10+ engineers?
