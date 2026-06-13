// Counter.java

/**
 * A counter that supports concurrent increment, decrement, and reset operations.
 *
 * Implementations must be thread-safe. The interface exposes behavior only —
 * synchronization strategy is an implementation detail hidden from callers.
 *
 * Design decision: long instead of int.
 * At high throughput (50k RPS), an int overflows in ~42,949 seconds (~12 hours).
 * A long overflows in ~292 billion years. Production counters use long.
 */
public interface Counter {

    /** Atomically increments the counter by 1. */
    void increment();

    /** Atomically decrements the counter by 1. */
    void decrement();

    /**
     * Atomically increments the counter by the given delta.
     *
     * @param delta must be positive; throws IllegalArgumentException otherwise.
     *              Negative increments should use decrement() explicitly —
     *              mixing signs through this method hides intent.
     */
    void incrementBy(long delta);

    /** Resets the counter to zero. Not guaranteed to be atomic with other operations. */
    void reset();

    /** Returns the current value. May reflect a stale snapshot under concurrent modification. */
    long get();
}