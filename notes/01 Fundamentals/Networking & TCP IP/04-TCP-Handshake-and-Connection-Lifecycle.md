# TCP — Three-Way Handshake & Connection Lifecycle

## Tags
#networking #tcp #backend-engineering #reliability

---

## Overview

- TCP (Transmission Control Protocol) provides **reliable, ordered, error-checked delivery** over unreliable IP
- IP is best-effort — packets can be dropped, duplicated, reordered, corrupted
- TCP implements reliability at the transport layer so every application doesn't solve it independently
- TCP is **stateful** — both sides maintain connection state, sequence numbers, and buffers
- The **end-to-end principle**: IP stays simple; TCP handles reliability at the endpoints

---

## What TCP Guarantees

```
Reliable delivery    — lost packets are retransmitted
Ordered delivery     — bytes arrive in the order sent
Error detection      — checksum on every segment
Flow control         — don't overwhelm the receiver (rwnd)
Congestion control   — don't overwhelm the network (cwnd)
```

---

## Three-Way Handshake (Connection Establishment)

```
Client                          Server
  |                               |
  |——— SYN (seq=x) ————————————→ |   Client picks random ISN x
  |←—— SYN-ACK (seq=y, ack=x+1)—|   Server picks random ISN y; ACKs client
  |——— ACK (ack=y+1) ——————————→ |   Client ACKs server
  |                               |
  |════════ data flows ════════════|
```

**Cost: 1 RTT before any data flows.**

**Why random sequence numbers?**
Predictable ISNs allow attackers to inject packets into live connections by guessing the next sequence number.

**Why 3 steps and not 2?**
Two steps only confirm client→server direction. The third ACK confirms server→client direction works. Both directions must be verified before data flows.

---

## Sequence Numbers — Reliability Mechanism

Every byte has a sequence number. Receiver ACKs the next expected byte:

```
Sender:   [seq=1 "HEL"] [seq=4 "LO"]
Receiver: [ack=4]        [ack=6]
```

**On packet loss — Fast Retransmit:**
```
Sender:   [seq=1] [seq=4] [seq=7]
Receiver: gets 1, misses 4, gets 7
ACKs:     [ack=4] [ack=4] [ack=4]   ← 3 duplicate ACKs
Sender:   retransmits seq=4 immediately (no timeout wait)
```

3 duplicate ACKs = **fast retransmit** signal. Faster than waiting for retransmit timeout.

---

## Flow Control — Receive Window (rwnd)

Prevents overwhelming the **receiver's buffer**.

- Receiver advertises available buffer space in every ACK: `[ack=5001, rwnd=4096]`
- Sender must not have more than `rwnd` bytes in-flight unacknowledged
- When `rwnd=0`: sender stops, sends periodic **Window Probe** (1 byte) to detect buffer drain
- Receiver sends **Window Update** when buffer drains → sender resumes

**Production implication:** slow application reading from socket (e.g. blocked on DB write) → buffer fills → `rwnd=0` → sender backs off. TCP backpressure propagates upstream. A slow database causes upstream callers to slow down even if the network is healthy.

---

## Four-Way Termination & TIME_WAIT

Each direction closes independently:

```
Client                          Server
  |——— FIN ————————————————————→ |   Client done sending
  |←—— ACK ————————————————————  |   Server ACKs (server may still send)
  |←—— FIN ————————————————————  |   Server done sending
  |——— ACK ————————————————————→ |   Client ACKs
  |                               |
[Client enters TIME_WAIT — 2×MSL ≈ 60 sec on Linux]
```

The side that sends the **final ACK** enters TIME_WAIT.

### Why TIME_WAIT Must Exist (Two Reasons)

**Reason 1 — Final ACK may be lost:**
If the last ACK is lost, the server retransmits FIN. The client must still be alive to respond with ACK (not RST). TIME_WAIT keeps the socket alive to handle this retransmit.

**Reason 2 — Delayed packets from old connections:**
If a new connection immediately reuses the same 4-tuple `(src IP, src port, dst IP, dst port)`, a delayed packet from the old connection could corrupt the new connection's data. TIME_WAIT ensures all old packets have expired (2×MSL = 2×30s = 60s).

**TIME_WAIT cannot simply be disabled — it exists for correctness.**

---

## Full TCP State Machine

```
CLOSED
  ↓ SYN sent
SYN_SENT
  ↓ SYN-ACK received, ACK sent
ESTABLISHED
  ↓ FIN sent (active close)
FIN_WAIT_1
  ↓ ACK received
FIN_WAIT_2
  ↓ FIN received, ACK sent
TIME_WAIT (60 sec)
  ↓
CLOSED

--- passive close side ---
ESTABLISHED
  ↓ FIN received, ACK sent
CLOSE_WAIT        ← application must call close()
  ↓ FIN sent
LAST_ACK
  ↓ ACK received
CLOSED
```

