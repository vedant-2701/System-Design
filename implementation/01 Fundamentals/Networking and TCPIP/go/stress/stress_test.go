package stress_test

import (
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"tcp-lab/protocol"
)

// startEchoServer starts a minimal echo server for stress testing.
// Returns the listener address.
func startEchoServer(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return // listener closed
			}
			go func(c net.Conn) {
				defer c.Close()
				_ = c.SetReadDeadline(time.Now().Add(10 * time.Second))
				for {
					msg, err := protocol.ReadMessage(c)
					if err != nil {
						return
					}
					_ = c.SetWriteDeadline(time.Now().Add(5 * time.Second))
					if err := protocol.WriteMessage(c, msg); err != nil {
						return
					}
				}
			}(conn)
		}
	}()

	t.Cleanup(func() { ln.Close() })
	return ln.Addr().String()
}

// TestStress_HighConcurrentConnections validates that the server handles
// many simultaneous connections without panicking, deadlocking, or
// producing incorrect echoes.
//
// This test catches:
//   - Goroutine leaks (run with go test -v and check goroutine count)
//   - Data races (run with -race flag)
//   - Cross-connection data contamination (each client verifies its own echo)
//   - Resource exhaustion (file descriptors, memory)
func TestStress_HighConcurrentConnections(t *testing.T) {
	addr := startEchoServer(t)

	const (
		numClients      = 200          // concurrent connections
		messagesPerConn = 10           // messages per connection
		timeout         = 15 * time.Second
	)

	var (
		wg           sync.WaitGroup
		successCount atomic.Int64
		errorCount   atomic.Int64
	)

	start := time.Now()

	for i := 0; i < numClients; i++ {
		wg.Add(1)
		clientID := i
		go func() {
			defer wg.Done()

			conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
			if err != nil {
				t.Logf("client %d: dial failed: %v", clientID, err)
				errorCount.Add(1)
				return
			}
			defer conn.Close()

			for j := 0; j < messagesPerConn; j++ {
				// Each message is unique to this client+message — detects cross-connection contamination
				want := fmt.Sprintf("client-%d-msg-%d", clientID, j)

				_ = conn.SetWriteDeadline(time.Now().Add(2 * time.Second))
				if err := protocol.WriteMessage(conn, []byte(want)); err != nil {
					t.Logf("client %d msg %d: write failed: %v", clientID, j, err)
					errorCount.Add(1)
					return
				}

				_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
				got, err := protocol.ReadMessage(conn)
				if err != nil {
					t.Logf("client %d msg %d: read failed: %v", clientID, j, err)
					errorCount.Add(1)
					return
				}

				if string(got) != want {
					t.Errorf("client %d msg %d: data corruption: got %q, want %q",
						clientID, j, got, want)
					errorCount.Add(1)
					return
				}

				successCount.Add(1)
			}
		}()
	}

	// Use a channel to detect if wg.Wait() finishes before timeout
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// All goroutines finished
	case <-time.After(timeout):
		t.Fatalf("stress test timed out after %s — possible goroutine leak or deadlock", timeout)
	}

	elapsed := time.Since(start)
	total := successCount.Load()
	errors := errorCount.Load()
	throughput := float64(total) / elapsed.Seconds()

	t.Logf("Stress test complete in %s", elapsed.Round(time.Millisecond))
	t.Logf("  Successful messages: %d / %d", total, numClients*messagesPerConn)
	t.Logf("  Errors:              %d", errors)
	t.Logf("  Throughput:          %.0f messages/sec", throughput)

	if errors > 0 {
		t.Errorf("stress test had %d errors (see logs above)", errors)
	}

	expectedTotal := int64(numClients * messagesPerConn)
	if total != expectedTotal {
		t.Errorf("expected %d successful messages, got %d", expectedTotal, total)
	}
}

// TestStress_RapidConnectDisconnect validates that the server handles
// clients that connect and immediately disconnect without sending data.
// This is the pattern that causes CLOSE_WAIT accumulation if the server
// doesn't handle EOF correctly.
func TestStress_RapidConnectDisconnect(t *testing.T) {
	addr := startEchoServer(t)

	const numClients = 500
	var wg sync.WaitGroup

	for i := 0; i < numClients; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
			if err != nil {
				return
			}
			// Immediately close without sending — server should handle EOF cleanly
			conn.Close()
		}()
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// success
	case <-time.After(10 * time.Second):
		t.Fatal("rapid connect/disconnect test timed out — possible goroutine leak")
	}

	// Give server goroutines time to clean up
	time.Sleep(100 * time.Millisecond)
	// If we reach here without timeout or panic, the test passes.
	// In a full production test, you'd also check goroutine count via runtime.NumGoroutine()
}

// TestStress_LargeMessages validates behavior with messages near the size limit.
func TestStress_LargeMessages(t *testing.T) {
	addr := startEchoServer(t)

	conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	sizes := []int{
		1024,              // 1 KB
		64 * 1024,         // 64 KB
		512 * 1024,        // 512 KB
		protocol.MaxMessageSize, // exactly at limit
	}

	for _, size := range sizes {
		payload := make([]byte, size)
		for i := range payload {
			payload[i] = byte(i % 256)
		}

		_ = conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
		if err := protocol.WriteMessage(conn, payload); err != nil {
			t.Fatalf("write %d bytes: %v", size, err)
		}

		_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
		got, err := protocol.ReadMessage(conn)
		if err != nil {
			t.Fatalf("read %d bytes echo: %v", size, err)
		}

		if len(got) != size {
			t.Errorf("size %d: got %d bytes back", size, len(got))
		}

		// Spot check a few bytes to detect corruption
		for _, idx := range []int{0, size / 2, size - 1} {
			if got[idx] != payload[idx] {
				t.Errorf("size %d: byte[%d] corrupted: got %d want %d",
					size, idx, got[idx], payload[idx])
			}
		}

		t.Logf("size=%d bytes: OK", size)
	}
}