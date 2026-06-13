# Port Exhaustion & TIME_WAIT

## Tags
#networking #tcp #production #backend-engineering #performance

---

## Overview

- TIME_WAIT is a TCP state entered by the **active-close side** after sending the final ACK
- Duration: `2 × MSL` ≈ 60 seconds on Linux
- Ports in TIME_WAIT cannot be reused for new connections to the same destination 4-tuple
- At high connection rates, TIME_WAIT accumulation exhausts the ephemeral port range → service failure
- The fix is **connection pooling** — reuse connections instead of creating new ones

---

## Ephemeral Port Range

```
Total ports:        65535  (16-bit)
Well-known:         0–1023 (reserved: HTTP=80, HTTPS=443, SSH=22)
Ephemeral (Linux):  32768–60999 (default)
Available:          ~28,000 on default; up to ~64k if range extended
```

Check and tune:
```bash
cat /proc/sys/net/ipv4/ip_local_port_range
# e.g. 32768 60999

# Extend range:
echo "1024 65535" > /proc/sys/net/ipv4/ip_local_port_range
```

The limit applies **per `(source IP, destination IP, destination port)` tuple** — not globally per source IP.

---

## TIME_WAIT Accumulation Math

```
Connection rate:   R  connections/sec to same destination
TIME_WAIT duration: T  seconds (60s on Linux)

Ports in TIME_WAIT at steady state = R × T

Example:
  800 conn/sec × 60 sec = 48,000 ports in TIME_WAIT
  Ephemeral range ≈ 28,000 (default) or 64,000 (extended)
  → Exhaustion in ~35 seconds at default range
  → New connections fail with EADDRNOTAVAIL
```

**Why TIME_WAIT can't be removed:** see [[TCP - Three-Way Handshake and Connection Lifecycle]]

---

## Connection Pool — The Primary Fix

Without pooling:
```
Every request → new TCP connection → handshake → data → close → TIME_WAIT
Pool overflow → connections discarded → TIME_WAIT accumulates
```

With correct pooling:
```
Connection created once → reused for N requests → TIME_WAIT paid once per connection lifetime
Pool of 100 connections handles thousands of requests/sec
```

### Go http.Transport Configuration

```go
transport := &http.Transport{
    MaxIdleConnsPerHost:   100,              // idle connections kept in pool per host
    MaxConnsPerHost:       200,              // hard ceiling on total connections per host
    IdleConnTimeout:       85 * time.Second, // slightly below upstream keep-alive timeout
    TLSHandshakeTimeout:   10 * time.Second,
    ResponseHeaderTimeout: 30 * time.Second,
    DialContext: (&net.Dialer{
        Timeout:   5 * time.Second,
        KeepAlive: 30 * time.Second,
    }).DialContext,
}
client := &http.Client{
    Transport: transport,
    Timeout:   30 * time.Second,
}
```

**Default `MaxIdleConnsPerHost` is 10** — dangerously small for high-throughput services.

### What Happens When Pool Is Exhausted (`MaxConnsPerHost` hit)

The goroutine making the request **blocks** until a connection becomes available. It does NOT create a new connection — that would defeat the ceiling.

```
MaxConnsPerHost=200, all 200 in use
→ 201st request goroutine blocks
→ goroutines accumulate (~2–8KB stack each)
→ slow upstream → goroutines pile up → memory pressure
→ Fix: context.WithTimeout on every outbound call
```

---

## Keep-Alive Mismatch — Hidden Production Trap

```
Upstream keep-alive timeout: 90 sec
Your IdleConnTimeout:        120 sec  ← WRONG: you hold longer than upstream

Scenario:
  Connection idle for 91 seconds
  Upstream silently closes (enters TIME_WAIT on their side)
  Your pool still holds the connection as "idle, reusable"
  Next request reuses it → gets RST → broken pipe error
  Go retries once, but latency spike occurs
```

**Fix:** set `IdleConnTimeout` to slightly less than upstream's keep-alive timeout:
```
Upstream keep-alive: 90 sec
Your IdleConnTimeout: 85 sec  ← you close first, cleanly
```

Your side enters TIME_WAIT — predictable and on your terms. No broken pipe.

---

## Which Side Enters TIME_WAIT Matters

