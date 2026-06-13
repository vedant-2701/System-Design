# UDP vs TCP

## Tags
#networking #tcp #udp #backend-engineering #tradeoffs

---

## Overview

- TCP: reliable, ordered, stateful, connection-oriented — high overhead, guaranteed delivery
- UDP: unreliable, unordered, stateless, connectionless — low overhead, no guarantees
- Choice depends on whether **timeliness** or **completeness** matters more for the workload
- UDP is not "worse" than TCP — it's a different contract for different use cases

---

## Comparison Table

| Feature | TCP | UDP |
|---------|-----|-----|
| Connection required | Yes (3-way handshake) | No |
| Reliable delivery | Yes (ACKs + retransmit) | No |
| Ordered delivery | Yes (sequence numbers) | No |
| Flow control | Yes (rwnd) | No |
| Congestion control | Yes (AIMD) | No |
| Header size | ~20 bytes | ~8 bytes |
| Latency | Higher (handshake + guarantees) | Lower (fire and forget) |
| State on sender | Yes (cwnd, rwnd, seq numbers) | None |
| Head-of-line blocking | Yes | No |

---

## When UDP Wins

**The core principle:** use UDP when timeliness matters more than completeness, or when application-level loss handling is cheaper than TCP's transport-level retry.

### DNS
- Query and response each fit in one packet
- TCP handshake would double latency for every lookup
- Packet loss → just resend the query (cheap retry)
- DNS over TCP used for large responses (zone transfers, DNSSEC)

### Real-Time Video / Voice (WebRTC, RTP)
- A retransmitted video frame arriving 200ms late is **useless** — the moment has passed
- Better to show a brief glitch and continue than stall the stream waiting for retransmit
- Application-level strategies: FEC, jitter buffer, adaptive bitrate

### Online Gaming
- Player position updates are time-sensitive
- A 100ms-old position update is stale — discard and use the next one
- Retransmitting old state wastes bandwidth and causes input lag

### QUIC (HTTP/3)
- Built on UDP; implements its own per-stream reliability
- TCP head-of-line blocking: one lost packet stalls **all streams** on the connection
- QUIC: one lost packet stalls **only that stream**; others continue
- Also eliminates TCP + TLS handshake overhead (0-RTT on reconnect)

---

## TCP on Lossy Networks — Why UDP Wins There

TCP's AIMD treats all loss as congestion — it can't distinguish:
- Congestion loss (router buffer full)
- Random loss (WiFi interference, mobile handoff)

On a lossy WiFi link with 2% packet loss:
```
TCP: every loss → cwnd halved → throughput severely degraded
UDP: 2% of frames dropped → brief glitch → stream continues
```

Switching a video stream from UDP to TCP on lossy network would cause:
- Constant cwnd backoff
- Retransmitted frames arriving too late to render
- Stream stalls, buffering, degraded experience

**Right approach for lossy UDP:**
- **FEC (Forward Error Correction)**: send redundant data; receiver reconstructs without retransmit
- **Adaptive bitrate**: detect loss → drop to lower quality stream rather than stall
- **Jitter buffer**: absorb variance at receiver
- **RTCP**: receiver reports loss metrics back to sender → sender adjusts encoding bitrate

---

## Tradeoffs Summary

| Workload | TCP | UDP |
|----------|-----|-----|
| File transfer | ✅ Correctness required | ❌ Loss = corruption |
| HTTP APIs | ✅ Standard | ❌ (QUIC brings UDP to HTTP) |
| DNS queries | ❌ Handshake overhead | ✅ One packet, retry is cheap |
| Live video | ❌ Retransmit arrives too late | ✅ Drop and continue |
| Online gaming | ❌ HOL blocking, retransmit lag | ✅ Low latency, drop stale state |
| Database queries | ✅ ACID requires reliability | ❌ |
| Large file upload | ✅ | ❌ |

---

## Failure Scenarios

**UDP:**
- Packet loss → data silently dropped; application must detect and handle
- No backpressure → fast sender can overwhelm slow receiver with no TCP-style slowdown
- Out-of-order delivery → application must reorder or tolerate it
- Amplification attacks: UDP used in DDoS amplification (DNS, NTP) — small request, large response sent to spoofed IP

**TCP on lossy networks:**
- Random loss triggers AIMD backoff even without congestion
- Head-of-line blocking stalls all multiplexed streams (solved by QUIC)

---

## Interview Perspective

- "Why use UDP for video streaming instead of TCP?" → Timeliness > completeness; retransmitted frames are useless; AIMD degrades TCP on loss
- "What is head-of-line blocking and how does QUIC fix it?" → TCP: one lost packet blocks all streams; QUIC: per-stream loss recovery on UDP
- "Why is DNS over UDP?" → Single packet fits query+response; handshake overhead would double latency; retry is trivial
- "If I switch a 2% loss video stream from UDP to TCP, what happens?" → Constant AIMD backoff, stream stalls, retransmitted frames arrive too late
- Common mistake: thinking UDP is simply "unreliable TCP" — it's a different contract designed for different workloads

---

## Revision Summary

- UDP = no connection, no guarantees, 8-byte header; TCP = stateful, reliable, ordered, 20-byte header
- Use UDP when timeliness > completeness: DNS, video, gaming, QUIC
- TCP punishes all loss equally (congestion or random) via AIMD — bad on lossy links
- QUIC (HTTP/3) = UDP + per-stream reliability + 0-RTT; fixes TCP HOL blocking
- For lossy UDP streams: FEC + adaptive bitrate + jitter buffer instead of switching to TCP
- UDP has no backpressure — application must implement flow control if needed

---

## Active Recall Questions

1. Why is DNS over UDP by default instead of TCP?
2. A live video stream drops 2% of packets. Should you switch from UDP to TCP? Justify your answer.
3. What is head-of-line blocking in TCP and how does QUIC solve it?
4. Why does TCP perform poorly on lossy WiFi networks specifically?
5. UDP has no congestion control. What risk does this introduce at the network level?
6. What application-level mechanisms compensate for UDP's lack of reliability in real-time video?

---

## Related Concepts

- [[TCP - Three-Way Handshake and Connection Lifecycle]]
- [[TCP Congestion Control]]
- [[HTTP Versions - HTTP1.1 vs HTTP2 vs HTTP3]]
- [[OSI Model & TCP/IP Stack]]
