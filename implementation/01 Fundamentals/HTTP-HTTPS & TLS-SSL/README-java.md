# HTTP Server from Scratch — Java

A production-quality HTTP/1.1 server built from raw TCP sockets using Java 21 virtual threads.
No servlet containers, no Spring, no Netty. Implements HTTP/1.1 wire protocol manually.

---

## Architecture

```
┌──────────────────────────────────────────────────────────┐
│                     HTTP Server                          │
│                                                          │
│  ServerSocket                                            │
│      │ accept() → Socket                                 │
│      ▼                                                   │
│  HttpServer.acceptLoop()                                 │
│      │ semaphore.acquire() (backpressure)                │
│      │ phaser.register()                                 │
│      ▼                                                   │
│  virtual thread: handleConnection(socket)                │
│      │ try-catch Throwable → 500                         │
│      │ BufferedReader + BufferedInputStream              │
│      ▼                                                   │
│  HttpParser.parseRequest()        ← pure byte logic      │
│      │ request line + headers + body                     │
│      ▼                                                   │
│  Router.lookup(method, path) → LookupResult             │
│      │ sealed interface: Found | NotFound | 405          │
│      │ pattern match in dispatch()                       │
│      ▼                                                   │
│  HandlerFunc.apply(req) → HttpResponse                   │
│      ▼                                                   │
│  HttpParser.serializeResponse()   ← writes bytes         │
│      ▼                                                   │
│  Socket OutputStream (keep-alive loop)                   │
└──────────────────────────────────────────────────────────┘
```

---

## Project Structure

```
http-server-java/
├── pom.xml
└── src/
    ├── main/java/com/httpserver/
    │   ├── Main.java                    # Entrypoint: wiring + shutdown hook
    │   ├── ParserTests.java             # Standalone test runner (no JUnit dep)
    │   ├── handler/
    │   │   └── Handlers.java            # /health, /hello, /echo handlers
    │   ├── parser/
    │   │   ├── HttpRequest.java         # Immutable request type
    │   │   ├── HttpResponse.java        # Response type with factory methods
    │   │   ├── HttpParser.java          # HTTP/1.1 wire parser + serializer
    │   │   └── HttpParseException.java  # Checked parse exception
    │   ├── router/
    │   │   └── Router.java              # Exact-match router (sealed LookupResult)
    │   └── server/
    │       ├── HttpServer.java          # TCP listener, virtual threads, shutdown
    │       └── ServerConfig.java        # Immutable config record
    └── test/java/com/httpserver/
        └── HttpParserTest.java          # JUnit 5 parser tests
```

---

## Go → Java Conceptual Mapping

Every design decision has a direct Go equivalent. This mapping is the point.

| Concern | Go | Java |
|---|---|---|
| Concurrency primitive | goroutine (2KB) | Virtual thread (Java 21 Loom) |
| Connection semaphore | `chan struct{}` (buffered) | `java.util.concurrent.Semaphore` |
| In-flight tracking | `sync.WaitGroup` | `Phaser` + `awaitTermination` |
| Shutdown signaling | `close(chan struct{})` | `AtomicBoolean` + `ServerSocket.close()` |
| Error handling | `(value, error)` multiple returns | Checked exceptions (`HttpParseException`) |
| Router result type | `(handler, *Response)` | Sealed interface `LookupResult` |
| Signal handling | `signal.Notify(ch, SIGTERM)` | `Runtime.addShutdownHook` |
| Config struct | `type Config struct{}` | `record ServerConfig(...)` |
| Handler type | `type HandlerFunc func(...)` | `@FunctionalInterface HandlerFunc` |
| Panic recovery | `recover()` in deferred func | `catch (Throwable t)` |

---

## Design Decisions

### Virtual Threads (Project Loom)

```java
ExecutorService executor = Executors.newVirtualThreadPerTaskExecutor();
```

Virtual threads are JVM-managed, not OS threads. Blocking I/O on a virtual
thread suspends the virtual thread and releases the OS thread to run other work.
This is semantically identical to Go's goroutine scheduler.

Before Loom (Java 8–20), thread-per-connection required either:
- OS threads: expensive (~1MB each), limited to ~10k concurrent
- NIO + callbacks: Netty/reactive style, complex non-blocking code

With Loom: blocking code, cheap threads, same scalability as reactive.
The Go model arrived in Java 21.

### Sealed Interface for Router Results

```java
public sealed interface LookupResult permits Found, NotFound, MethodNotAllowed {}
```

Go expresses "handler or error response" with two return values:
```go
func Lookup(...) (HandlerFunc, *Response)
```

Java sealed interfaces model this more explicitly — the three outcomes
are exhaustive and the compiler enforces handling all cases via pattern matching:

