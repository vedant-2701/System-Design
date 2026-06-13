// Package main implements a TCP echo client with reconnect logic.
//
// DESIGN DECISIONS:
//
// 1. Reconnect with exponential backoff
//    Networks are unreliable. A client that exits on first disconnect
//    is not production-ready. Exponential backoff prevents retry storms
//    — if the server is down, 1000 clients retrying every 100ms floods
//    the server the moment it comes back. Jitter (randomized delay)
//    further spreads reconnect attempts.
//
// 2. Separate send/receive goroutines NOT used here
//    For a simple request-response echo protocol, a single goroutine
//    that sends then reads is simpler and correct. Separate goroutines
//    would be needed for a bidirectional streaming protocol where server
//    can push messages independent of client requests.
//
// 3. Write deadline on every send
//    Without a write deadline, a write to a broken connection blocks
//    forever (the kernel buffers fill, then write blocks waiting for ACKs
//    that never come). Always bound writes with a deadline.

package main

import (
	"bufio"
	"context"
	"fmt"
	"log/slog"
	"math"
	"math/rand"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"tcp-lab/protocol"
)

const (
	serverAddress  = "127.0.0.1:9000"
	writeTimeout   = 10 * time.Second
	readTimeout    = 15 * time.Second
	minRetryDelay  = 500 * time.Millisecond
	maxRetryDelay  = 30 * time.Second
	maxRetries     = 10
)

// Client manages a TCP connection to the server.
type Client struct {
	address string
	logger  *slog.Logger
}

func NewClient(address string, logger *slog.Logger) *Client {
	return &Client{address: address, logger: logger}
}

// Connect establishes a TCP connection with exponential backoff retries.
// Returns the connection or an error if max retries are exceeded.
func (c *Client) Connect(ctx context.Context) (net.Conn, error) {
	var attempt int
	for {
		attempt++
		c.logger.Info("connecting", "address", c.address, "attempt", attempt)

		conn, err := net.DialTimeout("tcp", c.address, 5*time.Second)
		if err == nil {
			c.logger.Info("connected", "remote_addr", conn.RemoteAddr())
			return conn, nil
		}

		c.logger.Warn("connection failed", "err", err, "attempt", attempt)

		if attempt >= maxRetries {
			return nil, fmt.Errorf("failed to connect after %d attempts: %w", maxRetries, err)
		}

		// Exponential backoff with jitter.
		// Base delay: minRetryDelay * 2^attempt, capped at maxRetryDelay.
		// Jitter: ±25% of the base delay, so multiple clients don't retry in lockstep.
		baseDelay := time.Duration(math.Min(
			float64(minRetryDelay)*math.Pow(2, float64(attempt-1)),
			float64(maxRetryDelay),
		))
		jitter := time.Duration(rand.Int63n(int64(baseDelay) / 2))
		delay := baseDelay + jitter

		c.logger.Info("retrying after backoff", "delay", delay)

		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("context cancelled during reconnect: %w", ctx.Err())
		case <-time.After(delay):
		}
	}
}

// SendAndReceive sends a message and waits for the echo response.
func (c *Client) SendAndReceive(conn net.Conn, msg []byte) ([]byte, error) {
	// Set write deadline before sending.
	if err := conn.SetWriteDeadline(time.Now().Add(writeTimeout)); err != nil {
		return nil, fmt.Errorf("set write deadline: %w", err)
	}

	if err := protocol.WriteMessage(conn, msg); err != nil {
		return nil, fmt.Errorf("send: %w", err)
	}

	// Set read deadline before waiting for echo.
	if err := conn.SetReadDeadline(time.Now().Add(readTimeout)); err != nil {
		return nil, fmt.Errorf("set read deadline: %w", err)
	}

	response, err := protocol.ReadMessage(conn)
	if err != nil {
		return nil, fmt.Errorf("receive: %w", err)
	}

	return response, nil
}

// Run is the main interaction loop: reads from stdin, sends to server, prints response.
func (c *Client) Run(ctx context.Context) error {
	conn, err := c.Connect(ctx)
	if err != nil {
		return err
	}
	defer func() {
		_ = conn.Close()
		c.logger.Info("connection closed")
	}()

	scanner := bufio.NewScanner(os.Stdin)
	fmt.Println("Connected. Type messages and press Enter. Ctrl+C to quit.")

	// Run scanner in a separate goroutine so we can also listen for ctx cancellation.
	type scanResult struct {
		text string
		ok   bool
	}
	scanCh := make(chan scanResult, 1)

	go func() {
		for scanner.Scan() {
			scanCh <- scanResult{text: scanner.Text(), ok: true}
		}
		scanCh <- scanResult{ok: false}
	}()

	for {
		select {
		case <-ctx.Done():
			c.logger.Info("shutdown signal received")
			return nil

		case result := <-scanCh:
			if !result.ok {
				c.logger.Info("stdin closed")
				return nil
			}

			if result.text == "" {
				continue
			}

			start := time.Now()
			response, err := c.SendAndReceive(conn, []byte(result.text))
			if err != nil {
				c.logger.Error("send/receive failed", "err", err)
				// Connection is broken. In a production client, attempt reconnect here.
				return fmt.Errorf("connection error: %w", err)
			}

			rtt := time.Since(start)
			fmt.Printf("Echo [%s] rtt=%s\n", string(response), rtt)
			c.logger.Info("round trip complete",
				"sent_bytes", len(result.text),
				"rtt", rtt,
			)
		}
	}
}

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	client := NewClient(serverAddress, logger)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := client.Run(ctx); err != nil {
		logger.Error("client error", "err", err)
		os.Exit(1)
	}
}