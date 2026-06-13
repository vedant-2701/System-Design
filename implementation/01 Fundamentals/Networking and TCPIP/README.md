# TCP Lab — Implementation README

## Problem Statement

Implement a TCP echo server and client from scratch to deeply understand:
- TCP as a byte stream (not a message protocol)
- Connection lifecycle: establishment, data transfer, teardown
- Dead peer detection: keepalive + idle timeouts
- Graceful shutdown under concurrent load
- Production-quality error classification

---

## Architecture

```
┌─────────────────────────────────────────────────────────┐
│  Client                     Server                      │
│                                                         │
│  stdin → Scanner            acceptLoop()                │
│       ↓                          ↓                      │
│  WriteMessage()         handleConnection() [goroutine]  │
│  [4B header + payload]       ↓                          │
│       ──────────────────→ ReadMessage()                 │
│                              ↓                          │
│  ReadMessage()          WriteMessage() [echo]           │
│       ←────────────────── [4B header + payload]         │
│       ↓                                                 │
│  print echo + RTT                                       │
└─────────────────────────────────────────────────────────┘
```

### Framing Protocol (Wire Format)

```
Byte 0    Byte 1    Byte 2    Byte 3    Byte 4..N
─────────────────────────────────────────────────
[     big-endian uint32 length     ] [  payload  ]
```

Both Go and Java implementations use identical wire format — they are interoperable.

---

## Design Decisions

### 1. Length-Prefix Framing (not delimiter framing)

TCP provides a byte stream with no message boundaries. Two common solutions:

**Delimiter framing (`\n` terminator):**
- Simple to implement
- Breaks with binary payloads containing `\n`
- Requires scanning the entire buffer (O(n))
- Used by: HTTP/1.1, Redis RESP protocol, SMTP

**Length-prefix framing (chosen):**
- O(1) to frame and deframe
- Safe with arbitrary binary payloads
- Requires knowing message size upfront
- Used by: gRPC, most binary protocols, Kafka wire format

### 2. Go: Goroutine-Per-Connection

Alternatives considered:
- **Event loop (epoll directly)**: lower memory overhead, but complex callback code
- **Worker pool**: bounded concurrency, but head-of-line blocking if all workers are busy
- **Goroutine-per-connection (chosen)**: idiomatic Go, simple blocking I/O code, goroutines are cheap (~2-8KB each), Go runtime multiplexes onto OS threads automatically

Limitation: at 100k+ connections, goroutine overhead becomes measurable. At that scale, consider `netpoll` or a custom event loop.

### 3. Java: Virtual Threads (Project Loom)

Alternatives considered:
- **Platform thread pool (ThreadPoolExecutor)**: bounded by `ulimit -u`, ~1MB per thread, 10k connections = 10GB stack
- **NIO + Selector (non-blocking)**: manually manage state machines per connection, complex but memory-efficient
- **Netty**: production NIO framework, high throughput, but significant abstraction overhead
- **Virtual threads (chosen)**: blocking I/O semantics (simple code) + JVM multiplexes onto carrier threads (~10 by default)

**Virtual thread pitfall:** `synchronized` blocks pin virtual threads to carrier threads. At high concurrency, many pinned threads exhaust the carrier pool. Use `ReentrantLock` in production code that runs on virtual threads.

### 4. MaxMessageSize Validation

Without size validation, a client sends header = `0xFFFFFFFF` (4GB). The server allocates 4GB before reading a single byte. DoS attack via memory exhaustion.

Defence: validate size before `make([]byte, size)`. Reject and close connection if size > 1MB.

### 5. Graceful Shutdown

On SIGINT/SIGTERM:
1. Stop accepting new connections (close listener)
2. In-flight connections finish naturally
3. WaitGroup/executor drain with a timeout
4. Force-close remaining connections after timeout

Without step 3, in-flight responses are interrupted. Without step 4, a stuck handler blocks shutdown forever.

### 6. Dead Peer Detection — Two Mechanisms

**TCP Keepalive (OS-level):**
- OS sends probe packets after N seconds of silence
- Detects: unplugged cable, crashed OS, network partition
- Default idle time: 2 hours — we set it to 30 seconds
- Works even if application is not reading

**Read Deadline / SO_TIMEOUT (application-level):**
- `read()` returns timeout error after N seconds without data
- Detects: connected client that stops sending (alive but silent)
- Complements keepalive — keepalive requires TCP connectivity; read deadline handles application-level silence
- We set 60 seconds