| Scenario | TIME_WAIT side | Impact on your service |
|----------|---------------|----------------------|
| Your service closes first | Your service | Your ports consumed; manage with pooling |
| Upstream closes first | Upstream | Their port problem; but you get broken pipe if pool reuses stale connection |
| HTTP/1.1 default | Server usually closes | Browser/client rarely has port exhaustion |
| Microservice-to-microservice | Whichever closes first | Both services must coordinate keep-alive settings |

In a microservice calling another microservice at high rate: your service is the client, so your side typically initiates close → your TIME_WAIT. Size pool correctly and set `IdleConnTimeout` appropriately.

---

## Additional Mitigations

| Mitigation | What it does | When to use |
|------------|-------------|-------------|
| `SO_REUSEADDR` | Allows bind to port in TIME_WAIT | Server port rebind after restart |
| `tcp_tw_reuse` | Reuse TIME_WAIT sockets for new outbound connections | High-throughput outbound; requires timestamps |
| Extend port range | More ephemeral ports available | When pooling insufficient |
| Multiple source IPs | Multiply port budget | Extreme throughput; rare |
| Connection pooling | Eliminate TIME_WAIT accumulation | Always — primary fix |

```bash
# Enable tcp_tw_reuse (safe for outbound connections)
echo 1 > /proc/sys/net/ipv4/tcp_tw_reuse
```

---

## Failure Scenarios

- **`EADDRNOTAVAIL`**: port range exhausted; new connections fail immediately; service appears healthy otherwise
- **Broken pipe / RST**: stale pooled connection reused after upstream closed → request fails mid-flight
- **Goroutine pile-up**: `MaxConnsPerHost` hit + slow upstream → blocked goroutines accumulate → OOM
- **fd exhaustion**: each socket is a file descriptor; `ulimit -n` limit hit before port limit on some systems → `too many open files`

**Diagnosis commands:**
```bash
ss -tan | grep TIME_WAIT | wc -l     # count TIME_WAIT sockets
ss -tan | grep CLOSE_WAIT | wc -l    # count CLOSE_WAIT sockets
ss -tan state time-wait              # list them
cat /proc/sys/net/ipv4/ip_local_port_range
```

---

## Interview Perspective

- "What is TIME_WAIT and when does a socket enter it?" → Active-close side after final ACK; lasts 2×MSL
- "Why can't you just set TIME_WAIT duration to 0?" → Correctness: delayed packet collision + lost final ACK
- "Your service throws EADDRNOTAVAIL under load. Root cause?" → Port exhaustion from TIME_WAIT; pool size too small
- "Go's default MaxIdleConnsPerHost is 10. Why is this a problem?" → 10 connections handles ~10 concurrent requests; anything beyond creates new connections → TIME_WAIT accumulation
- "What happens to the 501st request when MaxConnsPerHost=500?" → Goroutine blocks until connection freed; use context timeout
- Common mistake: thinking TIME_WAIT is only on the server side

---

## Revision Summary

- TIME_WAIT entered by active-close side; lasts ~60s; required for correctness
- Port exhaustion = `R × T` ports consumed at steady state; fails with `EADDRNOTAVAIL`
- Go default `MaxIdleConnsPerHost=10` is dangerously small; must right-size to concurrency
- `MaxConnsPerHost` ceiling → blocks goroutine (does not create new connection)
- `IdleConnTimeout` must be slightly below upstream keep-alive to avoid broken pipe
- Diagnosis: `ss -tan | grep TIME_WAIT`; tune via `ip_local_port_range`, `tcp_tw_reuse`
- Primary fix is always connection pooling — mitigations are secondary

---

## Active Recall Questions

1. Your Go service makes 600 new connections/sec. After 60 seconds, how many ports are in TIME_WAIT? Will you hit exhaustion with default Linux port range?
2. What happens to a goroutine that requests a connection when `MaxConnsPerHost` is already at the ceiling?
3. Why should `IdleConnTimeout` be set slightly below the upstream's keep-alive timeout?
4. What is the difference between TIME_WAIT and CLOSE_WAIT from a production debugging perspective?
5. Why does `tcp_tw_reuse` require TCP timestamps to be safe?
6. Your `ss -tan` shows 50,000 TIME_WAIT sockets but zero CLOSE_WAIT. What does this tell you about the bug profile?

---

## Related Concepts

- [[TCP - Three-Way Handshake and Connection Lifecycle]]
- [[TCP Congestion Control]]
- [[NAT and Routing]]
- [[HTTP Connection Pooling]]
- [[HTTP Versions - HTTP1.1 vs HTTP2 vs HTTP3]]
