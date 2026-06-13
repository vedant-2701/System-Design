# HSTS & CORS

## Tags
#http #security #networking #backend #browser

---

## HSTS — HTTP Strict Transport Security

### Problem It Solves
User types `example.com` — browser makes initial HTTP request (port 80). Attacker on the network intercepts this plaintext request and strips TLS — **SSL stripping attack**. User ends up on HTTP without knowing.

HSTS instructs browsers to **never make HTTP requests to this domain** — all requests internally rewritten to HTTPS before sending.

### Header
```
Strict-Transport-Security: max-age=31536000; includeSubDomains; preload
```

| Directive | Meaning |
|---|---|
| `max-age=N` | Remember this policy for N seconds (1 year = 31536000) |
| `includeSubDomains` | Apply policy to all subdomains |
| `preload` | Eligible for browser preload list |

### How It Works
1. First HTTPS response includes HSTS header
2. Browser stores policy with expiry
3. For `max-age` duration: all HTTP requests silently upgraded to HTTPS before leaving browser
4. Even if user manually types `http://`, browser sends `https://`

### The First-Request Problem
HSTS only works **after** the first successful HTTPS response. That first visit is still vulnerable to SSL stripping.

### HSTS Preload — Solution to First-Request Problem
- Browser vendors maintain hardcoded list of HTTPS-only domains
- Submit at `hstspreload.org`
- Browsers ship with your domain in preload list
- HTTPS enforced on first ever visit, even on fresh browser installs, even without ever seeing your HSTS header

### Removal Is Painful
- Set `max-age=31536000` → need HTTP for some reason later → must wait 1 year for all cached policies to expire
- **Production practice**: start with short `max-age` (e.g. 300), verify everything works, incrementally increase to 1 year, then submit to preload

### `includeSubDomains` Risk
If any subdomain (`legacy.example.com`) doesn't support HTTPS, `includeSubDomains` breaks it entirely. Audit all subdomains before enabling.

---

## CORS — Cross-Origin Resource Sharing

### Same-Origin Policy (SOP)
Browser security model: JavaScript running on `evil.com` cannot read responses from `api.bank.com`. Origin = scheme + domain + port.

```
https://example.com       ← origin A
http://example.com        ← different origin (different scheme)
https://api.example.com   ← different origin (different subdomain)
https://example.com:8080  ← different origin (different port)
```

SOP prevents: malicious sites reading your data from other origins.

### What CORS Solves
Legitimate cross-origin requests. Frontend at `app.example.com` needs to call `api.example.com`. CORS allows servers to explicitly permit specific cross-origin requests.

**CORS is a browser mechanism.** It does not protect your server from non-browser clients. `curl` ignores CORS entirely. CORS is about what the browser will allow JavaScript to read — not about what the server will accept.

### Simple vs Preflight Requests

**Simple Requests** (browser sends directly, checks response):
- Methods: GET, POST, HEAD
- Safe content types (form data, plain text)
- No custom headers

**Preflight Requests** (browser sends OPTIONS first):
- Triggered by: PUT, DELETE, PATCH, custom headers, JSON content type
- Browser asks server for permission before sending actual request

```
OPTIONS /users/123 HTTP/1.1
Origin: https://app.example.com
Access-Control-Request-Method: DELETE
Access-Control-Request-Headers: Authorization, Content-Type
```

Server responds:
```
Access-Control-Allow-Origin: https://app.example.com
Access-Control-Allow-Methods: GET, POST, PUT, DELETE
Access-Control-Allow-Headers: Authorization, Content-Type
Access-Control-Max-Age: 86400
```

Browser checks all three. If permitted, sends actual DELETE request.

### Key CORS Response Headers

| Header | Purpose |
|---|---|
| `Access-Control-Allow-Origin` | Which origins are permitted |
| `Access-Control-Allow-Methods` | Which HTTP methods are permitted |
| `Access-Control-Allow-Headers` | Which request headers are permitted |
| `Access-Control-Allow-Credentials` | Whether cookies/auth headers are included |
| `Access-Control-Max-Age` | Cache preflight response for N seconds |
| `Access-Control-Expose-Headers` | Which response headers JS can read |

