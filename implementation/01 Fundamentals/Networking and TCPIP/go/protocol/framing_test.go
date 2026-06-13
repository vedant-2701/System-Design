package protocol_test

import (
	"bytes"
	"encoding/binary"
	"io"
	"strings"
	"testing"

	"tcp-lab/protocol"
)

// TestWriteReadRoundtrip verifies a message written then read back is identical.
func TestWriteReadRoundtrip(t *testing.T) {
	cases := []struct {
		name    string
		payload string
	}{
		{"simple message", "Hello, World!"},
		{"empty message", ""},
		{"unicode", "こんにちは世界"},
		{"binary-like", "\x00\x01\x02\x03"},
		{"large message", strings.Repeat("A", 512*1024)}, // 512 KB
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer

			if err := protocol.WriteMessage(&buf, []byte(tc.payload)); err != nil {
				t.Fatalf("WriteMessage failed: %v", err)
			}

			got, err := protocol.ReadMessage(&buf)
			if err != nil {
				t.Fatalf("ReadMessage failed: %v", err)
			}

			if string(got) != tc.payload {
				t.Errorf("payload mismatch: got %q, want %q", got, tc.payload)
			}
		})
	}
}

// TestMultipleMessagesInStream verifies that multiple messages written to
// the same stream are read back independently and in order.
// This is the core property TCP framing must provide.
func TestMultipleMessagesInStream(t *testing.T) {
	messages := []string{"first", "second", "third", "fourth"}

	var buf bytes.Buffer
	for _, msg := range messages {
		if err := protocol.WriteMessage(&buf, []byte(msg)); err != nil {
			t.Fatalf("WriteMessage(%q) failed: %v", msg, err)
		}
	}

	for i, expected := range messages {
		got, err := protocol.ReadMessage(&buf)
		if err != nil {
			t.Fatalf("ReadMessage #%d failed: %v", i, err)
		}
		if string(got) != expected {
			t.Errorf("message %d: got %q, want %q", i, got, expected)
		}
	}
}

// TestReadFromEmptyStream verifies that reading from a closed/empty reader
// returns io.EOF (clean disconnect signal), not a panic or confusing error.
func TestReadFromEmptyStream(t *testing.T) {
	_, err := protocol.ReadMessage(strings.NewReader(""))
	if err == nil {
		t.Fatal("expected error reading from empty reader, got nil")
	}
	if !strings.Contains(err.Error(), "EOF") {
		t.Errorf("expected EOF-related error, got: %v", err)
	}
}

// TestPartialHeaderReturnsUnexpectedEOF simulates a connection that drops
// mid-header — the server should get io.ErrUnexpectedEOF, not a panic.
func TestPartialHeaderReturnsUnexpectedEOF(t *testing.T) {
	// Only 2 bytes of header (need 4)
	_, err := protocol.ReadMessage(bytes.NewReader([]byte{0x00, 0x00}))
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "EOF") {
		t.Errorf("expected EOF-related error, got: %v", err)
	}
}

// TestOversizedMessageRejected verifies the server rejects messages claiming
// to be larger than MaxMessageSize — this is the memory exhaustion attack defence.
func TestOversizedMessageRejected(t *testing.T) {
	// Craft a header claiming a 2MB payload (over the 1MB limit)
	oversizeHeader := make([]byte, protocol.HeaderSize)
	binary.BigEndian.PutUint32(oversizeHeader, 2*1024*1024)

	_, err := protocol.ReadMessage(bytes.NewReader(oversizeHeader))
	if err == nil {
		t.Fatal("expected error for oversized message, got nil")
	}
	if !strings.Contains(err.Error(), "exceeds maximum") {
		t.Errorf("expected 'exceeds maximum' error, got: %v", err)
	}
}

// TestWriteOversizedPayloadRejected verifies the writer also enforces the limit.
func TestWriteOversizedPayloadRejected(t *testing.T) {
	oversized := make([]byte, protocol.MaxMessageSize+1)
	var buf bytes.Buffer
	err := protocol.WriteMessage(&buf, oversized)
	if err == nil {
		t.Fatal("expected error writing oversized payload, got nil")
	}
}

// TestReadTruncatedPayload simulates a connection that drops after the header
// but before the full payload is received.
func TestReadTruncatedPayload(t *testing.T) {
	var buf bytes.Buffer

	// Write a valid header claiming 10 bytes
	header := make([]byte, protocol.HeaderSize)
	binary.BigEndian.PutUint32(header, 10)
	buf.Write(header)

	// Write only 5 bytes of payload (truncated)
	buf.Write([]byte("hello"))

	_, err := protocol.ReadMessage(&buf)
	if err == nil {
		t.Fatal("expected error reading truncated payload, got nil")
	}
	// Should be ErrUnexpectedEOF (connection closed mid-payload)
	if !strings.Contains(err.Error(), "EOF") {
		t.Errorf("expected EOF-related error, got: %v", err)
	}
}

// TestWriteToClosedWriter verifies write errors are returned, not swallowed.
func TestWriteToClosedWriter(t *testing.T) {
	err := protocol.WriteMessage(io.Discard, []byte("test"))
	if err != nil {
		t.Errorf("writing to Discard should not fail: %v", err)
	}

	// Writing to a broken pipe-like writer
	pr, pw := io.Pipe()
	_ = pr.Close() // close read end — writes will fail

	err = protocol.WriteMessage(pw, []byte("test"))
	if err == nil {
		t.Fatal("expected error writing to closed pipe, got nil")
	}
}