# HTTP Connection Pooling

## Tags
#networking #http #go #backend-engineering #production #performance

---

## Overview

- Connection pooling reuses TCP (and TLS) connections across multiple HTTP requests
- Eliminates repeated handshake cost and TIME_WAIT accumulation
- Go's `http.Transport` is the connection pool — it is **not** per-`http.Client` by default; share one transport
- Default settings in Go are production-unsafe at any meaningful throughput

---

## Why It Exists

Each new TCP connection costs:
```
TCP handshake:   1 RTT    (~1–200ms depending on distance)
TLS handshake:   1–2 RTT  (HTTPS only)
Slow start:      several RTTs to ramp cwnd
─────────────────────────────────────────
Total overhead:  2–3 RTT before first byte of HTTP data
```

At 500 new connections/sec to a cross-region endpoint (50ms RTT):
```
500 × 150ms handshake overhead = 75 seconds of latency wasted per second
```

Connection pooling pays this cost once per connection lifetime, then reuses.

---

## Go http.Transport — Key Parameters

```go
transport := &http.Transport{
    // Pool sizing
    MaxIdleConns:          1000,             // total idle connections across all hosts
    MaxIdleConnsPerHost:   100,              // idle connections per host (DEFAULT: 10 — TOO SMALL)
    MaxConnsPerHost:       200,              // hard ceiling: blocks goroutine if hit (DEFAULT: 0 = unlimited)

    // Timeouts
    IdleConnTimeout:       85 * time.Second, // close idle conn after this; set below upstream keep-alive
    TLSHandshakeTimeout:   10 * time.Second,
    ResponseHeaderTimeout: 30 * time.Second,
    ExpectContinueTimeout: 1  * time.Second,

    // Dialer
    DialContext: (&net.Dialer{
        Timeout:   5  * time.Second,  // TCP connect timeout
        KeepAlive: 30 * time.Second,  // TCP keep-alive probes
    }).DialContext,
}

client := &http.Client{
    Transport: transport,
    Timeout:   30 * time.Second,  // end-to-end request timeout
}
```

**Always share one `http.Client` (and its Transport) across your service.** Creating a new `http.Client` per request creates a new pool per request — defeating pooling entirely.

---

## What Happens Without Correct Sizing

```
Traffic: 500 req/sec to same host
MaxIdleConnsPerHost: 10 (default)

Flow:
  10 requests → reuse pooled connections (no handshake)
  490 requests → create NEW connections → pay handshake cost
  490 connections complete → pool has room for 10 → 480 closed immediately
  480 connections → TIME_WAIT for 60 seconds

After 60 sec at 500 req/sec:
  480 × 60 = 28,800 ports in TIME_WAIT
  Ephemeral range exhausted → EADDRNOTAVAIL → service throws errors
```

---

## MaxConnsPerHost Ceiling Behavior

When `MaxConnsPerHost` is hit:
```
All 200 connections in use
201st request goroutine → BLOCKS (does not create new connection)
Goroutine waits for a connection to become available
```

If upstream is slow (high latency), connections are held longer → pool saturates faster → goroutines pile up:
```
200 connections × 500ms avg request time = 400 req/sec max throughput
At 500 req/sec: 100 req/sec queued → goroutine stack memory accumulates
Each goroutine: ~2–8 KB stack → 10k goroutines = 20–80 MB
```

**Fix:** `context.WithTimeout` on every outbound call to bound goroutine wait time:
```go
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()
req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
resp, err := client.Do(req)
```

---

## Keep-Alive Mismatch — Silent Failure

```
Upstream keep-alive timeout: 90 sec
Your IdleConnTimeout:        120 sec  ← holds connections longer than upstream

After 91 sec idle:
  Upstream closes connection (sends FIN → TIME_WAIT on their side)
  Your pool still marks connection as reusable
  Next request reuses it → gets TCP RST or EOF
  → request fails with "connection reset by peer" or "EOF"
  Go retries once on idempotent methods but adds latency spike
```

**Rule:** `IdleConnTimeout` = upstream keep-alive timeout − 5 seconds

