# Questions

> 1. A TCP connection is established. The receiver's buffer is full. What happens to the sender?
> 2. You see thousands of CLOSE_WAIT connections on your server. What's the bug?
> 3. Why does HTTP/2 help with slow start compared to HTTP/1.1?
> 4. A UDP-based video stream drops 2% of packets. Should you switch to TCP? Why or why not?
> 5. Your traceroute shows * * * at hop 3 but the destination is reachable. What does this mean?

---

# Answers

## 1. Receiver Buffer Full — Sender Is Blocked

Correct, but let's make the mechanism precise. This is called **flow control** and TCP handles it via the **receive window (rwnd)**.

The receiver advertises its available buffer space in every ACK:

```
ACK segment: [ack=5001, rwnd=4096]
             "I've received up to 5000, I have 4096 bytes of buffer left"
```

When the receiver's buffer fills completely:
```
rwnd = 0
```

The sender must stop transmitting. But it doesn't just block forever — it periodically sends **Window Probe** packets (1 byte) to check if rwnd has opened up. When the receiver drains its buffer and rwnd becomes non-zero, it sends a **Window Update** and the sender resumes.

```
Receiver buffer full → rwnd=0 advertised → sender stops
Receiver application reads data → buffer drains → rwnd>0
Receiver sends Window Update → sender resumes
```

**Production implication:** if your application is slow at reading from the socket — maybe a slow database write is blocking your request handler — TCP backpressure propagates all the way back to the sender. The sender's throughput drops. This is how a slow database causes upstream callers to slow down even though the network is fine. Backpressure is a feature, not a bug — it prevents buffer overflow at the cost of reduced throughput.

---

## 2. CLOSE_WAIT Bug — More Precise Than "Forgot to Call close()"

Your answer is directionally correct but too vague. Let's be precise.

CLOSE_WAIT means: the remote side sent FIN (they're done sending), your side sent ACK, but **your application has not yet called close() on the socket.**

In Go specifically, the most common cause is **not fully reading the HTTP response body:**

```go
resp, err := http.Get(url)
if err != nil {
    return err
}
// BUG: forgot resp.Body.Close()
// or returned early without reading body fully
```

Go's `http.Transport` cannot reuse or cleanly close a connection unless the response body is fully consumed and closed. If you don't, the connection sits in CLOSE_WAIT. Under load, you accumulate thousands of them, exhaust file descriptors, and the service dies with "too many open files."

The correct pattern always:
```go
resp, err := http.Get(url)
if err != nil {
    return err
}
defer resp.Body.Close()
io.Copy(io.Discard, resp.Body) // drain if you don't need the body
```

Other causes: goroutine leaks where the goroutine holding the connection is stuck and never closes it. The socket stays open in CLOSE_WAIT until the process dies.

---

## 3. HTTP/2 and Slow Start — Your Answer Has a Subtle Error

You said HTTP/1.1 needs 6 separate TCP connections for 6 requests. That's partially true — browsers open multiple parallel TCP connections (typically 6 per host) to work around HTTP/1.1's limitation. But the actual limitation is **head-of-line blocking**, not just the number of connections.

Let me make the comparison precise:

**HTTP/1.1:**
```
Connection 1: [request 1] → [response 1] → [request 2] → [response 2]
Connection 2: [request 3] → [response 3]
...
```
Each connection is sequential — you must wait for response N before sending request N+1 on that connection (without pipelining). Browsers compensate by opening 6 parallel connections, each paying its own TCP handshake and slow start cost.

**HTTP/2:**
```
One TCP connection
Stream 1: [request 1 frames] ←→ [response 1 frames]
Stream 2: [request 2 frames] ←→ [response 2 frames]
Stream 3: [request 3 frames] ←→ [response 3 frames]
All interleaved on the same connection
```

One connection, fully warmed-up cwnd, one slow start paid once. All streams share that warmed connection.

**The real slow start benefit:**
```
HTTP/1.1: 6 connections × slow start each = 6× slow start tax
HTTP/2:   1 connection × slow start once  = 1× slow start tax
```

But HTTP/2 has its own head-of-line blocking problem at the TCP layer — if one TCP packet is lost, all streams stall waiting for retransmission. This is why HTTP/3 (QUIC over UDP) exists — per-stream loss recovery means one lost packet only stalls that stream, not all of them.

---

## 4. UDP Video Stream With 2% Packet Loss — Correct Reasoning

Your answer is right. Don't switch to TCP. But let me add the precise reasoning you'd give in an interview.

2% packet loss on TCP via AIMD:
```
Every loss event → cwnd halved
At 2% loss, TCP is constantly backing off
Throughput severely degraded
Retransmitted frames arrive too late to be useful
```

The right approach for video:
```
Stay on UDP
Implement FEC (Forward Error Correction) — send redundant data
  so receiver can reconstruct lost packets without retransmission
Use adaptive bitrate — if loss is high, drop to lower quality stream
  rather than stalling
Implement jitter buffer — absorb network variance at receiver
Report loss metrics back to sender via RTCP
  sender adjusts encoding bitrate accordingly
```

This is exactly what WebRTC does — UDP underneath with application-layer loss handling tailored for real-time media.

**The principle:** TCP's retransmission is correct for data where accuracy matters more than timeliness. For real-time media, timeliness matters more than completeness. The application layer must decide what "good enough" means — TCP's blanket retransmission policy is too blunt.

---

## 5. `* * *` in traceroute — You Got This One Verbatim

Correct. Some routers are configured to not generate ICMP Time Exceeded responses — either for security reasons (not revealing internal topology) or because ICMP response generation is low priority. Real traffic flows through fine. The `* * *` just means that hop is silent, not broken.

The practical debugging implication: if you see `* * *` at hop 3 and then latency appears at hop 5, you know the problem is somewhere between 3 and 5 but can't pinpoint exactly where. You work with what you have.

---