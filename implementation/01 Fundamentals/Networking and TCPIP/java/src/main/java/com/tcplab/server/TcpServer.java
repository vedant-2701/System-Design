package com.tcplab.server;

import com.tcplab.protocol.MessageFramer;

import java.io.DataInputStream;
import java.io.DataOutputStream;
import java.io.EOFException;
import java.io.IOException;
import java.net.ServerSocket;
import java.net.Socket;
import java.net.SocketException;
import java.net.SocketTimeoutException;
import java.time.Duration;
import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;
import java.util.concurrent.TimeUnit;
import java.util.concurrent.atomic.AtomicInteger;
import java.util.logging.Level;
import java.util.logging.Logger;

/**
 * Production-oriented TCP echo server using Java virtual threads (Project Loom).
 *
 * <p>DESIGN DECISION — Virtual Threads vs Platform Threads:
 *
 * <p>Platform threads (classic):
 * - Each thread = 1 OS thread = ~1MB stack
 * - 10,000 connections = 10 GB of stack memory
 * - Context switching between OS threads is expensive (kernel mode switch)
 * - Thread pool size must be tuned manually (too small = queuing, too large = memory pressure)
 *
 * <p>Virtual threads (Java 21+):
 * - Multiplexed onto a small pool of carrier/platform threads by the JVM
 * - ~1KB overhead per virtual thread vs ~1MB per platform thread
 * - JVM automatically parks virtual threads on blocking I/O
 * - Thread-per-connection becomes viable for tens of thousands of connections
 * - CAUTION: synchronized blocks pin virtual thread to carrier thread —
 *   avoid synchronized in hot paths; use ReentrantLock instead
 *
 * <p>DESIGN DECISION — newVirtualThreadPerTaskExecutor():
 * Creates an unbounded executor where each task gets a new virtual thread.
 * "Unbounded" is safe here because virtual threads are cheap — unlike platform
 * thread pools where unbounded = OOM. The JVM carrier pool is bounded by GOMAXPROCS equivalent.
 *
 * <p>ALTERNATIVE CONSIDERED — Netty / NIO event loop:
 * Non-blocking I/O with an event loop avoids thread-per-connection entirely.
 * Rejected for this implementation because:
 * - Virtual threads achieve similar scalability with blocking I/O semantics
 * - Blocking code is far simpler to read, debug, and test
 * - Netty's callback model adds significant complexity
 * - For a learning exercise, virtual threads demonstrate the same concepts more clearly
 */
public class TcpServer {

    private static final Logger LOG = Logger.getLogger(TcpServer.class.getName());

    private static final int DEFAULT_PORT = 9000;
    private static final int READ_IDLE_TIMEOUT_MS = (int) Duration.ofSeconds(60).toMillis();
    private static final int KEEPALIVE_IDLE_SECONDS = 30;
    private static final int SHUTDOWN_TIMEOUT_SECONDS = 15;

    private final int port;
    private final AtomicInteger connectionCounter = new AtomicInteger(0);

    // volatile ensures the stop flag is visible across threads without synchronization overhead.
    // Using an AtomicBoolean would also be correct but volatile bool is sufficient here
    // since we only ever flip it from false→true (no compare-and-swap needed).
    private volatile boolean running = true;
    private ServerSocket serverSocket;
    private ExecutorService executor;

    public TcpServer(int port) {
        this.port = port;
    }

    public void start() throws IOException {
        // Virtual thread executor — each accepted connection gets its own virtual thread.
        // Shutdown via executor.shutdown() + awaitTermination() for graceful drain.
        executor = Executors.newVirtualThreadPerTaskExecutor();

        serverSocket = new ServerSocket(port);

        // SO_REUSEADDR: allows binding to a port that is in TIME_WAIT state.
        // Critical after server crash/restart — without this, binding fails with
        // "Address already in use" for up to 60 seconds.
        serverSocket.setReuseAddress(true);

        LOG.info(String.format("Server listening on port %d (virtual threads enabled)", port));

        // Register shutdown hook for clean JVM exit (Ctrl+C, kill signal).
        // Equivalent to signal.NotifyContext in Go.
        Runtime.getRuntime().addShutdownHook(new Thread(this::shutdown));

        acceptLoop();
    }

