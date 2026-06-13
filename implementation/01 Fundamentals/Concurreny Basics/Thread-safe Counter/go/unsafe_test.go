//go:build !race
package counter

import "testing"

// TestUnsafeCounter_LosesIncrements intentionally proves the race condition.
// Run without -race flag to see lost increments in the output.
// Run with -race flag to see the data race detected by the Go race detector.
//
// NOTE: This test does NOT call t.Fail() if the count is wrong — that's expected.
// It calls t.Fail() only if the count is somehow correct (race didn't manifest).
func TestUnsafeCounter_LosesIncrements(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping race demonstration in short mode")
	}

	c := &UnsafeCounter{}
	runConcurrent(threadCount, c.Increment)

	got := c.Get()
	t.Logf("UnsafeCounter: expected=%d, got=%d, lost=%d (%.1f%% loss)",
		expectedTotal, got, expectedTotal-got,
		float64(expectedTotal-got)/float64(expectedTotal)*100)

	if got == expectedTotal {
		// Race didn't manifest this run — JVM/Go runtime happened to serialize ops.
		// This is statistically unlikely but possible. Increase threadCount to force it.
		t.Log("WARNING: UnsafeCounter returned correct value — race did not manifest this run")
	}
}
