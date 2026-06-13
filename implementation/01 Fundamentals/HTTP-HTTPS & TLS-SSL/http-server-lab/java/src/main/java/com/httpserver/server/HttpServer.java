package com.httpserver.server;

import com.httpserver.parser.*;
import com.httpserver.router.Router;

import java.io.*;
import java.net.*;
import java.util.concurrent.*;
import java.util.concurrent.atomic.AtomicBoolean;
import java.util.logging.Level;
import java.util.logging.Logger;

/**
 * HTTP server built on Java virtual threads (Project Loom, Java 21).
 *
 * Conceptual mapping to Go implementation:
 * ┌─────────────────────────────────────────────────────────┐
 * │ Go                      │ Java                          │
 * │─────────────────────────────────────────────────────────│
 * │ goroutine per conn      │ virtual thread per conn       │
 * │ sync.WaitGroup          │ CountDownLatch / Phaser       │
 * │ chan struct{} semaphore │ Semaphore                     │
 * │ close(shutdown)         │ AtomicBoolean + interrupt     │
 * │ recover() from panic    │ try-catch Throwable           │
 * │ net.Listener            │ ServerSocket                  │
 * └─────────────────────────────────────────────────────────┘
 *
 * Virtual threads: lightweight, managed by JVM. Blocking I/O suspends the
 * virtual thread but does NOT block the underlying OS thread — identical
 * semantics to Go goroutines. This makes the thread-per-connection model
 * viable at scale without the memory cost of OS threads (~1MB each).
 */
public class HttpServer {

    private static final Logger log = Logger.getLogger(HttpServer.class.getName());

    private final ServerConfig config;
    private final Router router;

    // Virtual thread executor — Java 21's idiomatic way to create virtual threads.
    // Equivalent to Go's goroutine scheduler.
    private final ExecutorService executor = Executors.newVirtualThreadPerTaskExecutor();

    // Semaphore bounds concurrent connections — same role as Go's chan struct{}
    private final Semaphore connectionSlots;

    // Tracks in-flight connections for graceful shutdown — equivalent to sync.WaitGroup
    private final Phaser phaser = new Phaser(1); // starts with 1 (the accept loop)

    private final AtomicBoolean shutdownRequested = new AtomicBoolean(false);

    private volatile ServerSocket serverSocket; // volatile: written once, read by shutdown

    public HttpServer(ServerConfig config, Router router) {
        this.config = config;
        this.router = router;
        this.connectionSlots = new Semaphore(config.maxConnections());
    }

    /**
     * Binds the server socket and starts accepting connections.
     * Blocks until shutdown is requested.
     */
    public void start() throws IOException {
        ServerSocket socket = new ServerSocket();
        socket.setReuseAddress(true);
        socket.bind(new InetSocketAddress(config.host(), config.port()));
        serve(socket);
    }

    /**
     * Accepts connections on an already-bound ServerSocket.
     * Separated for testability — tests bind on port 0 and extract address before calling.
     * Direct equivalent of Go's ServeListener.
     */
    public void serve(ServerSocket socket) throws IOException {
        this.serverSocket = socket;
        log.info("Server started on " + socket.getLocalSocketAddress()
                + " maxConnections=" + config.maxConnections());

        try {
            acceptLoop(socket);
        } finally {
            phaser.arriveAndDeregister(); // accept loop done
        }
    }

    private void acceptLoop(ServerSocket socket) {
        while (!shutdownRequested.get()) {
            Socket conn;
            try {
                conn = socket.accept();
            } catch (IOException e) {
                if (shutdownRequested.get()) {
                    return; // expected during shutdown
                }
                log.log(Level.SEVERE, "Accept error", e);
                return;
            }

            // Acquire connection slot — blocks if at maxConnections.
            // Non-blocking tryAcquire would reject connections instead of waiting;
            // blocking acquire provides backpressure at the accept layer.
            try {
                connectionSlots.acquire();
            } catch (InterruptedException e) {
                Thread.currentThread().interrupt();
                closeQuietly(conn);
                return;
            }

            phaser.register(); // register this connection with the phaser

            executor.submit(() -> {
                try {
                    handleConnection(conn);
                } finally {
                    connectionSlots.release();
                    phaser.arriveAndDeregister();
                }
            });
        }
    }