```java
return switch (router.lookup(method, path)) {
    case Router.LookupResult.Found f            -> f.handler().apply(req);
    case Router.LookupResult.NotFound n         -> HttpResponse.notFound();
    case Router.LookupResult.MethodNotAllowed m -> HttpResponse.methodNotAllowed();
};
```

Missing a case is a compile error. The Go version relies on nil checks.

### Checked Exceptions vs Multiple Returns

Go:
```go
req, err := parser.ParseRequest(reader)
if err != nil { ... }
```

Java:
```java
try {
    HttpRequest req = HttpParser.parseRequest(reader, stream, addr);
} catch (HttpParseException e) { ... }
  catch (EOFException e) { ... }
```

Both enforce error handling at compile time. Go's approach: the compiler won't
let you use `req` if you haven't checked `err`. Java's approach: checked
exceptions must appear in `throws` clauses or be caught. Different syntax,
same intent — errors are not silently ignorable.

### BufferedReader + BufferedInputStream Duality

Go's `bufio.Reader` operates at the byte level throughout — reads both text
lines and binary body bytes from one abstraction.

Java's `BufferedReader` converts bytes to chars (text). Binary body reading
requires a raw `InputStream`. Two wrappers, one underlying socket stream:

```java
BufferedInputStream rawStream = new BufferedInputStream(conn.getInputStream());
BufferedReader reader = new BufferedReader(new InputStreamReader(rawStream, "ISO-8859-1"));
```

`ISO-8859-1` (Latin-1) for headers — RFC 7230 specifies headers as ASCII.
`rawStream.readNBytes(n)` for body — equivalent to Go's `io.ReadFull`.

**Critical**: both wrap the same underlying stream. Consuming bytes through
`reader` advances `rawStream`'s position. Reading headers through `reader`
then reading body through `rawStream` works because they share state.

### Immutable HttpRequest

```java
public final class HttpRequest {
    private final Map<String, String> headers;
    // Defensive copy in constructor, defensive copy on getBody()
}
```

Go's `Request` struct is passed by pointer, mutable in principle but not
mutated in practice. Java's immutability is enforced by the type system —
`final` fields, unmodifiable map view, defensive copies.

Safe to share across threads without synchronization. Handlers can't
accidentally corrupt request state.

---

## Security Limits

| Limit | Value | Reason |
|---|---|---|
| MAX_HEADER_SIZE | 8KB | Prevent slow-loris / header flood |
| MAX_BODY_SIZE | 1MB | Prevent OOM from unbounded body reads |
| MAX_HEADER_COUNT | 100 | Prevent header explosion |
| readTimeout | 10s | `socket.setSoTimeout()` — disconnect slow clients |
| maxConnections | 1000 | Semaphore backpressure at accept layer |

---

## Shutdown Design

```java
// 1. Signal accept loop
shutdownRequested.set(true);

// 2. Close server socket — unblocks accept() with SocketException
closeQuietly(serverSocket);

// 3. Stop accepting new virtual threads
executor.shutdown();

// 4. Wait for in-flight connections (with deadline)
executor.awaitTermination(timeoutMs, TimeUnit.MILLISECONDS);
```

Equivalent to Go's `context.WithTimeout` + `wg.Wait()`. The `AtomicBoolean`
replaces Go's `close(chan struct{})` — both provide a visible signal across
goroutines/threads without locks.

---

## Running

```bash
# Compile
javac --enable-preview --release 21 -d out $(find src/main/java -name "*.java")

# Run server
java --enable-preview -cp out com.httpserver.Main

# Run standalone parser tests
java --enable-preview -cp out com.httpserver.ParserTests
```

## Testing (JUnit)

```bash
mvn test
```

---

## Adding TLS

```java
// Replace ServerSocket with SSLServerSocket:
SSLServerSocketFactory factory = (SSLServerSocketFactory) SSLServerSocketFactory.getDefault();
SSLServerSocket serverSocket = (SSLServerSocket) factory.createServerSocket(8443);

// System properties for cert:
// -Djavax.net.ssl.keyStore=keystore.jks
// -Djavax.net.ssl.keyStorePassword=changeit

// Everything else (parser, router, handlers) is identical.
// SSLSocket implements Socket — transparent to handleConnection().
```

---

## Future Improvements

- **Chunked transfer encoding** — streaming without Content-Length
- **Pattern routing** — trie-based for `/users/{id}`
- **Middleware** — `Function<HandlerFunc, HandlerFunc>` composition
- **HTTP/2** — ALPN + stream multiplexing (use Netty or Jetty for this in prod)
- **Metrics** — Micrometer counters and histograms
- **Structured logging** — replace `java.util.logging` with SLF4J + Logback
