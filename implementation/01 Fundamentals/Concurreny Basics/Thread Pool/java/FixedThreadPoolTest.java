// FixedThreadPoolTest.java

import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.Test;

import java.util.concurrent.CountDownLatch;
import java.util.concurrent.TimeUnit;
import java.util.concurrent.atomic.AtomicInteger;

import static org.junit.jupiter.api.Assertions.*;

/**
 * Tests verify behavior under concurrency — not just happy-path sequential execution.
 *
 * Testing strategy:
 * - Use CountDownLatch to synchronize test thread with pool workers
 * - Use AtomicInteger for thread-safe assertion counters
 * - Always call awaitTermination() — avoids flaky assertions on in-flight work
 * - Short timeouts catch hangs rather than letting tests block indefinitely
 */
class FixedThreadPoolTest {

    private FixedThreadPool pool;

    @AfterEach
    void tearDown() throws InterruptedException {
        if (pool != null && !pool.isShutdown()) {
            pool.shutdownNow();
            pool.awaitTermination(2, TimeUnit.SECONDS);
        }
    }

    // -------------------------------------------------------------------------
    // Basic execution
    // -------------------------------------------------------------------------

    @Test
    void submittedTasksExecute() throws InterruptedException {
        pool = FixedThreadPool.builder().poolSize(2).queueCapacity(10).build();

        CountDownLatch latch = new CountDownLatch(5);
        AtomicInteger executedCount = new AtomicInteger(0);

        for (int i = 0; i < 5; i++) {
            pool.submit(() -> {
                executedCount.incrementAndGet();
                latch.countDown();
            });
        }

        boolean completed = latch.await(3, TimeUnit.SECONDS);
        assertTrue(completed, "All tasks should complete within timeout");
        assertEquals(5, executedCount.get());
    }

    // -------------------------------------------------------------------------
    // Concurrency correctness
    // -------------------------------------------------------------------------

    @Test
    void concurrentSubmissionsAreAllExecuted() throws InterruptedException {
        pool = FixedThreadPool.builder().poolSize(4).queueCapacity(200).build();

        int taskCount = 100;
        CountDownLatch submissionReady = new CountDownLatch(1);
        CountDownLatch allExecuted = new CountDownLatch(taskCount);
        AtomicInteger executedCount = new AtomicInteger(0);

        // 10 threads each submit 10 tasks simultaneously
        for (int t = 0; t < 10; t++) {
            Thread submitter = new Thread(() -> {
                try {
                    submissionReady.await(); // all start at same time
                    for (int i = 0; i < 10; i++) {
                        pool.submit(() -> {
                            executedCount.incrementAndGet();
                            allExecuted.countDown();
                        });
                    }
                } catch (InterruptedException e) {
                    Thread.currentThread().interrupt();
                }
            });
            submitter.start();
        }

        submissionReady.countDown(); // release all submitters simultaneously
        boolean completed = allExecuted.await(5, TimeUnit.SECONDS);

        assertTrue(completed, "All concurrently submitted tasks should execute");
        assertEquals(taskCount, executedCount.get());
    }

    // -------------------------------------------------------------------------
    // Task exceptions must not kill workers
    // -------------------------------------------------------------------------

    @Test
    void workerSurvivesTaskException() throws InterruptedException {
        pool = FixedThreadPool.builder().poolSize(1).queueCapacity(10).build();

        CountDownLatch afterException = new CountDownLatch(1);
        AtomicInteger executedAfterException = new AtomicInteger(0);

        // First task throws
        pool.submit(() -> { throw new RuntimeException("intentional task failure"); });

        // Second task should still execute — worker must survive the exception
        pool.submit(() -> {
            executedAfterException.incrementAndGet();
            afterException.countDown();
        });

        boolean completed = afterException.await(3, TimeUnit.SECONDS);
        assertTrue(completed, "Worker should survive task exception and process next task");
        assertEquals(1, executedAfterException.get());
    }

    // -------------------------------------------------------------------------
    // Shutdown — graceful
    // -------------------------------------------------------------------------

