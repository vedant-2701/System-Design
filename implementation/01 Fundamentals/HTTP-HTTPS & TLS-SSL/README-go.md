# HTTP Server from Scratch — Go

A production-quality HTTP/1.1 server built from raw TCP sockets. No `net/http` library.
Implements the HTTP/1.1 wire protocol manually to expose every layer that stdlib hides.

---

## Architecture

```
┌─────────────────────────────────────────────────────┐
│                     HTTP Server                     │
│                                                     │
│  net.Listener                                       │
│      │ Accept() → net.Conn                          │
│      ▼                                              │
│  Server.acceptLoop()                                │
│      │ semaphore acquire (backpressure)             │
│      │ wg.Add(1)                                    │
│      ▼                                              │
│  goroutine: handleConnection(conn)                  │
│      │ panic recovery → 500                         │
│      │ bufio.Reader over net.Conn                   │
│      ▼                                              │
│  parser.ParseRequest()          ← pure byte logic   │
│      │ request line + headers + body                │
│      ▼                                              │
│  router.Lookup(method, path)                        │
│      │ O(1) map lookup                              │
│      │ 404 / 405 discrimination                     │
│      ▼                                              │
│  HandlerFunc(req) → *Response                       │
│      ▼                                              │
│  parser.SerializeResponse()     ← writes bytes      │
│      ▼                                              │
│  net.Conn (keep-alive loop)                         │
└─────────────────────────────────────────────────────┘
```

---

## Project Structure

```
http-server-go/
├── cmd/server/
│   └── main.go              # Entrypoint: wiring + OS signal handling
├── internal/
│   ├── logger/
│   │   └── logger.go        # Structured logger (level-filtered, injectable writer)
│   ├── parser/
│   │   ├── types.go         # Request / Response types + factory methods
│   │   ├── parser.go        # HTTP/1.1 wire parser + serializer
│   │   └── parser_test.go   # 14 unit tests (no network)
│   ├── router/
│   │   ├── router.go        # Exact-match router with 404/405 discrimination
│   │   └── router_test.go   # 5 unit tests
│   ├── handler/
│   │   └── handlers.go      # Sample handlers: /health, /hello, /echo
│   └── server/
│       ├── server.go        # TCP listener, connection lifecycle, graceful shutdown
│       └── server_test.go   # 8 integration tests over real TCP connections
└── go.mod
```

---

## Design Decisions

### Why `net` package and not raw TCP?

`net.Listen` + `net.Conn` gives us real TCP sockets. From `Accept()` onwards,
every byte is ours — request line parsing, header parsing, body reading, response
serialization. The alternative (implementing TCP itself) teaches network engineering,
not HTTP engineering. The goal here is HTTP internals.

### Parser decoupled from net.Conn

`ParseRequest` takes `*bufio.Reader`, not `net.Conn`. This means:
- Tests use `strings.NewReader` — no network needed, no ports, fast
- The parser is pure byte logic with no I/O side effects
- Any `io.Reader` source works: file, memory buffer, network

### One goroutine per connection

Go's goroutines are ~2KB stack vs ~1MB for OS threads. Spawning one per
connection is idiomatic and scales to tens of thousands of concurrent connections.
The goroutine blocks on `conn.Read()` — Go's scheduler parks it and runs
other goroutines, using OS threads only when work is available.

### Semaphore as `chan struct{}`

```go
connSemaphore chan struct{}  // buffered channel of size MaxConnections
```

Sending to a full buffered channel blocks. This creates backpressure at
`Accept()` — new connections wait rather than spawning unlimited goroutines
when the server is saturated. Alternative: `sync.Semaphore` from `golang.org/x/sync`.
Channel is idiomatic Go and has no external dependency.

### 404 vs 405 distinction

The router maintains two maps:
1. `routes["METHOD /path"] → handler` — exact match
2. `paths["/path"] → true` — path existence without method

On lookup miss, we check `paths` first. If the path exists, return 405 (wrong method).
If the path doesn't exist, return 404 (resource not found). Returning 404 for a
wrong method confuses API clients — they think the resource doesn't exist.

### Panic recovery per connection

```go
defer func() {
    if r := recover(); r != nil { ... return 500 }
}()
```

A panic in one handler goroutine must not crash the server process. Recovery
is mandatory in any goroutine that isn't `main`. Production enhancement: emit
a `panic_count` metric here to detect handler bugs in dashboards.

### Graceful shutdown design

```
1. close(s.shutdown)            — signal accept loop
2. s.listener.Close()           — unblock Accept() with net.ErrClosed
3. wg.Wait() with ctx deadline  — drain in-flight connections
```

