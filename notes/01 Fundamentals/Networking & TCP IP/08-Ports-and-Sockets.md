# Ports & Sockets

## Tags
#networking #tcp #os #backend-engineering

---

## Overview

- **Port**: 16-bit number identifying a specific process/service on a host
- **Socket**: combination of IP address + port — the actual endpoint of a TCP/UDP connection
- A TCP connection is uniquely identified by a **4-tuple**: `(src IP, src port, dst IP, dst port)`
- Sockets are **file descriptors** in Linux — everything is a file; the kernel owns the TCP machinery

---

## Port Ranges

```
0–1023        Well-known ports   (require root to bind)
              HTTP=80, HTTPS=443, SSH=22, DNS=53, SMTP=25

1024–49151    Registered ports   (application use)
              PostgreSQL=5432, Redis=6379, MySQL=3306

49152–65535   Ephemeral ports    (OS-assigned for outbound connections)
              Linux default: 32768–60999
```

Check Linux ephemeral range:
```bash
cat /proc/sys/net/ipv4/ip_local_port_range
```

---

## The 4-Tuple — Why One Server Port Handles Thousands of Connections

A common misconception: "port 443 can only handle one connection."

A connection is unique by its full 4-tuple:
```
(src IP,   src port, dst IP,       dst port)
(clientA,  54321,    server:443,   443     )  ← connection 1
(clientB,  54322,    server:443,   443     )  ← connection 2
(clientA,  54323,    server:443,   443     )  ← connection 3 (same client, different src port)
```

All three share `dst port=443` but are distinct connections. The server can have millions of connections all on port 443 — as long as the source differs.

This is also why **a single server IP + port can handle massive concurrency** — the OS demultiplexes incoming packets by full 4-tuple to the correct socket.

---

## Sockets as File Descriptors

In Linux, a socket is a file descriptor:
```
socket()   → returns fd (e.g. fd=7)
connect()  → establishes TCP connection on fd=7
write(7)   → send bytes
read(7)    → receive bytes
close(7)   → close connection, fd released
```

**Implications:**
- `ulimit -n` limits file descriptors per process (default often 1024)
- Each open socket consumes one fd
- High-concurrency servers must increase fd limit:

```bash
ulimit -n 1000000              # session limit
# /etc/security/limits.conf:
*    soft    nofile    1000000
*    hard    nofile    1000000
```

- Fd exhaustion → `too many open files` error → service cannot accept new connections

---

## Server Socket vs Connection Socket

When a server listens:
```
server creates listening socket on port 443
client connects → OS creates a NEW socket for that specific connection
listening socket remains open for new connections

fd=5  → listening socket  (port 443, accepts new connections)
fd=6  → connection with clientA:54321
fd=7  → connection with clientB:54322
fd=8  → connection with clientC:54323
```

Each accepted connection is a separate fd. A server with 10k concurrent connections uses 10k + 1 fds (plus fds for files, pipes, etc.).

---

## Ports in Backend Engineering

**Binding a server port:**
- Requires the port to be free (no other process bound to it)
- Ports < 1024 require root / `CAP_NET_BIND_SERVICE`
- `SO_REUSEADDR`: allow binding to a port in TIME_WAIT state (critical for server restart after crash)
- `SO_REUSEPORT`: allow multiple sockets to bind same port — distributes incoming connections across workers (used by Nginx, modern servers)

**Ephemeral ports and connection pooling:**
- Each outbound connection consumes one ephemeral port until closed + TIME_WAIT expires
- Pool of 100 persistent connections = 100 ports held continuously but stably
- 1000 connections/sec without pooling = thousands of ephemeral ports cycling through TIME_WAIT

---

## Failure Scenarios

- **`EADDRINUSE`**: another process is bound to the port; or TIME_WAIT socket exists on that port (fix: `SO_REUSEADDR`)
- **`EADDRNOTAVAIL`**: ephemeral port range exhausted; cannot create new outbound socket
- **`EMFILE` / `ENFILE`**: process or system fd limit hit → "too many open files"
- **Port scanning**: open ports are discoverable; keep only necessary ports open; firewall all others

---

## Interview Perspective

- "How can a server handle 100,000 connections all on port 443?" → 4-tuple uniqueness; one listening socket, many connection sockets
- "What is a socket at the OS level?" → File descriptor; kernel manages TCP state behind it
- "Why does a server need SO_REUSEADDR after a crash?" → Restarting process binds same port; old socket may be in TIME_WAIT without it binding fails with EADDRINUSE
- "What is SO_REUSEPORT used for?" → Multiple workers bind same port; kernel load-balances incoming connections across them
- Common mistake: thinking port = connection (it's 4-tuple = connection)

---

## Revision Summary

- Port = 16-bit process identifier; socket = IP + port = endpoint
- TCP connection = unique 4-tuple; one server port handles millions of connections
- Sockets are fds; `ulimit -n` governs max open connections per process
- Server: one listening fd + one fd per accepted connection
- `SO_REUSEADDR` = rebind TIME_WAIT port; `SO_REUSEPORT` = multi-worker port sharing
- Ephemeral ports used for outbound; range ~28k (default) to ~64k (tuned)

---

## Active Recall Questions

1. How can a server simultaneously handle 100,000 connections all on destination port 443?
2. What Linux resource limits a process's maximum number of open connections?
3. What is the difference between `SO_REUSEADDR` and `SO_REUSEPORT`?
4. A server crashes and immediately restarts; it fails to bind port 8080 with `EADDRINUSE`. What caused this and how do you fix it?
5. What is the relationship between sockets and file descriptors in Linux?

---

## Related Concepts

- [[Port Exhaustion and TIME_WAIT]]
- [[TCP - Three-Way Handshake and Connection Lifecycle]]
- [[NAT and Routing]]
- [[OSI Model & TCP/IP Stack]]
- [[Linux File Descriptors]]
