// AtomicCounter.java

import java.util.concurrent.atomic.AtomicLong;

/**
 * Thread-safe counter backed by CAS (Compare-And-Swap) via AtomicLong.
 *
 * Correctness strategy:
 *   AtomicLong.incrementAndGet() maps to a single LOCK XADD CPU instruction
 *   on x86. The CPU locks the memory bus for the duration of the instruction.
 *   No OS involvement. No thread sleeping. No context switch.
 *
 * Why this is the production default for simple counters:
 *   - Under low-to-medium contention: significantly faster than mutex.
 *   - No thread ever sleeps — no context switch overhead.
 *   - Failed CAS operations retry immediately in user space.
 *   - JVM intrinsifies AtomicLong operations — compiled to a single CPU instruction.
 *
 * When CAS degrades:
 *   Under extreme contention (thousands of threads on the same counter),
 *   CAS retry loops burn CPU cycles. In this case:
 *   - Use LongAdder (Java 8+) which stripes across multiple cells to reduce contention.
 *   - LongAdder.sum() is eventually consistent — fine for metrics, wrong for exact counts.
 *
 * AtomicLong vs LongAdder:
 *   AtomicLong  — strong consistency, single value, slower under extreme contention.
 *   LongAdder   — higher throughput, sum() is approximate under concurrent modification.
 *   Choose AtomicLong when you need accurate reads. LongAdder for high-write metrics.
 */
public class AtomicCounter implements Counter {

    private final AtomicLong value = new AtomicLong(0);

    @Override
    public void increment() {
        value.incrementAndGet();
    }

    @Override
    public void decrement() {
        value.decrementAndGet();
    }

    @Override
    public void incrementBy(long delta) {
        if (delta <= 0) {
            throw new IllegalArgumentException(
                "delta must be positive, got: " + delta
            );
        }
        value.addAndGet(delta);
    }

    /**
     * Reset is not atomic with concurrent increments — same caveat as MutexCounter.
     *
     * set(0) is itself atomic, but a concurrent increment can race between
     * the caller's decision to reset and the actual set(0). Document this.
     *
     * If you need atomic "read-then-reset" (fetch current value and zero it),
     * use: long snapshot = value.getAndSet(0);
     */
    @Override
    public void reset() {
        value.set(0);
    }

    @Override
    public long get() {
        return value.get();
    }

    /**
     * Atomically reads the current value and resets to zero.
     *
     * Useful for metrics collection: "give me the count since last poll, then reset".
     * This operation is atomic — no increment is lost between read and reset.
     *
     * Not in the Counter interface because it's specific to this implementation's
     * capabilities. Exposing it here allows callers who know they have an
     * AtomicCounter to use it without breaking the interface contract.
     */
    public long getAndReset() {
        return value.getAndSet(0);
    }
}