### `Access-Control-Max-Age` — Performance Critical
Without it, every PUT/DELETE/custom-header request triggers an OPTIONS preflight — doubles the RTT for those requests. Set to a high value (86400) to cache preflight responses.

### Wildcard vs Explicit Origin

```
Access-Control-Allow-Origin: *           ← all origins permitted (public APIs)
Access-Control-Allow-Origin: https://app.example.com  ← explicit (authenticated APIs)
```

**Wildcard + credentials is blocked by spec**:
```
# This combination is rejected by browsers:
Access-Control-Allow-Origin: *
Access-Control-Allow-Credentials: true
```
Credentialed requests (cookies, Authorization header) require explicit origin — never wildcard.

### CORS Does Not Replace Server-Side Authorization
CORS controls what browsers allow JavaScript to read. Your server must still validate auth tokens, check permissions, and reject unauthorized requests independently. A misconfigured server with `Allow-Origin: *` on an authenticated endpoint is a security risk even if CORS "works."

---

## Failure Scenarios

### HSTS
- **Premature preload submission** — submitted to preload list before all subdomains support HTTPS; breaks subdomains for years across all users
- **`includeSubDomains` with non-HTTPS subdomain** — breaks subdomain completely
- **Short `max-age` in production** — SSL stripping window reopens between browser visits

### CORS
- **Missing `Access-Control-Max-Age`** — every cross-origin mutation request pays double RTT for preflight
- **`Allow-Origin: *` on authenticated API** — any website can read authenticated user data via JS
- **Wildcard + credentials** — browser rejects; auth breaks silently
- **CORS configured at app layer but not at API gateway** — preflight OPTIONS hits gateway, gateway returns 404/405, browser blocks actual request
- **Missing `Access-Control-Expose-Headers`** — JS cannot read custom response headers even if CORS allows the request

---

## Common Mistakes

- Thinking CORS protects the server — it only controls browser JS behavior
- Setting `Allow-Origin: *` on APIs that use cookies or Authorization headers
- Not configuring CORS at the API gateway/load balancer level for OPTIONS requests
- Forgetting `Access-Control-Max-Age` — performance cost on every mutation request
- Enabling `includeSubDomains` without auditing all subdomains for HTTPS support
- Submitting to HSTS preload too early — very hard to undo

---

## Interview Perspective

- "Does CORS protect the server?" → No — it's a browser constraint; `curl` ignores it
- HSTS preload: explain the first-request problem it solves
- Know the preflight flow: when it's triggered, what OPTIONS request looks like, what headers server must return
- Wildcard + credentials constraint — frequently tested
- `Access-Control-Max-Age` as performance optimization

---

## Revision Summary

- HSTS: instructs browser to always use HTTPS; protects against SSL stripping
- First-request still vulnerable → HSTS Preload hardcodes HTTPS-only domains in browser
- Removing HSTS takes `max-age` to expire — start small, increase gradually
- CORS: browser mechanism allowing cross-origin JS requests with server permission
- SOP: JavaScript cannot read cross-origin responses by default
- Preflight (OPTIONS) triggered by: non-simple methods, custom headers, JSON content type
- Wildcard origin cannot be combined with credentials — use explicit origin
- CORS does not replace server-side auth — it's purely a browser JS constraint
- `Access-Control-Max-Age` caches preflight — critical for performance

---

## Active Recall Questions

1. What attack does HSTS prevent? How does it work?
2. What is the first-request problem with HSTS and how does preload solve it?
3. Why is removing HSTS from a preload list difficult?
4. What is the Same-Origin Policy? What does it prevent?
5. CORS is a browser mechanism — what does this mean for `curl` requests?
6. What triggers a CORS preflight request? Walk through the OPTIONS flow
7. Why can't you use `Access-Control-Allow-Origin: *` with credentials?
8. What does `Access-Control-Max-Age` do and why does it matter for performance?

---

## Related Concepts

- [[TLS Handshake & Certificate Chain]]
- [[HTTP Authentication — Cookies & Sessions]]
- [[HTTPS Request Lifecycle]]
- [[HTTP Version Evolution]]
- [[API Security]]
