package server

import (
	"bufio"
	"context"
	// "crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"http-server/internal/logger"
	"http-server/internal/parser"
	"http-server/internal/router"
)

// Config holds all tunable server parameters.
// Centralizing config prevents magic numbers scattered across the codebase.
type Config struct {
	Addr string

	// ReadTimeout: max time to read the full request (headers + body).
	// Prevents slow-loris attacks — clients that send headers one byte at a time.
	ReadTimeout time.Duration

	// WriteTimeout: max time to write the full response.
	// Prevents holding connections open for unresponsive clients.
	WriteTimeout time.Duration

	// IdleTimeout: max time to wait for the next request on a keep-alive connection.
	// Should be slightly less than upstream's keep-alive timeout to avoid race conditions.
	IdleTimeout time.Duration

	// MaxConnections: cap on simultaneous connections.
	// Without this, a connection flood exhausts goroutine memory.
	MaxConnections int
}

func DefaultConfig() Config {
	return Config{
		Addr:           ":8080",
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		IdleTimeout:    60 * time.Second,
		MaxConnections: 1000,
	}
}

// Server is the top-level type owning the TCP listener and connection lifecycle.
type Server struct {
	config Config
	router *router.Router
	log    *logger.Logger

	// mu protects listener — written by ServeListener, read by Shutdown,
	// both potentially called from different goroutines.
	mu       sync.Mutex
	listener net.Listener

	// wg tracks active connection goroutines for graceful shutdown.
	// Shutdown waits on wg.Wait() before returning.
	wg sync.WaitGroup

	// connSemaphore limits concurrent connections.
	// Buffered channel used as a counting semaphore — idiomatic Go pattern.
	connSemaphore chan struct{}

	// shutdown signals all goroutines to stop accepting new connections.
	shutdown chan struct{}
}

func New(cfg Config, r *router.Router, log *logger.Logger) *Server {
	return &Server{
		config:        cfg,
		router:        r,
		log:           log,
		connSemaphore: make(chan struct{}, cfg.MaxConnections),
		shutdown:      make(chan struct{}),
	}
}

// Start binds the TCP listener and begins accepting connections.
// Blocks until the listener is closed (via Shutdown).
func (s *Server) Start() error {
	// TLS is transparent to your HTTP parsing layer
	// cert, _ := tls.LoadX509KeyPair("cert.pem", "key.pem")
	// cfg := &tls.Config{Certificates: []tls.Certificate{cert}}
	// ln, err := tls.Listen("tcp", ":8443", cfg)
	ln, err := net.Listen("tcp", s.config.Addr)
	if err != nil {
		return fmt.Errorf("binding to %s: %w", s.config.Addr, err)
	}
	return s.ServeListener(ln)
}

// ServeListener accepts connections on an already-bound listener.
// Separated from Start so tests can pre-bind on :0 (OS-assigned port),
// extract the actual address, then hand the listener to the server.
// This is the standard Go pattern for testable servers.
func (s *Server) ServeListener(ln net.Listener) error {
	s.mu.Lock()
	s.listener = ln
	s.mu.Unlock()

	s.log.Info("server started", map[string]any{
		"addr":            ln.Addr().String(),
		"max_connections": s.config.MaxConnections,
		"read_timeout":    s.config.ReadTimeout,
		"write_timeout":   s.config.WriteTimeout,
	})

	return s.acceptLoop()
}

// acceptLoop is the main loop: accept connection → acquire semaphore → spawn goroutine.
func (s *Server) acceptLoop() error {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			// net.ErrClosed is expected during graceful shutdown.
			// Any other error is unexpected — log it and stop.
			if errors.Is(err, net.ErrClosed) {
				return nil
			}
			s.log.Error("accept error", map[string]any{"error": err})
			return err
		}

		// Acquire semaphore slot — blocks if at MaxConnections.
		// This is backpressure: instead of spawning unlimited goroutines,
		// we apply pressure at the accept layer.
		select {
		case s.connSemaphore <- struct{}{}:
			// slot acquired, proceed
		case <-s.shutdown:
			conn.Close()
			return nil
		}

		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			defer func() { <-s.connSemaphore }() // release slot when done
			s.handleConnection(conn)
		}()
	}
}