In production, you typically want both.

---

## Alternatives Considered

### Not Implemented: Buffered Writing

Currently each `WriteMessage` does two `Write` calls (header then payload), potentially two TCP segments. A `bufio.Writer` would coalesce them into one:

```go
bw := bufio.NewWriter(conn)
binary.Write(bw, binary.BigEndian, uint32(len(payload)))
bw.Write(payload)
bw.Flush() // one syscall, one segment
```

Not implemented to keep the framing code focused. Add in production for throughput-sensitive paths.

### Not Implemented: Connection Pool (Client-Side)

The client creates one connection and reuses it for the session. A production client making many parallel requests would maintain a pool of connections. See [[HTTP Connection Pooling]] for the full discussion.

### Not Implemented: TLS

A production server would wrap the `net.Conn` in `tls.Conn` (Go) or use `SSLSocket` (Java). The framing layer is unchanged — TLS operates transparently at the connection level.

---

## Complexity Analysis

| Operation | Complexity | Notes |
|-----------|-----------|-------|
| WriteMessage | O(1) | Two writes: 4-byte header + payload |
| ReadMessage | O(1) | ReadFull header (4B), ReadFull payload (N bytes) |
| Accept connection | O(1) | OS accept() + goroutine spawn |
| Concurrent connections | O(n) memory | n goroutines × ~8KB each |

---

## Edge Cases Handled

| Edge Case | Handling |
|-----------|---------|
| Partial read (TCP fragmentation) | `io.ReadFull` / `DataInputStream.readFully()` handles internally |
| Oversized message header (DoS) | Validate size < MaxMessageSize before allocation |
| Clean client disconnect (FIN) | `io.EOF` / `EOFException` — log at Info, not Error |
| Abrupt disconnect (RST) | `ECONNRESET` / `"Connection reset"` — log at Info |
| Idle client (no data) | Read deadline triggers timeout → close connection |
| Dead peer (network failure) | TCP keepalive detects, `read()` returns error |
| Shutdown during active connections | WaitGroup drains with timeout |
| Port already in use after crash | `SO_REUSEADDR` on server socket |

---

## Running the Lab

### Go

```bash
cd go/

# Terminal 1: Start server
go run ./server/

# Terminal 2: Start client
go run ./client/

# Type messages, see echoes

# Run all tests
go test ./... -v
```

### Java

```bash
cd java/

# Compile
find src/main -name "*.java" | xargs javac --release 21 -d target/classes

# Terminal 1: Start server
java -cp target/classes com.tcplab.server.TcpServer

# Terminal 2: Start client
java -cp target/classes com.tcplab.client.TcpClient
```

### Cross-Language Test (Go server ↔ Java client)

Both implementations use the same wire format. Start the Go server and connect
with the Java client — messages should echo correctly. This validates wire
compatibility across languages.

---

## Go ↔ Java Conceptual Mapping

| Go | Java | Concept |
|----|------|---------|
| `goroutine` | `virtual thread` | Lightweight concurrent execution unit |
| `context.Context` | `volatile boolean` / `CompletableFuture` | Cancellation propagation |
| `sync.WaitGroup` | `executor.awaitTermination()` | Drain in-flight work |
| `io.ReadFull()` | `DataInputStream.readFully()` | Guaranteed N-byte read |
| `net.Error.Timeout()` | `SocketTimeoutException` | Read deadline exceeded |
| `io.EOF` | `EOFException` | Clean stream end |
| `syscall.ECONNRESET` | `"Connection reset"` SocketException | RST received |
| `signal.NotifyContext` | `Runtime.addShutdownHook` | Signal handling |
| `conn.SetKeepAlive(true)` | `socket.setKeepAlive(true)` | TCP keepalive |
| `conn.SetReadDeadline(t)` | `socket.setSoTimeout(ms)` | Idle timeout |

---

## Production Considerations

- **TLS**: wrap connection in TLS for any non-loopback traffic
- **Buffered writes**: use `bufio.Writer` to coalesce small writes into one syscall
- **Metrics**: instrument accept rate, active connection count, message latency (p50/p99), error rates
- **Tracing**: attach request ID to each message, include in logs for distributed tracing
- **Rate limiting**: limit connections per IP to prevent DoS
- **Backpressure**: if the handler is slow, TCP flow control (rwnd=0) will naturally slow the sender
- **File descriptor limits**: `ulimit -n` must be set higher than max expected connections
