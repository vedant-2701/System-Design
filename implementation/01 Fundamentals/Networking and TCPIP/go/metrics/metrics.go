// Package metrics provides lightweight connection-level instrumentation.
//
// WHY THIS EXISTS:
// A TCP server without metrics is a black box in production.
// When things go wrong, you need to answer:
//   - How many active connections right now?
//   - How many connections have we accepted total?
//   - How many died with errors vs clean disconnects?
//   - What is the p99 connection lifetime?
//
// This is intentionally minimal — in production you'd emit these
// as Prometheus counters/gauges and scrape them via /metrics.
// We use atomic operations (no mutex) for thread-safety at low overhead.
//
// DESIGN: why atomic and not a mutex-protected struct?
// Metrics are updated on every connection open/close — potentially
// thousands of times per second. A mutex would become a hot contention
// point. atomic.Int64 operations are CPU-native instructions (LOCK XADD),
// taking ~5ns vs ~50ns for an uncontended mutex and much worse under contention.

package metrics

import (
	"fmt"
	"sync/atomic"
	"time"
)

// ConnectionMetrics tracks server-wide connection statistics.
// All fields are safe for concurrent access without locks.
type ConnectionMetrics struct {
	// Counters — monotonically increasing
	totalAccepted   atomic.Int64
	totalClosed     atomic.Int64
	totalErrors     atomic.Int64
	totalMessages   atomic.Int64
	totalBytesRead  atomic.Int64
	totalBytesWrote atomic.Int64

	// Gauge — can increase and decrease
	activeConnections atomic.Int64

	// Server start time for uptime calculation
	startTime time.Time
}

func New() *ConnectionMetrics {
	return &ConnectionMetrics{startTime: time.Now()}
}

// OnAccept is called when a new connection is accepted.
func (m *ConnectionMetrics) OnAccept() {
	m.totalAccepted.Add(1)
	m.activeConnections.Add(1)
}

// OnClose is called when a connection closes cleanly.
func (m *ConnectionMetrics) OnClose() {
	m.totalClosed.Add(1)
	m.activeConnections.Add(-1)
}

// OnError is called when a connection closes due to an error.
func (m *ConnectionMetrics) OnError() {
	m.totalErrors.Add(1)
	m.activeConnections.Add(-1)
}

// OnMessage is called for each successfully read message.
func (m *ConnectionMetrics) OnMessage(bytesRead, bytesWrote int) {
	m.totalMessages.Add(1)
	m.totalBytesRead.Add(int64(bytesRead))
	m.totalBytesWrote.Add(int64(bytesWrote))
}

// Snapshot returns a point-in-time copy of all metrics.
// Safe to call concurrently — each Load() is atomic.
func (m *ConnectionMetrics) Snapshot() Snapshot {
	return Snapshot{
		ActiveConnections: m.activeConnections.Load(),
		TotalAccepted:     m.totalAccepted.Load(),
		TotalClosed:       m.totalClosed.Load(),
		TotalErrors:       m.totalErrors.Load(),
		TotalMessages:     m.totalMessages.Load(),
		TotalBytesRead:    m.totalBytesRead.Load(),
		TotalBytesWrote:   m.totalBytesWrote.Load(),
		Uptime:            time.Since(m.startTime).Round(time.Second),
	}
}

// Snapshot is an immutable point-in-time view of metrics.
type Snapshot struct {
	ActiveConnections int64
	TotalAccepted     int64
	TotalClosed       int64
	TotalErrors       int64
	TotalMessages     int64
	TotalBytesRead    int64
	TotalBytesWrote   int64
	Uptime            time.Duration
}

func (s Snapshot) String() string {
	return fmt.Sprintf(
		"uptime=%s active=%d accepted=%d closed=%d errors=%d messages=%d bytes_in=%d bytes_out=%d",
		s.Uptime,
		s.ActiveConnections,
		s.TotalAccepted,
		s.TotalClosed,
		s.TotalErrors,
		s.TotalMessages,
		s.TotalBytesRead,
		s.TotalBytesWrote,
	)
}