`sync.WaitGroup` tracks in-flight connection goroutines. Shutdown waits for
the WaitGroup to reach zero, respecting a context deadline. This is the
Kubernetes SIGTERM contract — pods get 30 seconds by default; we give
in-flight requests 15 seconds to complete.

### ServeListener separation

`Start()` binds and serves. `ServeListener(ln)` serves an already-bound listener.
This allows tests to:
1. Bind on `:0` (OS-assigned port)
2. Extract the actual address
3. Pass the listener to the server

Without this separation, tests would need to know the port before the server
starts, creating a race condition.

---

## Alternatives Considered

### Thread pool instead of goroutine-per-connection

Java's historical model: fixed thread pool, non-blocking I/O (NIO/Netty), callbacks.
Correct for OS threads — you can't have 10,000 OS threads (1MB each = 10GB RAM).
Wrong for goroutines — 2KB each, Go scheduler parks them on I/O automatically.
Goroutine-per-connection is simpler and equally scalable for Go.

### Event loop (epoll directly)

`epoll`/`kqueue` — the model nginx and Node.js use. Single thread, all I/O
is non-blocking, events dispatched via callbacks. Maximum efficiency, minimum
memory. Complexity cost: all handler code must be non-blocking. Go's net package
uses epoll internally — we get its benefits without the callback complexity.

### bufio.Writer for response writing

We write directly to `net.Conn` for response serialization. A `bufio.Writer`
would batch small writes into fewer syscalls. For our response sizes this
doesn't matter; for a production server serving many tiny responses (health
checks at high QPS), buffered writes reduce syscall overhead.

---

## Security Limits

| Limit | Value | Reason |
|---|---|---|
| MaxHeaderSize | 8KB | Prevent slow-loris / header flood |
| MaxBodySize | 1MB | Prevent OOM from unbounded body reads |
| MaxHeaderCount | 100 | Prevent header explosion attacks |
| ReadTimeout | 10s | Disconnect slow/stalled clients |
| WriteTimeout | 10s | Disconnect unresponsive clients |
| IdleTimeout | 60s | Reclaim connections idle between requests |
| MaxConnections | 1000 | Backpressure — reject floods at accept layer |

---

## Edge Cases Handled

- **Malformed request line** — wrong number of parts, invalid method, missing path prefix
- **Header case normalization** — RFC 7230 specifies case-insensitive names; normalized to lowercase
- **Non-numeric Content-Length** — rejected with 400
- **Negative Content-Length** — rejected with 400
- **Oversized headers** — rejected before reading body
- **Too many headers** — rejected to prevent map memory growth
- **Client closes connection mid-request** — detected via EOF, closed cleanly
- **Panic in handler** — recovered, 500 returned, connection closed
- **Shutdown during in-flight request** — request completes before server closes
- **Keep-alive loop** — connection reused across multiple sequential requests
- **Connection flood** — semaphore blocks accept until slot available

---

## Running

```bash
go run ./cmd/server
# Server starts on :8080

curl http://localhost:8080/health
curl http://localhost:8080/hello
curl -X POST http://localhost:8080/echo -d '{"test":1}'
```

## Testing

```bash
# All tests
go test ./...

# With race detector (mandatory before any production deploy)
go test -race ./...

# Verbose output
go test -race -v ./...

# Specific package
go test -race -v ./internal/parser/
```

---

## Adding TLS

The entire HTTP stack works unchanged with TLS. Only the listener changes:

```go
cert, err := tls.LoadX509KeyPair("cert.pem", "key.pem")
cfg := &tls.Config{Certificates: []tls.Certificate{cert}}
ln, err := tls.Listen("tcp", ":8443", cfg)
srv.ServeListener(ln)  // net.Conn returned by Accept() is now tls.Conn
                       // implements net.Conn — transparent to the rest
```

TLS is a wrapper on `net.Conn`. The parser reads bytes from it identically.

---

## Future Improvements

- **Chunked transfer encoding** — required for streaming responses; currently only Content-Length bodies
- **Pattern/prefix routing** — replace exact map with trie for `/users/{id}` support
- **Middleware chain** — wrap HandlerFunc for logging, auth, rate limiting
- **HTTP/2 upgrade** — requires ALPN negotiation during TLS + stream multiplexing
- **Buffered response writes** — `bufio.Writer` on response path for high-QPS scenarios
- **pprof endpoint** — `/debug/pprof` for production profiling
- **Metrics** — request count, latency histogram, active connection gauge
