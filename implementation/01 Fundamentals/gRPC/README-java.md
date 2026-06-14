# gRPC OrderService — Java Implementation

Java 21 + Project Loom (virtual threads) implementation of the same OrderService
as the Go version, demonstrating the same RPC patterns with Java's gRPC API.

---

## Prerequisites

```bash
# Java 21+ (required for virtual threads)
java -version

# Maven 3.8+
mvn -version
```

This environment has Java 21 and Maven installed, but **no access to Maven Central**
(`repo.maven.apache.org` is blocked here). Run this locally where Maven Central
is reachable.

---

## Build and Run

```bash
# Generates Protobuf/gRPC code from .proto, compiles, packages a fat jar
mvn clean package

# Terminal 1 — start server
java -jar target/grpc-demo-1.0-SNAPSHOT-jar-with-dependencies.jar

# Terminal 2 — run client demo
java -cp target/grpc-demo-1.0-SNAPSHOT-jar-with-dependencies.jar com.example.grpc.client.ClientMain
```

### Testing with grpcurl

Same as the Go server — wire format is identical, the schema is the same `.proto`.

```bash
grpcurl -plaintext localhost:50051 list

grpcurl -plaintext \
  -H "authorization: Bearer valid-token" \
  -d '{"order_id": 0}' \
  localhost:50051 order.OrderService/GetOrder
```

---

## Running Tests

```bash
# Unit + concurrency tests (fast)
mvn test

# Include high-concurrency stress tests
mvn test -Dtest=OrderServiceLoadTest

# Skip stress tests
mvn test -DskipStressTests=true
```

### What the tests cover

| Test | Validates |
|---|---|
| `getOrder_success` / `_notFound` | Correct gRPC status codes (NOT_FOUND vs generic error) |
| `createOrder_*InvalidArgument` | Input validation (empty items, zero quantity) |
| `createOrder_concurrentIdsAreUnique` | No duplicate IDs under 100 concurrent virtual-thread creates |
| `watchOrders_fanOutToMultipleSubscribers` | Broadcast reaches all subscribers over real in-process gRPC |
| `watchOrders_slowSubscriberDoesNotBlockCreateOrder` | **The core design property**: a subscriber whose queue fills never blocks `CreateOrder` |
| `watchOrders_cancellationCleansUpSubscriber` | Real client cancellation triggers `ServerCallStreamObserver.onCancelHandler` cleanup |
| `OrderServiceLoadTest.manyWatchersManyCreates` | 500 subscribers + 200×10 creates; max latency < 200ms; subscribers fully cleaned up |
| `OrderServiceLoadTest.floodSingleSlowSubscriber` | 1000 creates with 1 permanently-stuck subscriber complete without blocking |

### Why InProcessServerBuilder, not mocks

grpc-java's own Javadoc for `ServerCallStreamObserver` states: **"DO NOT MOCK: The
API is too complex to reliably mock. Use `InProcessChannelBuilder` to create
'real' RPCs suitable for testing."** Our tests follow this — `OrderServiceImpl`
runs behind a real (in-process) gRPC server, exercised by a real async/blocking
stub. This means `instanceof ServerCallStreamObserver` checks, cancellation
propagation, and flow control all behave exactly as they would in production.

We do **not** use `directExecutor()` (common in grpc-java's own examples for
simple unit tests) because it runs all callbacks synchronously on the calling
thread — which would serialize the very concurrency we're testing. Both server
and channel use `Executors.newVirtualThreadPerTaskExecutor()`, matching
`ServerMain`'s production configuration.

## Go vs Java — Conceptual Mapping

