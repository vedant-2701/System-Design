package server_test

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"http-server/internal/handler"
	"http-server/internal/logger"
	"http-server/internal/parser"
	"http-server/internal/router"
	"http-server/internal/server"
)

// startTestServer spins up a real server on an OS-assigned port.
// Pattern: bind :0 first to get address, then hand listener to server.
// Avoids port conflicts in parallel test runs and CI environments.
func startTestServer(t *testing.T) (addr string, shutdown func()) {
	t.Helper()

	log := logger.New(logger.ERROR)
	r := router.New()
	r.GET("/health", handler.Health)
	r.GET("/hello", handler.Hello)
	r.POST("/echo", handler.Echo)

	cfg := server.DefaultConfig()
	cfg.ReadTimeout = 2 * time.Second
	cfg.WriteTimeout = 2 * time.Second
	cfg.IdleTimeout = 2 * time.Second

	srv := server.New(cfg, r, log)

	ln, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatalf("failed to bind listener: %v", err)
	}
	addr = ln.Addr().String()

	go srv.ServeListener(ln)
	time.Sleep(10 * time.Millisecond)

	shutdown = func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		srv.Shutdown(ctx)
	}
	return addr, shutdown
}

// sendRawHTTP opens a TCP connection and sends a raw HTTP request string.
// Returns the full raw response for assertion.
func sendRawHTTP(t *testing.T, addr, rawRequest string) string {
	t.Helper()
	conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
	if err != nil {
		t.Fatalf("failed to connect to %s: %v", addr, err)
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(3 * time.Second))

	fmt.Fprint(conn, rawRequest)

	var sb strings.Builder
	io.Copy(&sb, conn)
	return sb.String()
}

func clamp(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

// --- Integration tests ---

func TestIntegration_GETHealth(t *testing.T) {
	addr, shutdown := startTestServer(t)
	defer shutdown()

	resp := sendRawHTTP(t, addr,
		"GET /health HTTP/1.1\r\nHost: localhost\r\nConnection: close\r\n\r\n")

	if !strings.HasPrefix(resp, "HTTP/1.1 200 OK") {
		t.Errorf("expected 200 OK, got: %q", clamp(resp, 50))
	}
	if !strings.Contains(resp, `"status"`) {
		t.Errorf("expected JSON body with status field, got: %q", clamp(resp, 200))
	}
}

func TestIntegration_NotFound(t *testing.T) {
	addr, shutdown := startTestServer(t)
	defer shutdown()

	resp := sendRawHTTP(t, addr,
		"GET /nonexistent HTTP/1.1\r\nHost: localhost\r\nConnection: close\r\n\r\n")

	if !strings.HasPrefix(resp, "HTTP/1.1 404") {
		t.Errorf("expected 404, got: %q", clamp(resp, 50))
	}
}

// Path exists (/health is GET-only) but method is wrong — must return 405 not 404.
func TestIntegration_MethodNotAllowed(t *testing.T) {
	addr, shutdown := startTestServer(t)
	defer shutdown()

	resp := sendRawHTTP(t, addr,
		"DELETE /health HTTP/1.1\r\nHost: localhost\r\nConnection: close\r\n\r\n")

	if !strings.HasPrefix(resp, "HTTP/1.1 405") {
		t.Errorf("expected 405, got: %q", clamp(resp, 50))
	}
}

func TestIntegration_MalformedRequest(t *testing.T) {
	addr, shutdown := startTestServer(t)
	defer shutdown()

	resp := sendRawHTTP(t, addr, "THIS IS NOT HTTP\r\n\r\n")

	if !strings.HasPrefix(resp, "HTTP/1.1 400") {
		t.Errorf("expected 400 for malformed request, got: %q", clamp(resp, 50))
	}
}

func TestIntegration_POSTWithBody(t *testing.T) {
	addr, shutdown := startTestServer(t)
	defer shutdown()

	body := `{"hello":"world"}`
	raw := fmt.Sprintf(
		"POST /echo HTTP/1.1\r\nHost: localhost\r\nContent-Type: application/json\r\nContent-Length: %d\r\nConnection: close\r\n\r\n%s",
		len(body), body,
	)

	resp := sendRawHTTP(t, addr, raw)
	if !strings.HasPrefix(resp, "HTTP/1.1 200") {
		t.Errorf("expected 200, got: %q", clamp(resp, 50))
	}
	// Echo handler JSON-encodes the body as a string field — check for its presence.
	if !strings.Contains(resp, "body") {
		t.Errorf("expected echoed body field in response, got: %q", clamp(resp, 200))
	}
}

// TestIntegration_KeepAlive verifies one TCP connection serves multiple requests.
// This validates the keep-alive loop in handleConnection.
func TestIntegration_KeepAlive(t *testing.T) {
	addr, shutdown := startTestServer(t)
	defer shutdown()

	conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(5 * time.Second))

	reader := bufio.NewReader(conn)

	for i := 0; i < 3; i++ {
		fmt.Fprint(conn, "GET /health HTTP/1.1\r\nHost: localhost\r\n\r\n")

		// Read status line
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("request %d: read error: %v", i+1, err)
		}
		if !strings.HasPrefix(line, "HTTP/1.1 200") {
			t.Errorf("request %d: expected 200, got: %q", i+1, line)
		}

		// Drain headers and body before sending next request
		contentLength := 0
		for {
			hdrLine, _ := reader.ReadString('\n')
			hdrLine = strings.TrimRight(hdrLine, "\r\n")
			if hdrLine == "" {
				break
			}
			if strings.HasPrefix(strings.ToLower(hdrLine), "content-length:") {
				fmt.Sscanf(hdrLine, "Content-Length: %d", &contentLength)
			}
		}
		bodyBuf := make([]byte, contentLength)
		io.ReadFull(reader, bodyBuf)
	}
}

