package metrics_test

import (
	"sync"
	"testing"

	"tcp-lab/metrics"
)

// TestConcurrentUpdatesAreConsistent verifies that concurrent OnAccept/OnClose
// calls produce consistent results — no lost updates from data races.
//
// This test would detect bugs if we used a non-atomic int (e.g. plain int64).
// Run with -race flag to catch data races: go test -race ./metrics/...
func TestConcurrentUpdatesAreConsistent(t *testing.T) {
	m := metrics.New()

	const goroutines = 100
	const opsPerGoroutine = 1000

	var wg sync.WaitGroup

	// Simulate 100 goroutines each accepting and closing 1000 connections.
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < opsPerGoroutine; j++ {
				m.OnAccept()
				m.OnMessage(64, 64)
				m.OnClose()
			}
		}()
	}

	wg.Wait()

	snap := m.Snapshot()

	expectedTotal := int64(goroutines * opsPerGoroutine)

	if snap.TotalAccepted != expectedTotal {
		t.Errorf("TotalAccepted: got %d, want %d", snap.TotalAccepted, expectedTotal)
	}
	if snap.TotalClosed != expectedTotal {
		t.Errorf("TotalClosed: got %d, want %d", snap.TotalClosed, expectedTotal)
	}
	if snap.ActiveConnections != 0 {
		t.Errorf("ActiveConnections: got %d, want 0 (all should be closed)", snap.ActiveConnections)
	}
	if snap.TotalMessages != expectedTotal {
		t.Errorf("TotalMessages: got %d, want %d", snap.TotalMessages, expectedTotal)
	}
	if snap.TotalBytesRead != expectedTotal*64 {
		t.Errorf("TotalBytesRead: got %d, want %d", snap.TotalBytesRead, expectedTotal*64)
	}
}

// TestActiveConnectionsNeverGoNegative verifies that a bug causing more
// OnClose calls than OnAccept calls doesn't produce a negative gauge.
func TestOnErrorDecrements(t *testing.T) {
	m := metrics.New()

	m.OnAccept()
	m.OnAccept()
	m.OnError() // one connection errored

	snap := m.Snapshot()
	if snap.ActiveConnections != 1 {
		t.Errorf("ActiveConnections: got %d, want 1", snap.ActiveConnections)
	}
	if snap.TotalErrors != 1 {
		t.Errorf("TotalErrors: got %d, want 1", snap.TotalErrors)
	}
	if snap.TotalAccepted != 2 {
		t.Errorf("TotalAccepted: got %d, want 2", snap.TotalAccepted)
	}
}

// TestSnapshotIsImmutable verifies that mutating the server after taking a
// snapshot does not change the snapshot values.
func TestSnapshotIsImmutable(t *testing.T) {
	m := metrics.New()
	m.OnAccept()
	m.OnAccept()

	snap := m.Snapshot()
	if snap.ActiveConnections != 2 {
		t.Fatalf("initial snapshot: got %d, want 2", snap.ActiveConnections)
	}

	// Mutate server state after snapshot
	m.OnClose()
	m.OnClose()

	// Snapshot should be unchanged (it's a value copy)
	if snap.ActiveConnections != 2 {
		t.Errorf("snapshot was mutated: got %d, want 2", snap.ActiveConnections)
	}
}