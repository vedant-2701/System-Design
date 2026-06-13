# Linux Networking Tools

## Tags
#linux #networking #backend-engineering #operations #foundations

---

## Overview

- `ss` and `netstat` inspect socket state — what ports are open, what connections exist
- `tcpdump` captures actual packets — what is being sent/received at the wire level
- Together they answer: "Is my service listening?", "Who is connected?", "Are packets actually flowing?"
- `netstat` is deprecated; `ss` is its modern replacement — faster, more info, same concepts

---

## ss — Socket Statistics

### Essential flags

| Flag | Meaning |
|------|---------|
| `-t` | TCP only |
| `-u` | UDP only |
| `-l` | Listening sockets only |
| `-n` | Numeric — don't resolve hostnames (faster) |
| `-p` | Show process name and PID |
| `-a` | All sockets (listening + established) |

### Key commands

```bash
# What ports is this machine listening on?
ss -tlnp

# All established TCP connections
ss -tnp state established

# All connections to postgres
ss -tnp dst :5432

# All connections from a specific IP
ss -tnp src 192.168.1.100

# Who owns port 8080?
ss -tlnp sport = :8080

# Connection state distribution
ss -tan | awk '{print $1}' | sort | uniq -c | sort -rn
```

### Output interpretation

```
State    Recv-Q  Send-Q  Local Address:Port  Peer Address:Port  Process
LISTEN   0       128     0.0.0.0:8080        0.0.0.0:*          users:(("java",pid=1234))
```

| Column | Meaning |
|--------|---------|
| `Recv-Q` | Data received, not yet processed by app |
| `Send-Q` | Data waiting to be sent |
| `0.0.0.0:8080` | Listening on all interfaces |
| `127.0.0.1:8080` | Listening on loopback only — external connections blocked |

**High `Recv-Q` on listening socket** = app not calling `accept()` fast enough → accept queue filling → connections being dropped. Related to `somaxconn` limit.

---

## TCP Connection States — Production Signals

```bash
ss -tan | awk '{print $1}' | sort | uniq -c | sort -rn
# Example output:
# 142 ESTABLISHED
#  23 TIME_WAIT
#   8 CLOSE_WAIT
#   1 LISTEN
```

| State | Normal? | Problem signal |
|-------|---------|----------------|
| `ESTABLISHED` | Yes | Very high count = connection leak or traffic spike |
| `TIME_WAIT` | Yes in small numbers | Large count = many short-lived connections, use connection pooling / keep-alive |
| `CLOSE_WAIT` | Should be near zero | Accumulating = **app not closing connections** — almost always a bug |
| `LISTEN` | Yes | Port missing = service not binding |

### CLOSE_WAIT deep dive

**What it means:** Remote end sent FIN (closed its side). Your app received it. Your app has not called `close()` on its side.

**Root cause:** Application code not properly closing sockets after use — typically missing `defer conn.Close()` or equivalent.

**Impact:** Each `CLOSE_WAIT` holds a file descriptor. FD limit exhausted → new connections fail → service appears broken while consuming no CPU.

**Diagnosis:**
```bash
ss -tan | grep CLOSE_WAIT | wc -l     # how many?
ls /proc/<PID>/fd | wc -l             # how close to FD limit?
cat /proc/<PID>/limits | grep "open files"
```

---

## Binding Interface — Critical Distinction

```
0.0.0.0:8080   → listening on ALL interfaces → accepts external connections
127.0.0.1:8080 → listening on loopback ONLY → rejects external connections
```

**Common production failure:** Service configured to bind `127.0.0.1` accidentally. `curl localhost:8080` from same server works. All external traffic times out. Engineers spend an hour on firewall rules. Fix is one config line.

---

## Service Not Listening — Diagnostic Tree

```
ss -tlnp shows port missing
    │
    ├── Port on wrong interface (127.0.0.1 vs 0.0.0.0)
    │       → config issue, change bind address
    │
    ├── Service crashed immediately after start
    │       → systemctl status, journalctl -u myapp -n 50
    │
    ├── Wrong port in config (typo)
    │       → ss -tlnp (find where it actually bound)
    │
    └── Port already in use by another process
            → ss -tlnp sport = :<port>  (find who owns it)
```

---

## tcpdump — Packet Capture

### Key commands

