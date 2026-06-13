# TCP Congestion Control

## Tags
#networking #tcp #performance #backend-engineering

---

## Overview

- TCP congestion control prevents **network collapse** — not just receiver buffer overflow (that's flow control)
- TCP infers congestion from **packet loss** — no explicit signal from the network needed
- Core algorithm: **AIMD** — Additive Increase, Multiplicative Decrease
- Congestion window (`cwnd`) controls how many bytes can be in-flight before an ACK is required
- Actual send rate = `min(cwnd, rwnd) / RTT`

---

## Why It Exists

- Routers have finite buffers; if senders transmit too fast, buffers fill, packets drop, everyone retransmits → more congestion → **congestion collapse**
- This actually happened on the early internet in **1986** — throughput dropped by a factor of 1000
- Van Jacobson's algorithms (slow start, congestion avoidance, fast retransmit) saved the internet
- Without congestion control, a single aggressive sender could collapse the network for everyone

---

## The Algorithms

### Slow Start

```
cwnd starts at initcwnd (default: 10 MSS on modern Linux, ~14KB)
For every ACK received: cwnd += 1 MSS
Effect: cwnd doubles every RTT (exponential growth)
Continues until cwnd reaches ssthresh (slow start threshold)
```

"Slow" only compared to sending at full speed immediately. Growth is exponential — fast in absolute terms.

### Congestion Avoidance

```
Once cwnd > ssthresh:
For every RTT: cwnd += 1 MSS (linear growth)
"Probe" for available bandwidth cautiously
```

### On Packet Loss Detection

```
ssthresh = cwnd / 2
cwnd     = 1 MSS          (timeout-based loss)
         = cwnd / 2        (fast retransmit / fast recovery — 3 dup ACKs)
Restart from slow start up to new ssthresh
```

### AIMD Sawtooth Pattern

```
cwnd
 ^
 |         /\
 |        /  \          /\
 |       /    \        /  \
 |      /  CA  \      /    \
 |  SS /        \  SS/      
 |____/          \/         
 └───────────────────────→ time
      loss      loss
```

Asymmetry is intentional: grow cautiously (additive), back off aggressively (multiplicative). Fairness property: competing flows converge to equal share of bandwidth.

---

## Key Parameters

| Parameter | Meaning | Linux Default |
|-----------|---------|---------------|
| `cwnd` | Congestion window (bytes in-flight) | Dynamic |
| `ssthresh` | Slow start threshold | Dynamic |
| `initcwnd` | Initial cwnd for new connections | 10 MSS (~14KB) |
| MSS | Maximum Segment Size | ~1460 bytes (Ethernet) |
| `2×MSL` | TIME_WAIT duration | 60 sec |

**Configure initcwnd:**
```bash
ip route change default via <gateway> initcwnd 10
```
Google research (2010): increasing initcwnd 3→10 reduced page load times ~10%. Now Linux default.

---

## Slow Start Penalty — Critical Production Insight

Every **new TCP connection** starts from `initcwnd`. It takes several RTTs to ramp to full throughput:

```
RTT 1: 10 MSS in-flight  (~14 KB)
RTT 2: 20 MSS            (~28 KB)
RTT 3: 40 MSS            (~56 KB)
...until ssthresh or loss
```

Short-lived connections never reach full bandwidth. This is why:
- **Connection reuse** (pooling) is critical — persistent connections have already ramped cwnd
- **HTTP/2 multiplexing** over one connection beats HTTP/1.1's 6 parallel connections (6 slow starts vs 1)
- **HTTP/3 (QUIC)** eliminates TCP handshake + slow start entirely for first request (0-RTT)

---

## Throughput Ceiling After Slow Start

Increasing `initcwnd` only moves the starting line. Real throughput ceiling after ramp-up is determined by:

```
throughput ≈ min(cwnd, rwnd) / RTT
```

Hard limits that initcwnd cannot fix:

**1. Client bandwidth:**
```
10 Mbps upload × 500MB = 400 seconds regardless of cwnd
```

**2. RTT:**
```
throughput = window_size / RTT
1MB window / 100ms RTT = 10 MB/s = 80 Mbps ceiling
```

**3. Packet loss (Mathis equation):**
```
throughput ≈ MSS / (RTT × √loss_rate)

At 1% loss, 100ms RTT:
≈ 1460 / (0.1 × 0.1) = 146,000 bytes/sec ≈ 1.1 Mbps

1% packet loss caps throughput at ~1 Mbps regardless of bandwidth or cwnd
```

TCP's AIMD is brutal on lossy networks — every loss halves cwnd. This is a primary reason QUIC exists.

---

## Tradeoffs

| Aspect | Benefit | Cost |
|--------|---------|------|
| Slow start | Avoids network overload on new connections | Low initial throughput |
| AIMD | Network stability, fairness between flows | Oscillating throughput |
| Loss as congestion signal | No explicit network support needed | Can't distinguish congestion loss from wireless loss |
| initcwnd=10 | Faster ramp for short connections | Slightly more aggressive on congested networks |

**Loss vs congestion**: TCP treats all loss as congestion. On lossy WiFi, random loss causes unnecessary cwnd reduction. BBR (Google's newer algorithm) uses RTT measurement instead of loss as the congestion signal — more appropriate for modern networks.

---

## Failure Scenarios

- **Bufferbloat**: large router buffers delay loss signal → cwnd grows too large → high latency before eventual loss → oscillation
- **Incast problem**: many servers respond simultaneously → brief congestion → multiple senders back off simultaneously → throughput collapses. Common in datacenter fan-in patterns
- **Lossy last-mile**: wireless loss triggers AIMD backoff even with no congestion → poor throughput on mobile
- **Long fat networks (LFN)**: high bandwidth × high RTT → requires large window → slow ramp-up time to fill the pipe

---

## Scaling Considerations

**Large file uploads (e.g. 500MB):**
- Slow start amortized over full transfer — not the main bottleneck
- Real bottleneck: client upload bandwidth, RTT, packet loss
- Fix: chunked parallel uploads (S3 multipart), edge ingestion to reduce RTT

**Many small API calls:**
- Each short-lived connection pays slow start repeatedly
- Fix: connection pooling, HTTP/2 multiplexing

**Cross-region traffic:**
- High RTT = throughput = window/RTT ceiling
- Fix: regional endpoints, connection kept alive across requests

---

## Interview Perspective

- "What is slow start and why does it exist?" → Probe network capacity safely; avoid congestion collapse
- "Why does increasing initcwnd help web performance?" → Short connections never ramp; higher start = faster first response
- "What does increasing initcwnd NOT fix?" → Client bandwidth, high RTT, packet loss
- "Why is TCP bad on lossy networks?" → Can't distinguish congestion loss from random loss; AIMD punishes both equally
- "Why does HTTP/2 help with slow start?" → One connection, one ramp-up, vs 6 connections × 6 ramp-ups for HTTP/1.1
- Common mistake: thinking initcwnd solves all throughput problems

---

## Revision Summary

- Congestion control prevents network collapse, not just receiver overflow (that's flow control)
- Slow start: exponential cwnd growth up to ssthresh
- Congestion avoidance: linear cwnd growth after ssthresh
- Loss → ssthresh = cwnd/2; cwnd reset → AIMD sawtooth
- initcwnd=10 MSS is Linux default; helps short connections; doesn't fix bandwidth/RTT/loss limits
- Throughput ceiling = `min(cwnd, rwnd) / RTT`; packet loss dominates via Mathis equation
- Connection reuse eliminates repeated slow start cost — primary reason for connection pooling
- HTTP/2: 1 slow start; HTTP/1.1: 6 parallel connections = 6 slow starts
- QUIC: eliminates TCP slow start entirely

---

## Active Recall Questions

1. What is the difference between flow control and congestion control?
2. Why is slow start named "slow" if it grows exponentially?
3. What happens to cwnd when 3 duplicate ACKs are received? How is this different from a timeout?
4. Your service uploads 500MB files and you increase initcwnd to 40. What problem does this not solve?
5. Why does 1% packet loss cap TCP throughput far below available bandwidth?
6. Why does HTTP/2 reduce the slow start tax compared to HTTP/1.1?
7. What is the AIMD sawtooth and why is the asymmetry (additive increase, multiplicative decrease) intentional?

---

## Related Concepts

- [[TCP - Three-Way Handshake and Connection Lifecycle]]
- [[Port Exhaustion and TIME_WAIT]]
- [[HTTP Connection Pooling]]
- [[HTTP Versions - HTTP1.1 vs HTTP2 vs HTTP3]]
- [[UDP vs TCP]]
