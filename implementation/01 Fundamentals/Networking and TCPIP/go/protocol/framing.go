// Package protocol implements length-prefix framing over TCP.
//
// WHY LENGTH-PREFIX FRAMING:
// TCP is a byte stream — it has no concept of message boundaries.
// A single Write("Hello") may be received as multiple Read() calls,
// or multiple writes may be coalesced into one read (Nagle's algorithm).
//
// Length-prefix framing adds a fixed-size header (4 bytes, big-endian uint32)
// before each message payload. The reader reads the header first to know
// exactly how many bytes to expect, then reads exactly that many bytes.
//
// Wire format:
//   [4 bytes: payload length (big-endian uint32)] [N bytes: payload]
//
// Alternative: delimiter framing (\n terminator). Rejected because:
//   - Payloads containing \n require escaping (complexity)
//   - Binary payloads are not safe with delimiter approach
//   - Length-prefix is O(1) to frame/deframe; delimiter is O(n) scan

package protocol

import (
	"encoding/binary"
	"fmt"
	"io"
)

const (
	// HeaderSize is the fixed size of the length-prefix header in bytes.
	HeaderSize = 4

	// MaxMessageSize caps message size to prevent memory exhaustion attacks.
	// A malicious client could send header value = 1GB, causing server to
	// allocate 1GB before reading. Always validate before allocating.
	MaxMessageSize = 1 * 1024 * 1024 // 1 MB
)

// WriteMessage writes a length-prefixed message to the writer.
// Thread-safety: NOT safe for concurrent use on the same writer.
// Callers must synchronize if writing from multiple goroutines.
func WriteMessage(w io.Writer, payload []byte) error {
	size := len(payload)
	if size > MaxMessageSize {
		return fmt.Errorf("payload size %d exceeds maximum %d", size, MaxMessageSize)
	}

	// Write header: 4-byte big-endian length
	header := make([]byte, HeaderSize)
	binary.BigEndian.PutUint32(header, uint32(size))

	// Write header + payload as two separate writes.
	// NOTE: This can result in two TCP segments (header in one, payload in another).
	// In production, use a bufio.Writer and flush once, or writev syscall,
	// to coalesce into a single segment and reduce small-packet overhead.
	if _, err := w.Write(header); err != nil {
		return fmt.Errorf("writing header: %w", err)
	}
	if _, err := w.Write(payload); err != nil {
		return fmt.Errorf("writing payload: %w", err)
	}
	return nil
}

// ReadMessage reads a length-prefixed message from the reader.
// Blocks until a complete message is available or an error occurs.
//
// Error handling:
//   - io.EOF: clean connection close (client disconnected gracefully)
//   - io.ErrUnexpectedEOF: connection closed mid-message (client crashed or network failure)
//   - Other errors: network error, timeout, etc.
func ReadMessage(r io.Reader) ([]byte, error) {
	// Step 1: read exactly 4 bytes for the header.
	// io.ReadFull handles partial reads internally — keeps reading until
	// exactly n bytes are read or an error occurs.
	// This is critical: a naive Read() may return fewer than 4 bytes.
	header := make([]byte, HeaderSize)
	if _, err := io.ReadFull(r, header); err != nil {
		// io.EOF here means the connection closed cleanly before any message bytes.
		// io.ErrUnexpectedEOF means it closed mid-header (crash/network drop).
		return nil, fmt.Errorf("reading header: %w", err)
	}

	// Step 2: parse the payload length from the header.
	size := binary.BigEndian.Uint32(header)

	// Step 3: validate size before allocating.
	// Without this check, a malicious client sends header=0xFFFFFFFF (~4GB)
	// and the server allocates 4GB before reading a single payload byte.
	if size > MaxMessageSize {
		return nil, fmt.Errorf("message size %d exceeds maximum %d: possible malicious client", size, MaxMessageSize)
	}

	// Step 4: allocate exactly the right buffer and read payload.
	payload := make([]byte, size)
	if _, err := io.ReadFull(r, payload); err != nil {
		return nil, fmt.Errorf("reading payload of %d bytes: %w", size, err)
	}

	return payload, nil
}