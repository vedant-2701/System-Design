# HTTP Authentication — Headers, Cookies & Sessions

## Tags
#http #authentication #security #backend #cookies

---

## Overview

- HTTP is stateless — every request must carry identity information independently
- Three locations for identity: `Authorization` header, `Cookie` header, request body/URL (avoid)
- Two architectural approaches: **server-side sessions** (cookie + session store) and **stateless tokens** (JWT in Authorization header)

---

## Authorization Header Types

### Basic Auth
```
Authorization: Basic dmVkYW50OnBhc3N3b3Jk
```
- Value = `base64(username:password)` — trivially reversible, not encrypted
- Safe **only over TLS** — plaintext password in base64 is not security
- No expiry, no revocation, credentials sent on every request
- Use case: simple internal tooling, machine-to-machine where simplicity matters

### Bearer Token
```
Authorization: Bearer eyJhbGciOiJIUzI1NiJ9...
```
Two token types:
- **Opaque token** — random string; server looks up in Redis/DB to find user context
- **JWT** — self-contained; server validates signature and reads claims without DB lookup

### Digest Auth
- Server sends challenge (nonce); client hashes credentials with nonce
- Password never transmitted — but TLS makes this mostly unnecessary
- Complex to implement correctly; rarely used in modern APIs

### API Key
```
Authorization: ApiKey xyz123
X-API-Key: xyz123          ← also common as custom header
```
- Simple, stateless, no expiry by default
- Used heavily for server-to-server communication
- Rotation strategy required — leaked keys must be invalidatable

### HMAC Signature
```
Authorization: HMAC-SHA256 Credential=keyId, Signature=abc123
```
- Client signs request (method + path + timestamp + body hash) with shared secret
- Server recomputes and verifies
- Proves **request integrity** — body cannot be tampered in transit
- AWS Signature V4 uses this pattern
- Timestamp in signature prevents replay attacks

---

## Cookies

### Mechanism
Server sends:
```
Set-Cookie: session_id=abc123; HttpOnly; Secure; SameSite=Strict; Max-Age=3600; Path=/
```
Browser stores it. On every subsequent request to that domain:
```
Cookie: session_id=abc123
```
**Transmission is automatic** — no JavaScript required. This is the power and the danger.

### Security Attributes — All Critical

| Attribute | Effect | Risk if Missing |
|---|---|---|
| `HttpOnly` | JS cannot read via `document.cookie` | XSS can steal cookie |
| `Secure` | Only sent over HTTPS | Transmitted in plaintext over HTTP |
| `SameSite=Strict` | Never sent on cross-site requests | CSRF vulnerable |
| `SameSite=Lax` | Sent on top-level navigation only | Partial CSRF protection |
| `SameSite=None` | Always sent (requires `Secure`) | CSRF vulnerable; needed for cross-site OAuth |
| `Max-Age` / `Expires` | Cookie lifetime | Session cookie deleted on browser close |
| `Domain` | Which subdomains receive cookie | Scope too broad or too narrow |

**Always set `HttpOnly`, `Secure`, and `SameSite` on session cookies in production.**

---

## Session-Based Authentication

### Flow
```
1. POST /login with credentials
2. Server validates
3. Server creates session: session_id → {user_id, role, expires} in Redis
4. Set-Cookie: session_id=abc123
5. Every request: Cookie: session_id=abc123
6. Server looks up session_id in Redis → user context
```

### Scaling Problem
Session state is server-side. With 10 application servers:
- **Sticky sessions** — same user always hits same server; fragile, complicates deployments
- **Centralized session store (Redis)** — all servers share one Redis; adds RTT per request, adds Redis as single point of failure

This is why stateless JWT became popular.

---

## CSRF — Cross-Site Request Forgery

### Attack
```
1. User logged into bank.com — has session cookie
2. User visits evil.com
3. evil.com has hidden form POSTing to bank.com/transfer
4. Browser automatically attaches bank.com session cookie
5. Bank processes transfer — thinks user initiated it
```

### Why Cookies Are Vulnerable
Automatic transmission means any page the user visits can trigger requests with their credentials attached.

### Mitigations
- `SameSite=Strict` — cookie never sent on cross-site request (breaks some OAuth flows)
- `SameSite=Lax` — good default; blocks cross-site POST, allows top-level GET navigation
- **CSRF tokens** — server embeds random token in form; validates on submit; attacker can't read it (same-origin policy)
- **Double submit cookie** — CSRF token in both cookie and request body; server verifies they match

