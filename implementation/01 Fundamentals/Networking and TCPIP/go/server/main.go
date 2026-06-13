// Package main implements a production-oriented TCP echo server.
//
// DESIGN DECISIONS:
//
// 1. One goroutine per connection (not event loop / io_uring)
//    Go goroutines are ~2-8KB vs ~1MB platform threads, making this viable
//    for tens of thousands of concurrent connections. The Go runtime multiplexes
//    goroutines onto OS threads (GOMAXPROCS) using cooperative + preemptive scheduling.
//    For pure I/O-bound workloads like this, goroutine-per-connection is idiomatic Go.
//
// 2. Graceful shutdown via context + WaitGroup
//    On SIGINT/SIGTERM: stop accepting new connections, wait for active
//    handlers to finish (with a deadline). Without WaitGroup, in-flight
//    requests are killed mid-response. Without a deadline, a stuck handler
//    blocks shutdown forever.
//
// 3. TCP keepalive enabled per-connection
//    Default OS keepalive idle time is 2 hours — unacceptable for detecting
//    dead peers. We set it to 30 seconds. This is per-socket configuration,
//    not a global kernel change.
//
// 4. Per-connection read deadline (idle timeout)
//    Keepalive detects dead network. Read deadline catches slow/idle clients
//    that are alive but not sending. Without this, a client that connects and
//    never sends data holds a goroutine open forever.
//
// 5. Structured logging (key=value pairs)
//    Machine-parseable logs enable log aggregation (ELK, Loki).
//    Each log line includes connection ID for tracing a single connection's lifecycle.

package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"tcp-lab/metrics"
	"tcp-lab/protocol"
)

const (
	defaultAddress    = "0.0.0.0:9000"
	readIdleTimeout   = 60 * time.Second  // disconnect if no message received for this long
	keepAliveInterval = 30 * time.Second  // TCP keepalive probe interval
	shutdownTimeout   = 15 * time.Second  // max time to wait for active connections on shutdown
)

// Server manages the TCP listener and active connections.
type Server struct {
	address  string
	logger   *slog.Logger
	listener net.Listener
	metrics  *metrics.ConnectionMetrics

	// wg tracks active connection handlers.
	// Shutdown waits on this before exiting.
	wg sync.WaitGroup
}

func NewServer(address string, logger *slog.Logger) *Server {
	return &Server{
		address: address,
		logger:  logger,
		metrics: metrics.New(),
	}
}

// Run starts the server and blocks until ctx is cancelled.
func (s *Server) Run(ctx context.Context) error {
	var lc net.ListenConfig
	listener, err := lc.Listen(ctx, "tcp", s.address)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", s.address, err)
	}
	s.listener = listener

	s.logger.Info("server started", "address", s.address)

	// Emit metrics snapshot every 10 seconds so you can observe the server
	// state in structured logs. In production, scrape these via Prometheus.
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				snap := s.metrics.Snapshot()
				s.logger.Info("metrics", "snapshot", snap.String())
			}
		}
	}()

	// When context is cancelled (signal received), close the listener.
	// This unblocks the Accept() loop below with a net.ErrClosed error.
	go func() {
		<-ctx.Done()
		s.logger.Info("shutdown signal received, stopping accept loop")
		_ = listener.Close()
	}()

	s.acceptLoop(ctx)

	// Wait for all active connection handlers to finish.
	// Use a timeout so a stuck handler doesn't block shutdown forever.
	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		s.logger.Info("all connections closed, shutdown complete",
			"final_metrics", s.metrics.Snapshot().String())
	case <-time.After(shutdownTimeout):
		s.logger.Warn("shutdown timeout exceeded, forcing exit", "timeout", shutdownTimeout)
	}

	return nil
}

// acceptLoop accepts incoming connections until the listener is closed.
func (s *Server) acceptLoop(ctx context.Context) {
	connID := 0
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			// Distinguish between "we closed the listener" (expected) and real errors.
			if errors.Is(err, net.ErrClosed) {
				return
			}
			// Transient accept errors (e.g. EMFILE - too many open files) should
			// not crash the server. Log and continue; a tight loop here would
			// spike CPU. In production, add a small backoff on repeated errors.
			s.logger.Error("accept error", "err", err)
			continue
		}

		connID++
		id := connID // capture for goroutine closure

		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			s.handleConnection(conn, id)
		}()
	}
}

// handleConnection manages the full lifecycle of a single TCP connection.
// It runs in its own goroutine and is fully isolated from other connections.
func (s *Server) handleConnection(conn net.Conn, id int) {
	remoteAddr := conn.RemoteAddr().String()
	logger := s.logger.With("conn_id", id, "remote_addr", remoteAddr)
	logger.Info("connection accepted")

	s.metrics.OnAccept()

	errored := false
	defer func() {
		_ = conn.Close()
		if errored {
			s.metrics.OnError()
		} else {
			s.metrics.OnClose()
		}
		logger.Info("connection closed")
	}()

	// Enable TCP keepalive on this connection.
	if tc, ok := conn.(*net.TCPConn); ok {
		_ = tc.SetKeepAlive(true)
		_ = tc.SetKeepAlivePeriod(keepAliveInterval)
	}

	for {
		if err := conn.SetReadDeadline(time.Now().Add(readIdleTimeout)); err != nil {
			logger.Error("set read deadline failed", "err", err)
			errored = true
			return
		}

		msg, err := protocol.ReadMessage(conn)
		if err != nil {
			isError := s.handleReadError(logger, err)
			errored = isError
			return
		}

		logger.Info("message received", "bytes", len(msg), "payload", string(msg))

		if err := conn.SetWriteDeadline(time.Now().Add(10 * time.Second)); err != nil {
			logger.Error("set write deadline failed", "err", err)
			errored = true
			return
		}

		if err := protocol.WriteMessage(conn, msg); err != nil {
			logger.Error("write failed", "err", err)
			errored = true
			return
		}

		s.metrics.OnMessage(len(msg), len(msg))
		logger.Info("echo sent", "bytes", len(msg))
	}
}

// handleReadError classifies read errors and logs at the appropriate level.
// Returns true if the disconnect was unexpected (an error), false if clean.
// Not all errors are equal:
//   - EOF / connection reset = normal client disconnect → Info, not an error
//   - Timeout = idle client disconnected → Info, not an error
//   - Other = unexpected → Error
func (s *Server) handleReadError(logger *slog.Logger, err error) (isError bool) {
	var netErr net.Error
	switch {
	case errors.Is(err, io.EOF):
		logger.Info("client disconnected cleanly")
		return false
	case errors.As(err, &netErr) && netErr.Timeout():
		logger.Info("connection idle timeout, closing")
		return false
	case isConnectionReset(err):
		logger.Info("connection reset by peer (client crashed or network drop)")
		return false
	default:
		logger.Error("unexpected read error", "err", err)
		return true
	}
}

// isConnectionReset checks for ECONNRESET — the OS-level signal that the
// remote peer forcibly closed the connection (sent TCP RST instead of FIN).
// This happens when: client process crashes, client calls close() with data
// still unread, or some network devices send RST to break idle connections.
func isConnectionReset(err error) bool {
	return errors.Is(err, syscall.ECONNRESET)
}

func main() {
	// Structured JSON logger — in production, use a log aggregation system.
	// slog is Go's standard structured logging package (Go 1.21+).
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	server := NewServer(defaultAddress, logger)

	// Context cancelled on SIGINT (Ctrl+C) or SIGTERM (container stop / systemd).
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := server.Run(ctx); err != nil {
		logger.Error("server error", "err", err)
		os.Exit(1)
	}
}