    private void acceptLoop() {
        while (running) {
            try {
                Socket socket = serverSocket.accept();
                int connId = connectionCounter.incrementAndGet();

                // Submit to virtual thread executor — non-blocking, returns immediately.
                executor.submit(() -> handleConnection(socket, connId));

            } catch (SocketException e) {
                // serverSocket.close() was called during shutdown — expected path.
                if (!running) {
                    LOG.info("Accept loop stopped (server shutting down)");
                    return;
                }
                // Transient accept error — log but continue accepting.
                // In production, add a backoff here if errors are sustained.
                LOG.log(Level.WARNING, "Accept error (transient)", e);
            } catch (IOException e) {
                if (!running) return;
                LOG.log(Level.SEVERE, "Unexpected accept error", e);
            }
        }
    }

    private void handleConnection(Socket socket, int connId) {
        String remoteAddr = socket.getRemoteSocketAddress().toString();
        LOG.info(String.format("[conn=%d] Accepted from %s", connId, remoteAddr));

        try (socket) { // try-with-resources ensures socket.close() even on exception
            configureSocket(socket);

            var framer = new MessageFramer(
                new DataInputStream(socket.getInputStream()),
                new DataOutputStream(socket.getOutputStream())
            );

            echoLoop(framer, connId, remoteAddr);

        } catch (IOException e) {
            // IOException from try-with-resources close() — usually benign.
            LOG.log(Level.FINE, String.format("[conn=%d] Error closing socket", connId), e);
        }

        LOG.info(String.format("[conn=%d] Connection closed (%s)", connId, remoteAddr));
    }

    private void echoLoop(MessageFramer framer, int connId, String remoteAddr) {
        while (true) {
            try {
                byte[] message = framer.readMessage();

                LOG.info(String.format("[conn=%d] Received %d bytes: %s",
                    connId, message.length, new String(message)));

                framer.writeMessage(message);

                LOG.info(String.format("[conn=%d] Echo sent %d bytes", connId, message.length));

            } catch (EOFException e) {
                // Clean client disconnect — FIN received, EOF on stream.
                LOG.info(String.format("[conn=%d] Client disconnected cleanly", connId));
                return;

            } catch (SocketTimeoutException e) {
                // Read deadline expired — client is idle.
                LOG.info(String.format("[conn=%d] Idle timeout, closing connection", connId));
                return;

            } catch (SocketException e) {
                // Connection reset by peer (RST) — client crashed or network drop.
                // SocketException message will contain "Connection reset".
                if (e.getMessage() != null && e.getMessage().contains("reset")) {
                    LOG.info(String.format("[conn=%d] Connection reset by peer", connId));
                } else {
                    LOG.warning(String.format("[conn=%d] Socket error: %s", connId, e.getMessage()));
                }
                return;

            } catch (IOException e) {
                // Oversized message, framing error, or other unexpected error.
                LOG.log(Level.WARNING,
                    String.format("[conn=%d] Read/write error", connId), e);
                return;
            }
        }
    }

    private void configureSocket(Socket socket) throws SocketException {
        // SO_KEEPALIVE: instruct OS to send TCP keepalive probes on this socket.
        // Detects dead peers (unplugged cable, crashed remote host).
        // Note: the per-socket keepalive idle time config requires JNI or JVM flags on Java <19.
        // On Java 21 + Linux, use ExtendedSocketOptions.TCP_KEEPIDLE.
        socket.setKeepAlive(true);

        // SO_TIMEOUT: causes read() to throw SocketTimeoutException after this
        // many milliseconds of waiting. This is our idle timeout mechanism.
        // Without this, a connected-but-silent client holds a virtual thread indefinitely.
        socket.setSoTimeout(READ_IDLE_TIMEOUT_MS);

        // TCP_NODELAY: disable Nagle's algorithm. Nagle coalesces small packets
        // to reduce overhead — it waits up to 200ms for more data before sending.
        // For an interactive echo protocol, this adds unacceptable latency.
        // Disable for request-response protocols; enable for bulk streaming.
        socket.setTcpNoDelay(true);
    }

    public void shutdown() {
        LOG.info("Shutdown initiated...");
        running = false;

        try {
            if (serverSocket != null && !serverSocket.isClosed()) {
                serverSocket.close(); // unblocks accept()
            }
        } catch (IOException e) {
            LOG.log(Level.WARNING, "Error closing server socket", e);
        }

        if (executor != null) {
            executor.shutdown();
            try {
                if (!executor.awaitTermination(SHUTDOWN_TIMEOUT_SECONDS, TimeUnit.SECONDS)) {
                    LOG.warning("Shutdown timeout exceeded, forcing remaining connections closed");
                    executor.shutdownNow();
                }
            } catch (InterruptedException e) {
                executor.shutdownNow();
                Thread.currentThread().interrupt();
            }
        }

        LOG.info("Server shutdown complete");
    }

    public static void main(String[] args) throws IOException {
        new TcpServer(DEFAULT_PORT).start();
    }
}