// handleConnection owns the full lifecycle of one TCP connection.
// Runs in its own goroutine. Must recover from panics — a bug in one handler
// must not crash the server process.
func (s *Server) handleConnection(conn net.Conn) {
	// Panic recovery: catch any handler panic, log it, return 500.
	// In production you'd also emit a metric here (panic_count counter).
	defer func() {
		if r := recover(); r != nil {
			s.log.Error("panic in connection handler", map[string]any{
				"remote": conn.RemoteAddr().String(),
				"panic":  fmt.Sprintf("%v", r),
			})
			// Best-effort 500 after panic — connection state may be corrupted.
			resp := parser.InternalServerError()
			_ = parser.SerializeResponse(conn, resp)
		}
		conn.Close()
	}()

	remoteAddr := conn.RemoteAddr().String()
	s.log.Debug("connection opened", map[string]any{"remote": remoteAddr})

	reader := bufio.NewReader(conn)

	// HTTP/1.1 keep-alive: one connection can serve multiple sequential requests.
	// Loop until the connection is closed by either side.
	for {
		// IdleTimeout applies while waiting for the next request to arrive.
		// This prevents idle connections from holding resources indefinitely.
		if err := conn.SetReadDeadline(time.Now().Add(s.config.IdleTimeout)); err != nil {
			return
		}

		req, err := parser.ParseRequest(reader)
		if err != nil {
			if isConnectionClosed(err) {
				// Normal: client closed the connection cleanly.
				s.log.Debug("connection closed by client", map[string]any{"remote": remoteAddr})
				return
			}
			// Bad request: parse failed. Return 400 and close connection.
			s.log.Warn("request parse error", map[string]any{
				"remote": remoteAddr,
				"error":  err,
			})
			resp := parser.BadRequest(err.Error())
			_ = parser.SerializeResponse(conn, resp)
			return
		}

		req.RemoteAddr = remoteAddr

		// ReadTimeout applies to reading the request body.
		// Now that we have the request, switch to WriteTimeout for the response.
		if err := conn.SetWriteDeadline(time.Now().Add(s.config.WriteTimeout)); err != nil {
			return
		}

		start := time.Now()
		resp := s.dispatch(req)
		duration := time.Since(start)

		// Structured access log: one line per request, always.
		// This is what feeds dashboards, alerting, and post-incident analysis.
		s.log.Info("request", map[string]any{
			"method":     req.Method,
			"path":       req.Path,
			"status":     resp.StatusCode,
			"duration_ms": duration.Milliseconds(),
			"remote":     remoteAddr,
		})

		if err := parser.SerializeResponse(conn, resp); err != nil {
			s.log.Warn("response write error", map[string]any{
				"remote": remoteAddr,
				"error":  err,
			})
			return
		}

		// HTTP/1.1 default is keep-alive. Close only when explicitly requested.
		// Design note: we don't implement Connection: keep-alive negotiation fully here.
		// A production server would also handle HTTP/1.0 (default close) and
		// respect the Connection header value from the request.
		if req.Headers["connection"] == "close" {
			return
		}
	}
}

// dispatch routes the request and executes the handler.
// Separated from handleConnection so it can be unit tested independently.
func (s *Server) dispatch(req *parser.Request) *parser.Response {
	handler, errResp := s.router.Lookup(req.Method, req.Path)
	if errResp != nil {
		return errResp
	}
	return handler(req)
}

// Shutdown performs graceful shutdown:
// 1. Stop accepting new connections (close listener)
// 2. Signal goroutines to exit when idle
// 3. Wait for in-flight requests to complete (with context deadline)
//
// This is what Kubernetes sends SIGTERM and waits for — your server must do this
// or connections are killed mid-response during deployments.
func (s *Server) Shutdown(ctx context.Context) error {
	s.log.Info("shutdown initiated", nil)

	// Signal accept loop to stop
	close(s.shutdown)

	// Close listener — unblocks Accept() with net.ErrClosed
	s.mu.Lock()
	ln := s.listener
	s.mu.Unlock()
	if ln != nil {
		ln.Close()
	}

	// Wait for all connection goroutines to finish, respecting context deadline.
	// If ctx expires before all connections drain, we return an error.
	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		s.log.Info("shutdown complete", nil)
		return nil
	case <-ctx.Done():
		s.log.Warn("shutdown timed out — connections forcefully closed", nil)
		return ctx.Err()
	}
}

// isConnectionClosed identifies expected EOF/reset errors that indicate
// the client closed the connection — not bugs.
func isConnectionClosed(err error) bool {
	if errors.Is(err, io.EOF) {
		return true
	}
	// net.OpError with "use of closed network connection" or "connection reset by peer"
	var netErr *net.OpError
	if errors.As(err, &netErr) {
		return true
	}
	return false
}