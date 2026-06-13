# HTTP Content Negotiation Headers

## Tags
#http #backend #api-design #performance

---

## Overview

- Mechanism for client and server to agree on response format, encoding, and language
- Client advertises capabilities; server picks the best match and declares what it chose
- Enables a single endpoint to serve multiple content types without separate URLs

---

## Client Request Headers

```
Accept: application/json, application/xml;q=0.9, */*;q=0.8
Accept-Encoding: gzip, br, deflate
Accept-Language: en-US, en;q=0.9, hi;q=0.7
Accept-Charset: utf-8  ← rarely used today; UTF-8 assumed
```

### Quality Values (q)
`q` parameter (0.0–1.0) expresses preference weight. Default = 1.0 if omitted.

```
Accept: application/json, application/xml;q=0.9, */*;q=0.8
```
Priority: JSON (1.0) > XML (0.9) > anything (0.8)

Server picks the highest-weight format it supports.

---

## Server Response Headers

```
Content-Type: application/json; charset=utf-8
Content-Encoding: br
Content-Language: en-US
Content-Length: 1284
```

Server declares exactly what it's sending. Client parses accordingly.

---

## Compression — Accept-Encoding

| Encoding | Compression | Support | Use Case |
|---|---|---|---|
| `gzip` | Good | Universal | Default for most content |
| `br` (Brotli) | 15-20% better than gzip | Modern browsers | Preferred when supported |
| `deflate` | Older | Legacy | Rarely used now |
| `identity` | None | Universal | Raw uncompressed |

**Always enable both `gzip` and `br` on servers.** Brotli is significantly better for text content (JSON, HTML, JS) at equivalent CPU cost.

Compression is most effective on text. Binary formats (images, video) are already compressed — don't compress them.

---

## Vary Header — Caching Interaction

```
Vary: Accept-Encoding
```

Tells CDN/proxies: cache separate copies per `Accept-Encoding` value.

Without `Vary`: CDN might cache a Brotli-compressed response and serve it to a client that only supports gzip → decoding failure.

With `Vary: Accept-Encoding`: CDN maintains separate cache entries per encoding variant.

**`Vary` increases cache storage requirements** — one cache entry per variant combination. Be conservative with what you vary on.

---

## Common Content Types

```
application/json          ← REST API standard
application/xml           ← legacy/enterprise APIs
application/x-protobuf    ← gRPC / binary APIs
text/html                 ← web pages
text/plain                ← plain text
multipart/form-data       ← file uploads
application/octet-stream  ← raw binary
application/pdf           ← PDF documents
```

---

## Failure Scenarios

- **Missing `Vary: Accept-Encoding`** — CDN serves wrong encoding variant; client fails to decode
- **Server ignores `Accept` header** — always returns JSON even when client requests XML; breaks non-JSON consumers
- **Compressing already-compressed content** — wastes CPU, may slightly increase size (images, video)
- **Missing `charset=utf-8` in Content-Type** — some clients default to Latin-1; breaks non-ASCII characters

---

## Interview Perspective

- Content negotiation rarely the focus — but `Vary` header interaction with caching is commonly tested
- Know that Brotli is better than gzip and when to use each
- Explain why `Vary: Accept-Encoding` is required for correct CDN behavior

---

## Revision Summary

- Client advertises with `Accept`, `Accept-Encoding`, `Accept-Language`
- Server responds with `Content-Type`, `Content-Encoding`, `Content-Language`
- `q` values express preference weight; server picks best supported match
- Brotli (`br`) ~15-20% better compression than gzip for text; enable both on servers
- `Vary: Accept-Encoding` required to prevent CDN serving wrong compression variant
- Don't compress already-compressed binary content (images, video)

---

## Active Recall Questions

1. What does the `q` value in `Accept` headers represent?
2. Why is `Vary: Accept-Encoding` necessary when serving compressed responses through a CDN?
3. Why is Brotli preferred over gzip? When should you not compress?
4. What happens if a server ignores the `Accept` header?

---

## Related Concepts

- [[HTTP Caching Headers]]
- [[HTTP Version Evolution]]
- [[HTTPS Request Lifecycle]]