// TestIntegration_ConcurrentConnections verifies concurrent connection safety.
// Run with: go test -race ./... to catch data races.
func TestIntegration_ConcurrentConnections(t *testing.T) {
	addr, shutdown := startTestServer(t)
	defer shutdown()

	const numClients = 50
	var wg sync.WaitGroup
	errCh := make(chan error, numClients)

	for i := 0; i < numClients; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			resp := sendRawHTTP(t, addr,
				"GET /health HTTP/1.1\r\nHost: localhost\r\nConnection: close\r\n\r\n")
			if !strings.HasPrefix(resp, "HTTP/1.1 200") {
				errCh <- fmt.Errorf("client %d: expected 200, got %q", id, clamp(resp, 30))
			}
		}(i)
	}

	wg.Wait()
	close(errCh)
	for err := range errCh {
		t.Error(err)
	}
}

// TestIntegration_GracefulShutdown verifies in-flight requests complete
// before the server closes — the Kubernetes SIGTERM scenario.
func TestIntegration_GracefulShutdown(t *testing.T) {
	log := logger.New(logger.ERROR)
	r := router.New()

	// Slow handler simulates an in-flight long-running request
	r.GET("/slow", func(req *parser.Request) *parser.Response {
		time.Sleep(300 * time.Millisecond)
		return parser.OK([]byte("done"), "text/plain")
	})

	cfg := server.DefaultConfig()
	cfg.ReadTimeout = 2 * time.Second
	cfg.WriteTimeout = 2 * time.Second
	cfg.IdleTimeout = 2 * time.Second
	srv := server.New(cfg, r, log)

	ln, _ := net.Listen("tcp", ":0")
	addr := ln.Addr().String()
	go srv.ServeListener(ln)
	time.Sleep(10 * time.Millisecond)

	// Fire the slow request in background
	respCh := make(chan string, 1)
	go func() {
		respCh <- sendRawHTTP(t, addr,
			"GET /slow HTTP/1.1\r\nHost: localhost\r\nConnection: close\r\n\r\n")
	}()

	// Let the request reach the handler, then initiate shutdown
	time.Sleep(50 * time.Millisecond)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	srv.Shutdown(ctx)

	// The in-flight request must complete successfully
	resp := <-respCh
	if !strings.HasPrefix(resp, "HTTP/1.1 200") {
		t.Errorf("in-flight request during shutdown: expected 200, got %q", clamp(resp, 50))
	}
	if !strings.Contains(resp, "done") {
		t.Errorf("expected body 'done', got: %q", clamp(resp, 100))
	}
}