# Networking & TCP/IP — Index

## Tags
#networking #index #backend-engineering

---

## Note Map

```
Networking & TCP/IP
├── Foundations
│   ├── [[OSI Model & TCP/IP Stack]]
│   ├── [[IP Addressing, Subnets, CIDR]]
│   └── [[NAT and Routing]]
│
├── TCP Deep Dive
│   ├── [[TCP - Three-Way Handshake and Connection Lifecycle]]
│   ├── [[TCP Congestion Control]]
│   └── [[Port Exhaustion and TIME_WAIT]]
│
├── Protocols
│   ├── [[UDP vs TCP]]
│   ├── [[Ports and Sockets]]
│   └── [[ICMP, ping, traceroute]]
│
└── Production Engineering
    └── [[HTTP Connection Pooling]]
```

---

## Key Mental Models

**Layering:** each layer solves one problem; app code lives at L7 and is insulated from L1–L4 chaos.

**IP is best-effort; TCP adds reliability at endpoints** (end-to-end principle). Two handshakes before first HTTPS byte.

**TCP connection lifecycle:** SYN → SYN-ACK → ACK → data → FIN → ACK → FIN → ACK → TIME_WAIT.

**TIME_WAIT is correctness, not waste.** Cannot be removed without risking data corruption.

**AIMD:** grow linearly, back off multiplicatively on loss. Packet loss → throughput cliff on TCP.

**Connection pooling is the primary fix** for handshake cost, TIME_WAIT accumulation, and slow start repeated tax.

**UDP = timeliness over completeness.** Retransmitted data is useless when it arrives late.

---

## Critical Production Rules

1. `MaxIdleConnsPerHost` default of 10 is too small — right-size to concurrency
2. Always drain + close `resp.Body` — CLOSE_WAIT = not closed
3. `IdleConnTimeout` must be slightly below upstream keep-alive timeout
4. `context.WithTimeout` on every outbound call — prevents goroutine pile-up
5. Share one `http.Client` across the service — never create per-request
6. `EADDRNOTAVAIL` = port exhaustion = TIME_WAIT from undersized pool
7. `too many open files` = fd exhaustion = CLOSE_WAIT from unclosed bodies

---

## Cross-Topic Connections

| Topic | Connects To |
|-------|-------------|
| TIME_WAIT | Port exhaustion, NAT limits, keep-alive mismatch |
| Slow start | Connection pooling, HTTP/2 multiplexing, QUIC |
| Flow control (rwnd) | Backpressure, slow DB causing upstream slowdown |
| UDP | QUIC/HTTP3, real-time media, DNS |
| ICMP TTL | traceroute, routing loops, path MTU |
| 4-tuple uniqueness | One port → millions of connections, socket multiplexing |

---

## Prerequisite For

- [[HTTP Versions - HTTP1.1 vs HTTP2 vs HTTP3]]
- [[TLS Handshake]]
- [[DNS]]
- [[Load Balancing]]
- [[WebSockets]]
- [[gRPC]]
- [[Connection Pooling — DB]]
