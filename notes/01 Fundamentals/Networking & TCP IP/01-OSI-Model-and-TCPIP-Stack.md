# OSI Model & TCP/IP Stack

## Tags
#networking #foundations #backend-engineering

---

## Overview

- OSI is a 7-layer conceptual model for network communication
- TCP/IP is the practical 4-layer implementation used on the internet
- Layering enforces separation of concerns — each layer only talks to the layer directly above and below
- Enables swapping implementations at one layer without affecting others (e.g. WiFi vs Ethernet with no app code change)

---

## Why It Exists

- Networks involve many distinct problems: physical transmission, addressing, routing, reliability, application semantics
- Without layering, changing one component (e.g. physical medium) would break everything
- The **end-to-end principle**: keep the network simple; push complexity to endpoints (TCP lives at endpoints, not routers)

---

## OSI Layers

```
7 — Application    HTTP, DNS, SMTP, gRPC
6 — Presentation   Encoding, encryption, compression (TLS lives here conceptually)
5 — Session        Session management, connection state
4 — Transport      TCP, UDP — reliability, ports, segmentation
3 — Network        IP — addressing, routing across networks
2 — Data Link      MAC addresses, frames, local delivery (Ethernet, WiFi)
1 — Physical       Raw bits — voltages, signals, cables
```

---

## TCP/IP Model (4 Layers)

```
OSI 7,6,5  →  Application   (HTTP, DNS, TLS...)
OSI 4      →  Transport     (TCP, UDP)
OSI 3      →  Internet      (IP)
OSI 2,1    →  Network Access (Ethernet, WiFi)
```

Layers 5,6,7 collapsed: session and presentation are almost always handled inside application protocols (HTTP manages sessions, TLS handles encryption).

---

## Encapsulation Flow

Data travels **DOWN** the stack when sending, **UP** when receiving. Each layer wraps the payload with its own header:

```
[App data]
→ Transport: [TCP header | App data]
→ Network:   [IP header | TCP header | App data]
→ Data Link: [Frame | IP header | TCP header | App data | Frame trailer]
→ Physical:  raw bits
```

Receiver strips each header and passes the payload up. Application only ever sees raw app data.

---

## Where Application Code Operates

- Your Go/Java service lives at **Layer 7 (Application)**
- The code never deals with: packet routing, raw bits, frame delivery, retransmission, IP TTL
- The OS + kernel handles Layers 1–4 transparently

**Practical implication:** an HTTPS call from your service triggers:
1. TCP 3-way handshake (Layer 4)
2. TLS handshake (Layer 6/Application)
3. Only then: HTTP request (Layer 7)

Two handshakes = minimum 2 RTTs before first byte of real data. This cost is paid per new connection.

---

## Tradeoffs

| Aspect | Benefit | Cost |
|--------|---------|------|
| Layering | Modularity, interoperability | Overhead per layer (headers) |
| End-to-end principle | Simple network, smart endpoints | Endpoints must handle reliability |
| TCP/IP collapse of OSI | Practical simplicity | Less clear separation at L5/L6/L7 |

---

## Failure Scenarios

- **Layer mismatch debugging**: a network issue at L3 (routing) looks like an app-level failure — traceroute isolates the layer
- **MTU mismatch**: frames too large for a link get fragmented or dropped at L3 — causes mysterious packet loss
- **TLS misconfiguration at L6**: app layer receives garbage; looks like L7 bug

---

## Interview Perspective

- "Where does TCP live?" → Layer 4, Transport
- "Why does layering exist?" → Separation of concerns, modularity, interoperability
- "Why doesn't application code deal with packet loss?" → TCP at L4 handles it; end-to-end principle
- Common mistake: confusing OSI and TCP/IP layer counts (7 vs 4)

---

## Revision Summary

- OSI = 7 layers; TCP/IP = 4 layers (L5+L6+L7 → Application, L1+L2 → Network Access)
- Each layer encapsulates the layer above with its own header
- App code lives at L7; OS/kernel owns L1–L4
- Two handshakes (TCP + TLS) before first HTTPS byte
- End-to-end principle: network stays dumb, endpoints handle reliability
- Layering allows medium changes (WiFi ↔ Ethernet) without app code changes

---

## Active Recall Questions

1. Why does TCP/IP collapse OSI layers 5, 6, 7 into one?
2. When your Go service makes an HTTPS call, how many handshakes happen before HTTP data flows? Which layers are responsible?
3. What is encapsulation and which direction does it happen?
4. Why can you switch from WiFi to Ethernet without changing application code?
5. What is the end-to-end principle and why does it justify keeping IP simple?

---

## Related Concepts

- [[TCP - Three-Way Handshake and Connection Lifecycle]]
- [[TLS Handshake]]
- [[IP Addressing, Subnets, CIDR]]
- [[UDP vs TCP]]
- [[HTTP Versions - HTTP1.1 vs HTTP2 vs HTTP3]]