```bash
# Capture all traffic on interface
tcpdump -i eth0

# Capture on specific port
tcpdump -i eth0 port 8080

# Capture to/from specific host
tcpdump -i eth0 host 192.168.1.100

# Combine: traffic to Redis
tcpdump -i eth0 -n host redis-host port 6379

# Show packet contents in ASCII
tcpdump -i eth0 port 8080 -A

# Save for Wireshark analysis
tcpdump -i eth0 port 8080 -w capture.pcap

# Read saved capture
tcpdump -r capture.pcap

# Don't resolve hostnames (faster, avoid DNS lag)
tcpdump -i eth0 -n port 8080
```

### When to use tcpdump

- Service logs show "connection refused" but dependency appears healthy — verify packets are actually flowing
- TLS handshake failures — see what's happening at packet level
- Intermittent connection resets — capture to find pattern
- Verifying load balancer sends traffic to correct upstream
- Can't tell if error is client-side or server-side from logs alone

### Reading tcpdump output

```
# SYN sent, no SYN-ACK → network issue or server not listening
12:00:01 IP 10.0.0.1.45678 > 10.0.0.2.6379: Flags [S]    ← SYN
12:00:04 IP 10.0.0.1.45678 > 10.0.0.2.6379: Flags [S]    ← retransmit
# No response = network blocked or Redis not listening

# Successful handshake
12:00:01 IP 10.0.0.1 > 10.0.0.2: Flags [S]     ← SYN
12:00:01 IP 10.0.0.2 > 10.0.0.1: Flags [S.]    ← SYN-ACK
12:00:01 IP 10.0.0.1 > 10.0.0.2: Flags [.]     ← ACK
```

---

## Tool Selection Guide

| Question | Tool |
|----------|------|
| What ports am I listening on? | `ss -tlnp` |
| Is my service binding on the right interface? | `ss -tlnp` |
| How many established connections? | `ss -tnp state established` |
| Connection state distribution? | `ss -tan \| awk '{print $1}' \| sort \| uniq -c` |
| Are CLOSE_WAIT connections accumulating? | `ss -tan \| grep CLOSE_WAIT \| wc -l` |
| Are packets actually flowing to Redis? | `tcpdump -i eth0 host redis-host port 6379` |
| Is a TLS handshake completing? | `tcpdump -i eth0 port 443 -A` |
| Who owns a specific port? | `ss -tlnp sport = :<port>` |

---

## Failure Scenarios

- `CLOSE_WAIT` accumulation → FD exhaustion → service rejects new connections while appearing healthy
- `TIME_WAIT` storm → running out of ephemeral ports → new connections fail → use connection pooling
- Binding to `127.0.0.1` → external traffic times out → appears as network/firewall issue
- `somaxconn` too low → high `Recv-Q` → connections silently dropped under load
- SYN sent, no SYN-ACK → firewall blocking, wrong host, or service not listening on that port

---

## Interview Perspective

- "How do you check what ports a service is listening on?" → `ss -tlnp`
- "Thousands of CLOSE_WAIT connections — what does this mean?" → app not closing sockets, FD leak, almost always a bug
- "Service starts but external connections time out — first thing you check?" → `ss -tlnp` — is it binding on `0.0.0.0` or `127.0.0.1`?
- "Service logs show 'connection refused' to Redis but Redis seems healthy — how do you debug?" → `tcpdump` to verify packets are flowing, then check Redis bind address and port

---

## Revision Summary

- `ss -tlnp` = listening ports with owning process
- `ss -tan | awk... | sort | uniq -c` = connection state distribution — first check for connection issues
- `CLOSE_WAIT` accumulating = app not closing sockets = FD exhaustion incoming
- `TIME_WAIT` large = too many short-lived connections = use keep-alive/pooling
- `0.0.0.0` = all interfaces, `127.0.0.1` = loopback only — binding wrong interface is a common silent failure
- `tcpdump` = packet-level visibility — use when logs say connection error but can't determine where the failure is
- SYN with no SYN-ACK = network/firewall issue or service not listening

---

## Active Recall Questions

1. What does a large number of `CLOSE_WAIT` connections indicate and what eventually breaks?
2. Your service is running but external HTTP requests time out. `ss -tlnp` shows `127.0.0.1:8080`. What is wrong?
3. `tcpdump` shows SYN packets leaving your server to Redis but no SYN-ACK returns. Where is the problem?
4. How do you check connection state distribution and what does each state tell you?
5. What is the difference between `ss -tlnp` and `ss -tnp state established`?
6. How do you find which process is listening on port 5432?

---

## Related Concepts

- [[Linux Process Management]]
- [[Linux Filesystem Structure]]
- [[Networking — TCP 3-Way Handshake]]
- [[Networking — TCP Connection States]]
- [[Linux Systemd and Services]]
- [[File Descriptors and epoll]]
- [[net.core.somaxconn]]
