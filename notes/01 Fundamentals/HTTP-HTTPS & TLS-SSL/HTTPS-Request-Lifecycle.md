# HTTPS Request Lifecycle

## Tags
#networking #http #https #backend

---

## Overview

- Full lifecycle of a browser request from URL entry to response rendering
- Involves DNS, TCP, TLS, HTTP — each adding latency and guarantees
- Understanding the lifecycle is foundational for diagnosing latency, designing optimizations, and reasoning about where failures occur

---

## Complete Request Flow

```
1. URL Parsing
2. DNS Resolution
3. TCP Handshake
4. TLS Handshake
5. HTTP Request Construction + Transmission
6. Network Routing (hop-by-hop)
7. Server Processing
8. HTTP Response
9. Client Processing
```

---

## Step-by-Step Breakdown

### 1. URL Parsing
```
https://api.example.com/users/123
  ↓
scheme: https  → port 443, TLS required
host:   api.example.com  → needs DNS resolution
path:   /users/123  → resource identifier
```

### 2. DNS Resolution
Checked in order — stops at first hit:
```
Browser DNS cache
  → OS DNS cache (/etc/hosts)
    → Router cache
      → Recursive resolver (ISP / 8.8.8.8)
        → Root → TLD (.com) → Authoritative NS
          → Returns IP, cached with TTL
```
- Uses **UDP port 53** — separate network transaction before TCP
- Result cached per TTL; on repeat visits this RTT disappears entirely

### 3. TCP Handshake — 1 RTT
```
Client  →  SYN         →  Server
Client  ←  SYN-ACK     ←  Server
Client  →  ACK         →  Server
```
Connection established. No data yet.

### 4. TLS Handshake — 1 RTT (TLS 1.3)
```
Client  →  ClientHello (cipher suites, key share, client random)  →  Server
Client  ←  ServerHello + Certificate + CertificateVerify + Finished  ←  Server
Client  →  Finished  →  Server
        ← Encrypted HTTP begins →
```
- TLS 1.2 required 2 RTTs — TLS 1.3 reduced to 1
- Certificate verified during this step (see [[TLS Handshake & Certificate Chain]])

### 5. HTTP Request
```
GET /users/123 HTTP/1.1
Host: api.example.com
Accept: application/json
Accept-Encoding: gzip, br
Connection: keep-alive
```
- Passed to TLS for encryption → TCP for segmentation → IP for routing

### 6. Network Routing
- Hop-by-hop: each router reads destination IP, decrements TTL, rewraps in new Ethernet frame
- TTL = 0 → packet dropped, ICMP Time Exceeded sent back (basis of `traceroute`)
- DLL adds MAC addresses for each hop; MAC changes per hop, IP does not

### 7. Server Processing
Stack traversal bottom-up: Physical → Ethernet → IP → TCP → TLS decrypt → HTTP parse → handler → business logic → DB → response construction

### 8. Response Transmission
Same path in reverse, TLS encrypted

---

## RTT Budget (Cold Start, New Server)

```
DNS resolution:    1 RTT  (UDP)
TCP handshake:     1 RTT
TLS 1.3:           1 RTT
HTTP request:      1 RTT
─────────────────────────
Total:             4 RTTs minimum
```

---

## Latency Optimization Strategies

| Optimization | RTTs Saved | Mechanism |
|---|---|---|
| DNS caching (TTL tuned) | 1 RTT | Repeat visits skip DNS entirely |
| TCP keep-alive / connection reuse | 2 RTTs | Skip TCP + TLS on reused connection |
| TLS session resumption | Partial TLS RTT | Session ticket skips full TLS negotiation |
| HTTP/3 (QUIC) | 1 RTT | Combines TCP + TLS into 1 RTT |
| QUIC 0-RTT (repeat) | 2 RTTs | Data sent in first packet |
| CDN edge proximity | All RTTs reduced | Shorter physical path, fewer hops |

---

## Failure Scenarios

- **DNS failure** — request never reaches TCP; client sees "server not found"
- **TCP SYN timeout** — server unreachable or port blocked; client waits for timeout (default 75s on Linux)
- **TLS certificate failure** — expired cert, domain mismatch, untrusted CA → browser blocks request entirely
- **Keep-alive mismatch** — client holds connection open after server closed it → broken pipe / connection reset errors; `IdleConnTimeout` should be tuned slightly below server's keep-alive timeout
- **TTL = 0 drop** — misconfigured TTL causes packets to die before reaching destination

---

## Real-World Usage

- Chrome DevTools Network tab shows each phase: DNS, TCP connect, TLS, TTFB, content download
- TTFB (Time to First Byte) = DNS + TCP + TLS + server processing time — primary latency metric
- CDN reduces TTFB by terminating TCP + TLS at edge, serving from cache or proxying to origin on fast backbone

---

## Interview Perspective

- Draw the full lifecycle when asked "what happens when you type a URL"
- Distinguish which RTTs can be eliminated and how
- Know that DNS uses UDP; TCP and TLS are separate handshakes with separate costs
- Keep-alive eliminates TCP+TLS RTTs on reuse — critical for HTTP/1.1 performance

---

## Revision Summary

- DNS → TCP → TLS → HTTP: four distinct phases, each with RTT cost
- DNS uses UDP port 53 — separate transaction before TCP
- TLS 1.3 = 1 RTT; TLS 1.2 = 2 RTTs
- Cold start to new server = 4 RTTs minimum
- Connection reuse eliminates 2 RTTs (TCP + TLS)
- CDN reduces all RTTs by physical proximity, not by skipping steps
- Keep-alive mismatch causes broken pipe — tune `IdleConnTimeout` below server keep-alive

---

## Active Recall Questions

1. In what order do DNS, TCP, and TLS occur? Can any be parallelized?
2. How many RTTs does a cold HTTPS request take with TLS 1.3? With TLS 1.2?
3. What does TTFB measure and what contributes to it?
4. What happens at the network layer on each router hop? What changes, what stays constant?
5. Name three optimizations that reduce RTT count without changing backend code
6. Why does a keep-alive mismatch cause broken pipe errors?

---

## Related Concepts

- [[HTTP Version Evolution]]
- [[TLS Handshake & Certificate Chain]]
- [[DNS Resolution]]
- [[TCP Three-Way Handshake]]
- [[HTTP Caching Headers]]
