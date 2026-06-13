// CounterTest.java

import org.junit.jupiter.api.Test;
import org.junit.jupiter.params.ParameterizedTest;
import org.junit.jupiter.params.provider.MethodSource;

import java.util.concurrent.CountDownLatch;
import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;
import java.util.concurrent.TimeUnit;
import java.util.stream.Stream;

import static org.junit.jupiter.api.Assertions.*;

/**
 * Test strategy:
 *
 * 1. Functional correctness — single-threaded, verifies basic operations.
 * 2. Concurrency correctness — multi-threaded, verifies no lost updates.
 * 3. Race condition demonstration — proves UnsafeCounter loses increments.
 * 4. Edge cases — invalid inputs, boundary behavior.
 *
 * Key testing challenge: concurrency bugs are non-deterministic.
 * We use high thread counts and high operation counts to make races
 * statistically certain to manifest, not just occasionally possible.
 *
 * CountDownLatch pattern:
 *   All threads wait at a gate (startLatch) until released simultaneously.
 *   This maximizes thread contention — the worst-case scenario.
 *   Without it, threads start staggered and rarely actually contend.
 */
class CounterTest {

    private static final int THREAD_COUNT = 10;
    private static final int OPS_PER_THREAD = 100_000;
    private static final long EXPECTED_TOTAL = (long) THREAD_COUNT * OPS_PER_THREAD;

    // Parameterized: run same concurrency tests against all thread-safe implementations.
    static Stream<Counter> threadSafeCounters() {
        return Stream.of(new MutexCounter(), new AtomicCounter());
    }

    // --- Functional correctness (single-threaded) ---

    @ParameterizedTest
    @MethodSource("threadSafeCounters")
    void increment_singleThread_correctValue(Counter counter) {
        counter.increment();
        counter.increment();
        counter.increment();
        assertEquals(3, counter.get());
    }

    @ParameterizedTest
    @MethodSource("threadSafeCounters")
    void decrement_belowZero_allowed(Counter counter) {
        counter.decrement();
        assertEquals(-1, counter.get(), "Counters should support negative values");
    }

    @ParameterizedTest
    @MethodSource("threadSafeCounters")
    void incrementBy_validDelta_correctValue(Counter counter) {
        counter.incrementBy(50);
        assertEquals(50, counter.get());
    }

    @ParameterizedTest
    @MethodSource("threadSafeCounters")
    void incrementBy_zeroDelta_throwsException(Counter counter) {
        assertThrows(IllegalArgumentException.class, () -> counter.incrementBy(0));
    }

    @ParameterizedTest
    @MethodSource("threadSafeCounters")
    void incrementBy_negativeDelta_throwsException(Counter counter) {
        assertThrows(IllegalArgumentException.class, () -> counter.incrementBy(-5));
    }

    @ParameterizedTest
    @MethodSource("threadSafeCounters")
    void reset_afterIncrements_returnsZero(Counter counter) {
        counter.incrementBy(1000);
        counter.reset();
        assertEquals(0, counter.get());
    }

    // --- Concurrency correctness ---

    @ParameterizedTest
    @MethodSource("threadSafeCounters")
    void increment_highConcurrency_noLostUpdates(Counter counter)
            throws InterruptedException {

        ExecutorService executor = Executors.newFixedThreadPool(THREAD_COUNT);
        CountDownLatch startLatch = new CountDownLatch(1);  // gate — holds all threads
        CountDownLatch doneLatch = new CountDownLatch(THREAD_COUNT);

        for (int i = 0; i < THREAD_COUNT; i++) {
            executor.submit(() -> {
                try {
                    startLatch.await();  // all threads wait here until released
                    for (int j = 0; j < OPS_PER_THREAD; j++) {
                        counter.increment();
                    }
                } catch (InterruptedException e) {
                    Thread.currentThread().interrupt();
                } finally {
                    doneLatch.countDown();
                }
            });
        }

        startLatch.countDown();  // release all threads simultaneously — maximum contention
        assertTrue(doneLatch.await(30, TimeUnit.SECONDS), "Threads did not complete in time");
        executor.shutdown();

        assertEquals(EXPECTED_TOTAL, counter.get(),
            "Expected " + EXPECTED_TOTAL + " but got " + counter.get() +
            " — lost " + (EXPECTED_TOTAL - counter.get()) + " increments");
    }