---

## Response Body — Critical Correctness Rule

`http.Transport` cannot return a connection to the pool unless:
1. The response body is **fully read**
2. The response body is **closed**

```go
resp, err := client.Do(req)
if err != nil {
    return err
}
defer resp.Body.Close()

// If you don't need the body, still drain it:
io.Copy(io.Discard, resp.Body)
```

**Not draining/closing body:**
- Connection cannot be reused → new connection created for next request
- Connection enters CLOSE_WAIT (remote sent FIN, your side hasn't called close())
- Thousands of CLOSE_WAIT → fd exhaustion → "too many open files"

---

## Cost Summary Per Scenario

| Scenario | Handshake cost | TIME_WAIT risk | Goroutine risk |
|----------|---------------|----------------|----------------|
| New conn per request | Every request | High | Low |
| Pool too small (default) | Most requests | Medium-High | Low |
| Pool right-sized | First conn only | Low | Medium if upstream slow |
| Pool right-sized + timeouts | First conn only | Low | Low |

---

## Failure Scenarios

- **EADDRNOTAVAIL**: port exhaustion from TIME_WAIT; pool too small
- **"too many open files"**: fd exhaustion from CLOSE_WAIT; not closing response bodies
- **"connection reset by peer"**: keep-alive mismatch; upstream closed stale connection
- **Goroutine leak / OOM**: MaxConnsPerHost hit + slow upstream + no context timeout
- **Latency spikes**: every request pays handshake (wrong: pool not shared across goroutines / new client per request)

---

## Diagnosis Commands

```bash
# TIME_WAIT accumulation
ss -tan | grep TIME_WAIT | wc -l

# CLOSE_WAIT accumulation (response body not closed)
ss -tan | grep CLOSE_WAIT | wc -l

# File descriptor usage
ls -la /proc/<pid>/fd | wc -l
cat /proc/sys/fs/file-max         # system-wide fd limit

# Active connections per remote host
ss -tan | awk '{print $5}' | cut -d: -f1 | sort | uniq -c | sort -rn
```

---

## Interview Perspective

- "Why is Go's default MaxIdleConnsPerHost=10 a problem?" → Most services have far more than 10 concurrent requests; excess connections created and discarded → TIME_WAIT
- "What happens when MaxConnsPerHost is hit?" → Goroutine blocks; does not create new connection
- "Why must you close the response body even if you don't need it?" → Pool cannot reclaim connection; CLOSE_WAIT accumulation
- "What is a keep-alive mismatch and how do you fix it?" → Upstream closes connection your pool thinks is alive; fix with IdleConnTimeout < upstream keep-alive
- Common mistake: creating new http.Client per request

---

## Revision Summary

- Pooling eliminates per-request TCP + TLS handshake cost and TIME_WAIT accumulation
- Default `MaxIdleConnsPerHost=10` is too small for any production service
- `MaxConnsPerHost` ceiling → goroutine blocks (not errors); must combine with `context.WithTimeout`
- Always drain + close response body; CLOSE_WAIT = not closed
- `IdleConnTimeout` must be slightly below upstream keep-alive timeout
- Share one `http.Client` (and Transport) across entire service

---

## Active Recall Questions

1. Your Go service creates a new `http.Client` on every request. What are the consequences?
2. `MaxIdleConnsPerHost=10`, traffic is 300 req/sec. After 60 seconds, how many ports are in TIME_WAIT?
3. What happens to the goroutine when `MaxConnsPerHost` ceiling is hit?
4. Why does not closing the response body cause CLOSE_WAIT accumulation?
5. Your service occasionally gets "connection reset by peer" errors. What is the most likely cause?
6. How do you set timeouts correctly to prevent goroutine pile-up from a slow upstream?

---

## Related Concepts

- [[Port Exhaustion and TIME_WAIT]]
- [[TCP - Three-Way Handshake and Connection Lifecycle]]
- [[TCP Congestion Control]]
- [[NAT and Routing]]
- [[HTTP Versions - HTTP1.1 vs HTTP2 vs HTTP3]]
