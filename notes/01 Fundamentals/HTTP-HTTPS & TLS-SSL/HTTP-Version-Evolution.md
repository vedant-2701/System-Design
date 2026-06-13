# HTTP Version Evolution

## Tags
#networking #http #backend #performance

---

## Overview

- HTTP is an application-layer request-response protocol running over TCP (HTTP/1.1, HTTP/2) or QUIC/UDP (HTTP/3)
- Each version exists to solve performance problems introduced or left unsolved by the previous version
- Evolution is driven by three recurring problems: Head-of-Line blocking, connection overhead, and header redundancy

---

## HTTP/1.1 Problems

### Three Core Problems

**1. Head-of-Line (HOL) Blocking**
- Responses must be returned in request order on a single connection
- One slow request stalls all subsequent requests
- Browser workaround: open 6 parallel TCP connections per domain
- Each connection incurs its own TCP slow start + TLS handshake overhead

**2. Header Redundancy**
- Headers sent as plaintext on every request
- Cookie, User-Agent, Accept headers repeated across all 80+ requests on a page load
- No compression — pure bandwidth waste

**3. Poor TCP Utilization**
- 6-connection hack means 6 separate congestion windows, 6 slow starts
- TCP designed for long-lived connections but HTTP/1.1 fights against this

---

## HTTP/2 Solutions and New Problem

### What HTTP/2 Solved
- **Multiplexing** — multiple streams over a single TCP connection; streams are independent at HTTP layer
- **HPACK header compression** — shared header table; repeated headers sent as index references
- **Single TCP connection** — one congestion window, one slow start, proper TCP utilization
- **Stream prioritization** — client hints at which resources matter most

### What HTTP/2 Did NOT Solve — TCP-Level HOL Blocking
- All HTTP/2 streams share one TCP byte stream
- TCP guarantees ordered delivery — a single dropped packet stalls **all streams** until retransmission completes
- HTTP layer is unaware; it simply stops receiving data
- HTTP/2 can be **worse** than HTTP/1.1 under high packet loss — one connection vs six means one packet loss affects everything

---

## HTTP/3 — QUIC over UDP

### Why Not Fix TCP?
**Protocol ossification** — TCP is implemented in OS kernels; middleboxes (firewalls, NAT, routers) deeply assume TCP's wire format. Modifying TCP requires kernel updates across billions of devices and breaks infrastructure you don't control. UDP is treated as pass-through — no middlebox interference.

### What QUIC Provides
- **Per-stream loss recovery** — dropped packet affects only its stream, not others
- **0-RTT on repeat connections** — data sent in first packet to known servers
- **Combined transport + TLS handshake** — 1 RTT total (vs 2 RTTs for TCP + TLS 1.3)
- **Connection migration** — connections identified by connection ID, not IP:port tuple; survives WiFi → cellular switches without reconnecting

### QUIC Architecture
```
HTTP/3 (application)
    ↓
QUIC (reliability + streams + encryption — userspace)
    ↓
UDP (transport)
    ↓
IP (network)
```

---

## RTT Comparison

| Version | Protocol | Min RTTs (cold start) | HOL Blocking |
|---|---|---|---|
| HTTP/1.1 | TCP | 4 RTTs (DNS + TCP + TLS + HTTP) | HTTP layer |
| HTTP/2 | TCP | 4 RTTs | TCP layer |
| HTTP/3 | QUIC/UDP | 2 RTTs (DNS + QUIC) | None |
| HTTP/3 (repeat) | QUIC/UDP | 1 RTT (0-RTT data) | None |

---

## Tradeoffs

| Feature | HTTP/1.1 | HTTP/2 | HTTP/3 |
|---|---|---|---|
| HOL blocking | HTTP layer | TCP layer | None |
| Header compression | None | HPACK | QPACK |
| Multiplexing | No (6 conn hack) | Yes | Yes |
| Connection migration | No | No | Yes |
| Packet loss resilience | Poor (multiple conns) | Worse (single conn) | Best |
| Middlebox compatibility | Universal | Near-universal | Some issues |
| Server complexity | Low | Medium | High |

---

## Failure Scenarios

- **HTTP/2 under packet loss** — single dropped packet stalls all streams; can be worse than HTTP/1.1 in lossy networks (mobile, satellite)
- **QUIC middlebox blocking** — some enterprise firewalls block UDP 443; browsers fall back to HTTP/2 automatically
- **0-RTT replay attacks** — QUIC 0-RTT data can be replayed by attackers; must not be used for non-idempotent requests (POST, payments)

---

## Real-World Usage

- HTTP/2: near-universal in modern web servers (nginx, caddy, envoy)
- HTTP/3: deployed by Google, Cloudflare, Meta; ~30% of web traffic as of 2024
- Mobile APIs benefit most from HTTP/3 due to connection migration and packet loss resilience

---

## Interview Perspective

- "Why did HTTP/3 choose UDP over fixing TCP?" → ossification, not technical inferiority of TCP
- "Does HTTP/2 solve HOL blocking?" → partially; solves HTTP-layer HOL but introduces TCP-layer HOL
- "When would you choose HTTP/2 over HTTP/3?" → when UDP is blocked by infrastructure or server complexity isn't justified

---

## Revision Summary

- HTTP/1.1 problems: HOL blocking, header redundancy, poor TCP utilization
- HTTP/2 solves HOL at HTTP layer but introduces TCP-level HOL — worse under packet loss
- TCP can't be evolved due to ossification — middleboxes assume TCP wire format
- QUIC runs on UDP, implemented in userspace, ships in Chrome/servers like software updates
- HTTP/3 key wins: per-stream loss recovery, 0-RTT, connection migration
- 0-RTT is unsafe for non-idempotent operations — replay attack vector

---

## Active Recall Questions

1. What are the three core problems with HTTP/1.1?
2. HTTP/2 uses a single TCP connection. How does this make packet loss worse than HTTP/1.1?
3. Why was QUIC built on UDP instead of extending TCP?
4. What is connection migration and why does it matter for mobile clients?
5. When is 0-RTT dangerous and why?
6. What does HPACK do and how does it differ from regular compression?

---

## Related Concepts

- [[HTTPS Request Lifecycle]]
- [[TLS Handshake]]
- [[TCP Congestion Control]]
- [[HTTP Caching Headers]]
- [[CORS]]