    @ParameterizedTest
    @MethodSource("threadSafeCounters")
    void mixedIncrementDecrement_highConcurrency_correctNetValue(Counter counter)
            throws InterruptedException {

        // Half threads increment, half decrement — net result must be zero.
        ExecutorService executor = Executors.newFixedThreadPool(THREAD_COUNT);
        CountDownLatch startLatch = new CountDownLatch(1);
        CountDownLatch doneLatch = new CountDownLatch(THREAD_COUNT);

        for (int i = 0; i < THREAD_COUNT; i++) {
            final boolean shouldIncrement = (i % 2 == 0);
            executor.submit(() -> {
                try {
                    startLatch.await();
                    for (int j = 0; j < OPS_PER_THREAD; j++) {
                        if (shouldIncrement) counter.increment();
                        else counter.decrement();
                    }
                } catch (InterruptedException e) {
                    Thread.currentThread().interrupt();
                } finally {
                    doneLatch.countDown();
                }
            });
        }

        startLatch.countDown();
        assertTrue(doneLatch.await(30, TimeUnit.SECONDS), "Threads did not complete in time");
        executor.shutdown();

        assertEquals(0, counter.get(),
            "Mixed increment/decrement should net to zero, got: " + counter.get());
    }

    // --- Race condition demonstration ---

    @Test
    void unsafeCounter_highConcurrency_losesIncrements() throws InterruptedException {
        // This test EXPECTS the unsafe counter to produce a wrong result.
        // It is not a bug in the test — it is proof that UnsafeCounter is broken.
        // If this test fails (unsafe counter returns correct value), the JVM
        // happened to serialize operations by luck — re-run to see the race.
        UnsafeCounter unsafe = new UnsafeCounter();
        ExecutorService executor = Executors.newFixedThreadPool(THREAD_COUNT);
        CountDownLatch startLatch = new CountDownLatch(1);
        CountDownLatch doneLatch = new CountDownLatch(THREAD_COUNT);

        for (int i = 0; i < THREAD_COUNT; i++) {
            executor.submit(() -> {
                try {
                    startLatch.await();
                    for (int j = 0; j < OPS_PER_THREAD; j++) {
                        unsafe.increment();
                    }
                } catch (InterruptedException e) {
                    Thread.currentThread().interrupt();
                } finally {
                    doneLatch.countDown();
                }
            });
        }

        startLatch.countDown();
        assertTrue(doneLatch.await(30, TimeUnit.SECONDS));
        executor.shutdown();

        long actual = unsafe.get();
        System.out.printf(
            "UnsafeCounter: expected=%d, actual=%d, lost=%d (%.1f%% loss)%n",
            EXPECTED_TOTAL, actual, EXPECTED_TOTAL - actual,
            (double)(EXPECTED_TOTAL - actual) / EXPECTED_TOTAL * 100
        );

        // We assert it is WRONG. If this assertion fails, the race didn't manifest —
        // increase THREAD_COUNT or OPS_PER_THREAD.
        assertNotEquals(EXPECTED_TOTAL, actual,
            "UnsafeCounter should have lost increments under concurrency");
    }

    // --- AtomicCounter-specific behavior ---

    @Test
    void atomicCounter_getAndReset_returnsSnapshotAndZeros() throws InterruptedException {
        AtomicCounter counter = new AtomicCounter();
        counter.incrementBy(500);

        long snapshot = counter.getAndReset();

        assertEquals(500, snapshot, "getAndReset should return value before reset");
        assertEquals(0, counter.get(), "counter should be zero after getAndReset");
    }

    @Test
    void atomicCounter_getAndReset_atomicity_noIncrementLost() throws InterruptedException {
        // Verifies that getAndReset doesn't lose concurrent increments.
        // After getAndReset completes, all increments that happened before
        // the reset are captured in the snapshot. Increments after are in the counter.
        AtomicCounter counter = new AtomicCounter();
        int incrementThreads = 4;
        ExecutorService executor = Executors.newFixedThreadPool(incrementThreads + 1);
        CountDownLatch startLatch = new CountDownLatch(1);
        CountDownLatch doneLatch = new CountDownLatch(incrementThreads);

        for (int i = 0; i < incrementThreads; i++) {
            executor.submit(() -> {
                try {
                    startLatch.await();
                    for (int j = 0; j < 10_000; j++) counter.increment();
                } catch (InterruptedException e) {
                    Thread.currentThread().interrupt();
                } finally {
                    doneLatch.countDown();
                }
            });
        }

        startLatch.countDown();
        doneLatch.await(10, TimeUnit.SECONDS);
        executor.shutdown();

        long snapshot = counter.getAndReset();
        long remaining = counter.get();

        // snapshot + remaining must equal total increments.
        // remaining captures any increments that occurred after getAndReset().
        assertEquals(incrementThreads * 10_000L, snapshot + remaining,
            "No increments should be lost across getAndReset boundary");
    }
}