    /**
     * Handles the full lifecycle of one TCP connection.
     * Runs in a virtual thread. Must catch all Throwables — a bug in any handler
     * must not crash the server process.
     *
     * Java equivalent of Go's handleConnection goroutine.
     */
    private void handleConnection(Socket conn) {
        String remoteAddr = conn.getRemoteSocketAddress().toString();

        try {
            conn.setSoTimeout((int) config.readTimeout().toMillis());

            // Java-specific: we need both a BufferedReader (for text line reading)
            // and a BufferedInputStream (for raw body bytes).
            // Go's bufio.Reader handles both at the byte level cleanly.
            // Here we wrap the same underlying stream — careful not to create two
            // independent buffers over the same socket.
            BufferedInputStream rawStream = new BufferedInputStream(conn.getInputStream());
            BufferedReader reader = new BufferedReader(
                    new InputStreamReader(rawStream, "ISO-8859-1")); // RFC 7230: headers are ASCII
            OutputStream out = conn.getOutputStream();

            log.fine("Connection opened: " + remoteAddr);

            // Keep-alive loop — serve multiple requests per connection
            while (!shutdownRequested.get()) {
                HttpRequest req;
                try {
                    req = HttpParser.parseRequest(reader, rawStream, remoteAddr);
                } catch (EOFException e) {
                    log.fine("Connection closed by client: " + remoteAddr);
                    return;
                } catch (HttpParseException e) {
                    log.warning("Parse error from " + remoteAddr + ": " + e.getMessage());
                    HttpParser.serializeResponse(out, HttpResponse.badRequest(e.getMessage()));
                    return;
                } catch (SocketTimeoutException e) {
                    log.fine("Idle timeout for: " + remoteAddr);
                    return;
                }

                long start = System.currentTimeMillis();
                HttpResponse resp = dispatch(req);
                long duration = System.currentTimeMillis() - start;

                log.info(String.format("method=%s path=%s status=%d duration_ms=%d remote=%s",
                        req.getMethod(), req.getPath(), resp.getStatusCode(), duration, remoteAddr));

                HttpParser.serializeResponse(out, resp);

                // Close connection if client requested it
                String connHeader = req.getHeader("connection");
                if ("close".equalsIgnoreCase(connHeader)) {
                    return;
                }
            }

        } catch (Throwable t) {
            // Catch Throwable (not just Exception) — catches OutOfMemoryError, etc.
            // Log and attempt 500. Equivalent to Go's recover().
            log.log(Level.SEVERE, "Unhandled error for connection " + remoteAddr, t);
            try {
                HttpParser.serializeResponse(
                        conn.getOutputStream(), HttpResponse.internalServerError());
            } catch (IOException ignored) {}
        } finally {
            closeQuietly(conn);
        }
    }

    private HttpResponse dispatch(HttpRequest req) {
        return switch (router.lookup(req.getMethod(), req.getPath())) {
            case Router.LookupResult.Found f       -> f.handler().apply(req);
            case Router.LookupResult.NotFound n    -> HttpResponse.notFound();
            case Router.LookupResult.MethodNotAllowed m -> HttpResponse.methodNotAllowed();
        };
    }

    /**
     * Initiates graceful shutdown:
     * 1. Stop accepting new connections
     * 2. Wait for in-flight requests to complete (up to timeoutMs)
     *
     * Direct equivalent of Go's server.Shutdown(ctx).
     *
     * @param timeoutMs max milliseconds to wait for in-flight requests
     */
    public void shutdown(long timeoutMs) {
        log.info("Shutdown initiated");
        shutdownRequested.set(true);

        // Close server socket — unblocks accept() with SocketException
        closeQuietly(serverSocket);

        executor.shutdown();

        // Wait for all in-flight connections (phaser reaches 0)
        try {
            // Phaser.awaitAdvanceInterruptibly on phase 0
            // Timeout equivalent to Go's context.WithTimeout
            boolean drained = executor.awaitTermination(timeoutMs, TimeUnit.MILLISECONDS);
            if (!drained) {
                log.warning("Shutdown timed out — some connections forcefully closed");
                executor.shutdownNow();
            }
        } catch (InterruptedException e) {
            Thread.currentThread().interrupt();
            executor.shutdownNow();
        }

        log.info("Shutdown complete");
    }

    public int getPort() {
        if (serverSocket == null) return -1;
        return serverSocket.getLocalPort();
    }

    private static void closeQuietly(Closeable c) {
        if (c == null) return;
        try { c.close(); } catch (IOException ignored) {}
    }
}