### Why JWT in Authorization Header Is Not CSRF Vulnerable
Browser never automatically attaches Authorization headers. JavaScript must explicitly set them. Cross-origin JavaScript cannot read tokens due to same-origin policy.

---

## Cookie vs JWT Tradeoffs

| Dimension | Session Cookie | JWT |
|---|---|---|
| State storage | Server-side (Redis) | Client-side (token) |
| Revocation | Easy — delete from Redis | Hard — valid until expiry |
| Scalability | Requires shared session store | Stateless — any server validates |
| XSS risk | `HttpOnly` protects token | Token in localStorage is stealable |
| CSRF risk | Requires SameSite + CSRF token | Not vulnerable in Authorization header |
| Token size | Small (session ID only) | Larger (carries all claims) |
| Microservices | Session store becomes bottleneck | Each service validates independently |
| Offline validation | No | Yes (verify signature without network) |

### JWT Revocation Problem
JWT is valid until expiry — you cannot invalidate it without a blocklist, which reintroduces server-side state.

**Production pattern**: short-lived JWTs (15 min access token) + long-lived refresh token in `HttpOnly` cookie. Compromise window bounded by access token TTL. Refresh token can be revoked server-side.

---

## Common Configuration Mistakes

```
# Dangerous — never do this on authenticated endpoints
Cache-Control: public, max-age=86400   # CDN caches user data → data breach
Access-Control-Allow-Origin: *          # Wildcard + credentials = blocked by spec; wildcard alone leaks data
Set-Cookie: session=abc123             # Missing HttpOnly, Secure, SameSite → XSS + CSRF vulnerable
Authorization: Bearer <jwt>            # Using both cookie and JWT simultaneously → redundant attack surface
```

---

## Failure Scenarios

- **Missing `HttpOnly`** — XSS attack exfiltrates session cookie; attacker hijacks session
- **Missing `Secure`** — cookie sent over HTTP on mixed-content page; MITM captures it
- **`SameSite=None` without `Secure`** — browser rejects the cookie entirely
- **Long-lived JWT without revocation** — compromised token valid for days; no way to invalidate
- **Session store (Redis) down** — all authenticated requests fail if sessions aren't replicated
- **CSRF token not validated on state-changing requests** — CSRF attack succeeds

---

## Real-World Usage

- **Google, GitHub** — session cookies with CSRF tokens for browser; OAuth tokens for API clients
- **Stripe** — API keys with HMAC for webhook validation
- **AWS** — HMAC Signature V4 for all API requests
- **Mobile apps** — JWT in Authorization header (no cookie support issues)

---

## Interview Perspective

- Know the difference between session-based and token-based auth architectures
- Explain JWT revocation problem and the short-lived token + refresh token pattern
- CSRF: why it exists, why cookies are vulnerable, why JWT in header is not
- Cookie security attributes: be able to explain all five (HttpOnly, Secure, SameSite, Max-Age, Domain)
- Scaling sessions: sticky sessions vs centralized store tradeoffs

---

## Revision Summary

- Basic = base64 credentials; Bearer = token; HMAC = signed request; API Key = shared secret
- Cookies transmitted automatically — power and CSRF vulnerability
- `HttpOnly` blocks XSS; `Secure` blocks HTTP transmission; `SameSite` blocks CSRF
- Sessions: state server-side in Redis; scalability problem → JWT
- JWT: stateless, can't revoke; production fix = short TTL + refresh tokens
- CSRF: cookie auto-attach exploited; JWT in Authorization header not vulnerable
- Never `Cache-Control: public` on authenticated endpoints

---

## Active Recall Questions

1. What does `HttpOnly` protect against? What does `Secure` protect against?
2. Why is JWT in the Authorization header not vulnerable to CSRF?
3. Explain the JWT revocation problem and the production solution
4. What is the scaling problem with server-side sessions and how is it solved?
5. What is a CSRF attack? How does `SameSite=Strict` mitigate it?
6. Why can't you use `Access-Control-Allow-Origin: *` with `Access-Control-Allow-Credentials: true`?
7. What does HMAC authentication prove that Bearer token auth does not?
8. Difference between opaque tokens and JWTs — when would you choose each?

---

## Related Concepts

- [[TLS Handshake & Certificate Chain]]
- [[CORS]]
- [[HTTP Caching Headers]]
- [[OAuth 2.0]]
- [[JWT Security]]
- [[Rate Limiting]]