| Concept | Go | Java |
|---|---|---|
| Per-RPC concurrency | Goroutine (automatic) | Virtual thread (configured via `Executors.newVirtualThreadPerTaskExecutor()`) |
| Unary response | `return resp, err` | `responseObserver.onNext(resp); onCompleted();` or `onError(...)` |
| Server streaming | Imperative `for { stream.Send(event) }` loop | Register `StreamObserver`, call `onNext()` from any thread later |
| Client disconnect detection | `ctx.Done()` in `select{}` | `ServerCallStreamObserver.setOnCancelHandler()` |
| Request-scoped values | `context.WithValue(ctx, key, val)` | `Context.current().withValue(KEY, val)`, `.attach()/.detach()` |
| Metadata (headers) | `metadata.FromIncomingContext(ctx)` | `Metadata.Key<String>`, read in interceptor |
| Interceptor chaining | `grpc.ChainUnaryInterceptor(a, b, c)` — first = outermost | `ServerInterceptors.interceptForward(svc, a, b, c)` — first = outermost |
| Concurrent map | `sync.RWMutex` + `map` | `ConcurrentHashMap` |
| Atomic counter | `sync.Mutex` + increment, or `atomic.Uint32` | `AtomicLong` |
| Generated base for forward-compat | Embed `UnimplementedXxxServer` | Extend `XxxGrpc.XxxImplBase` |
| Health check | `health.NewServer()` | `HealthStatusManager` |
| Keepalive config | `grpc.KeepaliveParams(...)` | `.keepAliveTime(...).keepAliveTimeout(...)` on `NettyServerBuilder` |

---

## The Most Important Conceptual Shift: Pull vs Push Streaming

**Go (pull model):**
```go
for {
    event, err := stream.Recv()
    if err == io.EOF { break }
    process(event)
}
```
You are in control of the loop. You decide when to call `Recv()` again.

**Java (push model, async stub):**
```java
asyncStub.watchOrders(request, new StreamObserver<OrderEvent>() {
    public void onNext(OrderEvent event) { process(event); }
    public void onError(Throwable t) { /* handle */ }
    public void onCompleted() { /* handle */ }
});
```
The framework calls your callbacks whenever data arrives. You never "ask" for
the next message — it's pushed to you on a framework-managed thread.

**Why this matters for backpressure:** In Go, if your processing loop is slow,
`Recv()` simply isn't called again — natural backpressure, the server's `Send()`
blocks (or fails if buffer is full, as in our fan-out). In Java's async stub,
`onNext()` calls can arrive faster than you process them unless you use
`ServerCallStreamObserver.disableAutoInboundFlowControl()` and manually call
`request(1)` after processing each message — explicit flow control.

This demo doesn't implement manual flow control (out of scope), but it's the
detail that separates a toy implementation from a production one when message
volume is high.

---

## Virtual Threads — Why They Matter Here

```java
Executor virtualThreadExecutor = Executors.newVirtualThreadPerTaskExecutor();
NettyServerBuilder.forPort(PORT).executor(virtualThreadExecutor)...
```

Without this, grpc-java uses a fixed-size thread pool (default executor) for
handler invocation. If `GetOrder` made a blocking JDBC call:

- **Fixed pool**: handler thread blocks on JDBC for the query duration. Under
  load, pool exhausts — new RPCs queue waiting for a free thread. This is the
  classic "thread pool exhaustion" production incident.

- **Virtual threads**: handler runs on a virtual thread. When it blocks on JDBC
  (assuming JDBC driver supports virtual-thread-friendly blocking, which is true
  for most modern drivers), the virtual thread parks and the underlying carrier
  OS thread is freed to run other virtual threads. You can have tens of thousands
  of concurrent in-flight RPCs with a handful of OS threads.

This is the closest Java analog to Go's goroutine-per-RPC model. It does NOT
make synchronous JDBC as efficient as Go's netpoller-based I/O, but it removes
the thread-pool-sizing problem as a primary bottleneck for I/O-bound RPC handlers.

---

## Interceptor Chain Order

Registered in `ServerMain.java`:

```java
io.grpc.ServerInterceptors.interceptForward(
    orderService,
    recoveryInterceptor(),   // outermost
    loggingInterceptor(),
    authInterceptor()        // innermost, closest to handler
);
```

Same reasoning as Go:
- **Recovery outermost** — catches exceptions from Logging and Auth too
- **Logging before Auth** — auth failures are still logged (forensics, brute-force detection)
- **Auth innermost** — handler never executes for unauthenticated callers

---

## What's Missing (Production Concerns)

Same list as the Go README:
1. TLS / mTLS
2. Real JWT validation (replace `validateToken` stub)
3. Database-backed repository instead of `ConcurrentHashMap`
4. Metrics (Micrometer + Prometheus)
5. Distributed tracing (OpenTelemetry)
6. Rate limiting interceptor
7. gRPC-Gateway equivalent (or separate REST controller)
8. Manual flow control for high-volume streaming (`disableAutoInboundFlowControl`)