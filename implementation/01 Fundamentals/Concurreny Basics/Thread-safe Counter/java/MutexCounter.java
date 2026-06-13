// MutexCounter.java

import java.util.concurrent.locks.ReentrantLock;

/**
 * Thread-safe counter backed by a ReentrantLock (mutex semantics).
 *
 * Correctness strategy:
 *   Every read-modify-write operation acquires the lock before reading
 *   and releases it after writing. Only one thread executes any critical
 *   section at a time. Visibility is guaranteed by the happens-before
 *   relationship established by lock release → lock acquire.
 *
 * Why ReentrantLock over synchronized:
 *   - ReentrantLock makes the locking intent explicit and readable.
 *   - Allows try-lock, timed lock, and interruptible lock acquisition
 *     if needed in future extensions.
 *   - synchronized is fine too — but ReentrantLock is more idiomatic
 *     for production code where lock behavior may evolve.
 *
 * Performance characteristic:
 *   Under high contention, threads sleep waiting for the lock.
 *   Each wake-up costs a context switch (~5,000–10,000 ns).
 *   At 50,000 RPS with many threads, this overhead accumulates.
 *   For simple increment-only use cases, prefer AtomicCounter.
 *
 * When to prefer this over AtomicCounter:
 *   - When you need to atomically update multiple fields together.
 *   - When the critical section involves logic beyond a single operation.
 *   - When you need condition-based waiting (use with Condition).
 */
public class MutexCounter implements Counter {

    private long value = 0;

    // Fair mode (true) = FIFO ordering — prevents starvation, lower throughput.
    // Unfair mode (false) = higher throughput, possible starvation under extreme load.
    // Unfair is correct default for a counter — starvation risk is negligible.
    private final ReentrantLock lock = new ReentrantLock(false);

    @Override
    public void increment() {
        lock.lock();
        try {
            value++;
        } finally {
            // finally block is critical — guarantees unlock even if an exception
            // occurs inside the critical section. Without this, lock is never
            // released and every subsequent caller blocks forever (deadlock).
            lock.unlock();
        }
    }

    @Override
    public void decrement() {
        lock.lock();
        try {
            value--;
        } finally {
            lock.unlock();
        }
    }

    @Override
    public void incrementBy(long delta) {
        if (delta <= 0) {
            throw new IllegalArgumentException(
                "delta must be positive, got: " + delta
            );
        }
        lock.lock();
        try {
            value += delta;
        } finally {
            lock.unlock();
        }
    }

    /**
     * Reset is NOT atomic with respect to concurrent increments.
     *
     * A thread can increment between this thread's lock acquisition
     * and the reset write — but that increment is then overwritten by
     * the reset. This is expected behavior: reset is a best-effort
     * operation. Document this in your API contract.
     */
    @Override
    public void reset() {
        lock.lock();
        try {
            value = 0;
        } finally {
            lock.unlock();
        }
    }

    /**
     * Returns a snapshot of the current value.
     *
     * The value may change immediately after this call returns.
     * Callers must not assume the value remains stable after get().
     * This is correct and expected behavior for concurrent counters.
     */
    @Override
    public long get() {
        lock.lock();
        try {
            return value;
        } finally {
            lock.unlock();
        }
    }

    /** Diagnostic: exposes whether any thread holds the lock. For testing only. */
    public boolean isLocked() {
        return lock.isLocked();
    }

    /** Diagnostic: how many threads are queued waiting for the lock. For monitoring. */
    public int getQueueLength() {
        return lock.getQueueLength();
    }
}