---

## CLOSE_WAIT — Critical Production State

CLOSE_WAIT = remote sent FIN, your side sent ACK, **but your application has not called `close()`**.

**In Go — most common cause: not reading/closing response body:**

```go
resp, err := http.Get(url)
if err != nil { return err }
defer resp.Body.Close()              // REQUIRED
io.Copy(io.Discard, resp.Body)       // drain if body not needed
```

If `resp.Body.Close()` is omitted:
- `http.Transport` cannot reuse or cleanly close the connection
- Socket stays in CLOSE_WAIT
- Under load → thousands of CLOSE_WAIT sockets → file descriptor exhaustion → service dies

**Diagnosis:** `ss -tan | grep CLOSE_WAIT`

---

## Keep-Alive Mismatch

If upstream closes connections after a keep-alive timeout and your pool holds connections longer:

```
Your pool: connection idle, assumed reusable
Upstream:  already sent FIN (enters TIME_WAIT on their side)
Your side: tries to reuse → gets RST → broken pipe error
```

**Fix:** set `IdleConnTimeout` slightly below upstream's keep-alive timeout so **your side closes first**. This makes your side enter TIME_WAIT (predictable) and avoids broken pipe errors.

---

## Tradeoffs

| Feature | Benefit | Cost |
|---------|---------|------|
| Reliability (ACKs + retransmit) | No data loss | Latency on loss; retransmit overhead |
| Ordered delivery | Application simplicity | Head-of-line blocking |
| Flow control | Prevents receiver overflow | Backpressure can slow sender |
| Congestion control | Network stability | Throughput reduction on loss |
| Stateful connection | Reliable streams | Setup cost (handshake), teardown cost (TIME_WAIT) |

---

## Failure Scenarios

- **TIME_WAIT exhaustion**: too many short-lived outbound connections → port exhaustion → `EADDRNOTAVAIL`
- **CLOSE_WAIT accumulation**: not closing response bodies → fd exhaustion
- **SYN flood attack**: attacker sends many SYN packets, never completes handshake → server allocates state for each → memory exhaustion. Mitigated by **SYN cookies**
- **Head-of-line blocking**: single lost packet stalls entire TCP stream (all subsequent data buffered waiting for retransmit)
- **Broken pipe**: reusing a connection the upstream already closed

---

## Interview Perspective

- "Why is TCP's handshake 3-way and not 2-way?" → Must verify both directions
- "Why do sequence numbers start random?" → Prevent injection attacks
- "What is TIME_WAIT and why can't you just disable it?" → Correctness: handles lost final ACK + delayed packet collision
- "What is CLOSE_WAIT and what causes it in Go?" → App not calling `close()`; not draining response body
- "How does TCP backpressure work?" → `rwnd=0` stops sender; propagates upstream through connection pool blocking
- Common mistake: saying TIME_WAIT is a bug or unnecessary overhead

---

## Revision Summary

- TCP exists because IP is best-effort; TCP adds reliability, ordering, flow/congestion control
- 3-way handshake = 1 RTT cost before data; random ISNs prevent injection attacks
- Fast retransmit = 3 duplicate ACKs → immediate retransmit without timeout
- `rwnd=0` stops sender; slow app reading causes backpressure upstream
- TIME_WAIT lasts ~60s on Linux; entered by active-close side; required for correctness
- CLOSE_WAIT = remote closed, app hasn't called `close()` yet → fd exhaustion in Go if body not closed
- Keep-alive mismatch → broken pipe; fix by closing your side before upstream does

---

## Active Recall Questions

1. Why is the TCP handshake 3-way instead of 2-way?
2. What triggers fast retransmit? How is it different from timeout-based retransmit?
3. Which side enters TIME_WAIT — active close or passive close? Why does it matter for your service?
4. What is the difference between TIME_WAIT and CLOSE_WAIT?
5. Your Go service accumulates thousands of CLOSE_WAIT connections under load. What is the most likely bug?
6. How does a slow database on your server cause the upstream caller to slow down via TCP?
7. What are the two reasons TIME_WAIT must exist?

---

## Related Concepts

- [[Port Exhaustion and TIME_WAIT]]
- [[TCP Congestion Control]]
- [[HTTP Connection Pooling]]
- [[OSI Model & TCP/IP Stack]]
- [[UDP vs TCP]]
- [[TLS Handshake]]
