// UnsafeCounter.java

/**
 * UNSAFE: Do not use in production or multi-threaded contexts.
 *
 * Intentionally non-thread-safe. Exists to demonstrate the race condition
 * that occurs on counter++ under concurrent access.
 *
 * Race condition anatomy:
 *   counter++ compiles to three bytecode instructions:
 *     1. GETFIELD  — read current value into register
 *     2. IADD      — add 1 in register
 *     3. PUTFIELD  — write register back to field
 *
 *   Two threads can both execute step 1 before either executes step 3.
 *   Both read the same value. Both write value+1. One increment is lost.
 *
 * Expected lost increments at 10 threads × 100,000 ops: typically 50,000–900,000.
 * The exact loss is timing-dependent and non-deterministic.
 */
public class UnsafeCounter implements Counter {

    private long value = 0;

    @Override
    public void increment() {
        value++;  // NOT atomic — read-modify-write race condition
    }

    @Override
    public void decrement() {
        value--;  // same race condition
    }

    @Override
    public void incrementBy(long delta) {
        if (delta <= 0) {
            throw new IllegalArgumentException(
                "delta must be positive, got: " + delta
            );
        }
        value += delta;
    }

    @Override
    public void reset() {
        value = 0;
    }

    @Override
    public long get() {
        return value;  // may read partially written value on 32-bit JVMs for long
    }
}