    @Test
    void shutdownDrainsQueueBeforeTerminating() throws InterruptedException {
        pool = FixedThreadPool.builder().poolSize(1).queueCapacity(20).build();

        AtomicInteger executedCount = new AtomicInteger(0);
        CountDownLatch blockFirstTask = new CountDownLatch(1);

        // First task blocks the single worker — ensures queue builds up
        pool.submit(() -> {
            try { blockFirstTask.await(); } catch (InterruptedException e) {
                Thread.currentThread().interrupt();
            }
        });

        // Queue 5 more tasks while worker is blocked
        for (int i = 0; i < 5; i++) {
            pool.submit(executedCount::incrementAndGet);
        }

        pool.shutdown(); // initiate graceful shutdown

        // Submitting after shutdown should be rejected
        assertThrows(RejectedTaskException.class,
            () -> pool.submit(() -> {}),
            "Submit after shutdown should throw"
        );

        blockFirstTask.countDown(); // unblock worker — let queue drain

        boolean terminated = pool.awaitTermination(5, TimeUnit.SECONDS);
        assertTrue(terminated, "Pool should terminate after draining queue");
        assertEquals(5, executedCount.get(), "All queued tasks should have executed");
        assertTrue(pool.isTerminated());
    }

    // -------------------------------------------------------------------------
    // Shutdown — immediate
    // -------------------------------------------------------------------------

    @Test
    void shutdownNowInterruptsWorkersAndClearsQueue() throws InterruptedException {
        pool = FixedThreadPool.builder().poolSize(1).queueCapacity(20).build();

        CountDownLatch workerBlocked = new CountDownLatch(1);
        CountDownLatch workerInterrupted = new CountDownLatch(1);

        // Task that blocks and responds to interrupt
        pool.submit(() -> {
            try {
                workerBlocked.countDown();
                Thread.sleep(60_000); // blocks until interrupted
            } catch (InterruptedException e) {
                workerInterrupted.countDown();
                Thread.currentThread().interrupt();
            }
        });

        workerBlocked.await(2, TimeUnit.SECONDS); // ensure worker is in task

        // Queue tasks that should be abandoned
        for (int i = 0; i < 5; i++) {
            pool.submit(() -> {});
        }

        pool.shutdownNow();

        boolean interrupted = workerInterrupted.await(2, TimeUnit.SECONDS);
        assertTrue(interrupted, "Worker should receive interrupt signal");

        boolean terminated = pool.awaitTermination(3, TimeUnit.SECONDS);
        assertTrue(terminated, "Pool should terminate after shutdownNow");
        assertEquals(0, pool.getQueueSize(), "Queue should be cleared by shutdownNow");
    }

    // -------------------------------------------------------------------------
    // Rejection policies
    // -------------------------------------------------------------------------

    @Test
    void abortPolicyThrowsOnFullQueue() {
        pool = FixedThreadPool.builder()
            .poolSize(1)
            .queueCapacity(1)
            .rejectionPolicy(new RejectionPolicy.AbortPolicy())
            .build();

        CountDownLatch blockWorker = new CountDownLatch(1);
        pool.submit(() -> { try { blockWorker.await(); } catch (InterruptedException e) { Thread.currentThread().interrupt(); } });
        pool.submit(() -> {}); // fills the queue

        // Next submit should be rejected
        assertThrows(RejectedTaskException.class, () -> pool.submit(() -> {}));
        blockWorker.countDown();
    }

    @Test
    void callerRunsPolicyExecutesTaskOnCallerThread() throws InterruptedException {
        pool = FixedThreadPool.builder()
            .poolSize(1)
            .queueCapacity(1)
            .rejectionPolicy(new RejectionPolicy.CallerRunsPolicy())
            .build();

        CountDownLatch blockWorker = new CountDownLatch(1);
        pool.submit(() -> { try { blockWorker.await(); } catch (InterruptedException e) { Thread.currentThread().interrupt(); } });
        pool.submit(() -> {}); // fills the queue

        Thread callerThread = Thread.currentThread();
        AtomicInteger callerThreadId = new AtomicInteger(-1);

        // This submit should be rejected → CallerRuns executes on current thread
        pool.submit(() -> callerThreadId.set((int) callerThread.getId()));

        assertEquals((int) callerThread.getId(), callerThreadId.get(),
            "CallerRuns should execute task on the submitting thread");

        blockWorker.countDown();
    }

    // -------------------------------------------------------------------------
    // Null safety
    // -------------------------------------------------------------------------

    @Test
    void submitNullTaskThrowsNPE() {
        pool = FixedThreadPool.builder().poolSize(2).build();
        assertThrows(NullPointerException.class, () -> pool.submit(null));
    }
}