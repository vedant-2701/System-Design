# Question

> Your company runs api.example.com. 
A new engineer proposes this configuration for an authenticated user profile API: `GET /users/me`.

```
Cache-Control: public, max-age=86400
Authorization: Bearer <jwt>
Set-Cookie: session=abc123
Access-Control-Allow-Origin: *
```
Identify every problem with this configuration. 

---

# Answer

**Issue 1 — `Cache-Control: public` on authenticated endpoint ✓**

Correct. `public` tells CDNs they can cache this response. But this is `/users/me` — personal profile data. User A's profile gets cached on CDN edge. User B makes the same request, hits the same edge node, gets User A's profile back.

This is a **data breach by misconfiguration.** Extremely dangerous. Should be `Cache-Control: private, no-store` for any authenticated personal data.

---

**Issue 2 — `Access-Control-Allow-Origin: *` on authenticated endpoint ✓**

Correct reasoning but you need to be more precise.

The real issue is this: `Access-Control-Allow-Origin: *` combined with credentials is **blocked by the browser spec entirely.** You cannot use wildcard origin with `Access-Control-Allow-Credentials: true`. Browser will reject it.

But even without credentials — wildcard on a user profile endpoint means any website can make cross-origin requests and read the response. If the API relies on cookies for auth, a malicious site could potentially read user data.

For authenticated APIs, always specify explicit allowed origins:
```
Access-Control-Allow-Origin: https://app.example.com
Access-Control-Allow-Credentials: true
```

---

**Issue 3 — Redundant auth mechanisms — Partially correct but your conclusion is wrong**

You said "it creates a mess on the server about what to trust." That's not quite right.

Using both JWT and session cookie simultaneously isn't inherently wrong — some architectures use cookies for browser clients and Bearer tokens for mobile/API clients on the same endpoint. The server checks both and accepts either.

**The real problem** is security surface area. Two auth mechanisms means two attack vectors. JWT in Authorization header is safe from CSRF. Session cookie needs CSRF protection. If you're already using JWT Bearer tokens, adding a session cookie gains you nothing and adds complexity and risk.

Also — this cookie has no security attributes:
```
Set-Cookie: session=abc123
```

Missing `HttpOnly` — JavaScript can steal it via XSS. Missing `Secure` — transmitted over HTTP in plaintext. Missing `SameSite` — CSRF vulnerable. This is a severely misconfigured cookie regardless of whether it's redundant.

---

**Issue 4 — Cookie cached on CDN — This is wrong**

CDNs cache **response bodies**, not cookies in a way that's served to other users. `Set-Cookie` headers from cached responses are actually stripped by most CDNs precisely to prevent this scenario.

**The real cookie problem** is what I described in issue 3 — missing security attributes make the cookie vulnerable to XSS theft and CSRF attacks independently of caching.

The actual danger of `Cache-Control: public` here is caching the **response body** — the JSON profile data — not the cookie.

---

**The issue you missed entirely:**

`max-age=86400` — 24 hour cache on a user profile.

Even if we fixed `public` to `private`, caching a user profile for 24 hours means if the user updates their name, email, or profile picture, their own browser serves stale data for up to 24 hours.

For user profile data, correct configuration is:
```
Cache-Control: private, no-store
```

Or if you want some caching:
```
Cache-Control: private, max-age=0, must-revalidate
ETag: "user123-v456"
```

Always revalidate, but skip body transmission if unchanged.

---

**Correct configuration for this endpoint:**

```
Cache-Control: private, no-store
Access-Control-Allow-Origin: https://app.example.com
Access-Control-Allow-Credentials: true
Set-Cookie: session=abc123; HttpOnly; Secure; SameSite=Strict; Max-Age=3600
```

And pick **one** auth mechanism — not both.

---