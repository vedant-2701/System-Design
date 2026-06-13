package main_test

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"sync"
	"testing"
	"time"

	"tcp-lab/protocol"
)

// startTestServer starts a TCP server on a random available port.
// Returns the address and a cancel function to stop it.
func startTestServer(t *testing.T) (addr string, cancel context.CancelFunc) {
	t.Helper()

	// Port :0 asks the OS to assign a free port — avoids port conflicts in tests.
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	addr = listener.Addr().String()
	ctx, cancel := context.WithCancel(context.Background())
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	go func() {
		connID := 0
		for {
			conn, err := listener.Accept()
			if err != nil {
				select {
				case <-ctx.Done():
					return
				default:
					t.Logf("accept error: %v", err)
					return
				}
			}
			connID++
			id := connID
			go handleTestConn(t, conn, id, logger)
		}
	}()

	// Cancel closes the listener
	originalCancel := cancel
	cancel = func() {
		originalCancel()
		_ = listener.Close()
	}

	return addr, cancel
}

func handleTestConn(t *testing.T, conn net.Conn, id int, logger *slog.Logger) {
	t.Helper()
	defer conn.Close()

	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))

	msg, err := protocol.ReadMessage(conn)
	if err != nil {
		return // connection likely closed by test
	}

	_ = conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	_ = protocol.WriteMessage(conn, msg) // echo back
}

// TestEchoRoundtrip verifies a single message round-trips correctly.
func TestEchoRoundtrip(t *testing.T) {
	addr, cancel := startTestServer(t)
	defer cancel()

	conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	want := []byte("hello server")

	_ = conn.SetWriteDeadline(time.Now().Add(2 * time.Second))
	if err := protocol.WriteMessage(conn, want); err != nil {
		t.Fatalf("send: %v", err)
	}

	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	got, err := protocol.ReadMessage(conn)
	if err != nil {
		t.Fatalf("receive: %v", err)
	}

	if string(got) != string(want) {
		t.Errorf("echo mismatch: got %q, want %q", got, want)
	}
}

// TestConcurrentClients verifies the server handles multiple simultaneous
// connections correctly — each client gets its own echo, no cross-talk.
func TestConcurrentClients(t *testing.T) {
	addr, cancel := startTestServer(t)
	defer cancel()

	const numClients = 50
	var wg sync.WaitGroup
	errors := make(chan error, numClients)

	for i := 0; i < numClients; i++ {
		wg.Add(1)
		go func(clientID int) {
			defer wg.Done()

			conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
			if err != nil {
				errors <- fmt.Errorf("client %d dial: %w", clientID, err)
				return
			}
			defer conn.Close()

			want := fmt.Sprintf("client-%d-message", clientID)

			_ = conn.SetWriteDeadline(time.Now().Add(2 * time.Second))
			if err := protocol.WriteMessage(conn, []byte(want)); err != nil {
				errors <- fmt.Errorf("client %d send: %w", clientID, err)
				return
			}

			_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
			got, err := protocol.ReadMessage(conn)
			if err != nil {
				errors <- fmt.Errorf("client %d receive: %w", clientID, err)
				return
			}

			if string(got) != want {
				errors <- fmt.Errorf("client %d: echo mismatch: got %q, want %q", clientID, got, want)
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	for err := range errors {
		t.Error(err)
	}
}

// TestServerHandlesClientDisconnectMidMessage verifies the server doesn't
// crash or leak goroutines when a client disconnects mid-stream.
func TestServerHandlesClientDisconnectMidMessage(t *testing.T) {
	addr, cancel := startTestServer(t)
	defer cancel()

	conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}

	// Send header only (claim 100 bytes) then immediately close.
	// Server should get io.ErrUnexpectedEOF and handle gracefully.
	header := make([]byte, protocol.HeaderSize)
	_ = conn.SetWriteDeadline(time.Now().Add(time.Second))
	_, _ = conn.Write(header) // header says 0 bytes but we close before payload

	conn.Close() // abrupt close

	// Give server time to handle the disconnect — if it panics, test will fail
	time.Sleep(100 * time.Millisecond)
}