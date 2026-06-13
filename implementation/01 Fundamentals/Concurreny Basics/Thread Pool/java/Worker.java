// Worker.java

import java.util.concurrent.BlockingQueue;
import java.util.concurrent.TimeUnit;
import java.util.concurrent.atomic.AtomicInteger;
import java.util.logging.Level;
import java.util.logging.Logger;

/**
 * A worker thread that pulls tasks from the shared queue and executes them.
 *
 * Key design decisions:
 *
 * 1. Poll with timeout instead of blocking take():
 *    - take() blocks indefinitely — worker can't notice shutdown
 *    - poll(timeout) wakes up periodically to check pool state
 *    - Tradeoff: tiny wakeup overhead vs guaranteed shutdown responsiveness
 *
 * 2. Task exceptions must not kill the worker:
 *    - If a task throws, the worker catches it, logs it, and continues
 *    - A dead worker means permanent capacity reduction — silent throughput loss
 *
 * 3. activeWorkerCount is decremented AFTER the task completes:
 *    - Ensures getActiveWorkerCount() reflects actual in-flight work
 *    - decrementAndGet happens in finally — guaranteed even on task exception
 *
 * 4. InterruptedException during poll = shutdown signal:
 *    - shutdownNow() calls thread.interrupt() on all workers
 *    - Worker exits cleanly on interrupt rather than swallowing it
 */
class Worker implements Runnable {

    private static final Logger log = Logger.getLogger(Worker.class.getName());

    // How long to wait for a task before re-checking pool state.
    // Short enough for responsive shutdown, long enough to avoid busy-waiting.
    private static final long POLL_TIMEOUT_MS = 100;

    private final BlockingQueue<Runnable> taskQueue;
    private final AtomicInteger activeWorkerCount;
    private final FixedThreadPool pool;
    private final String workerName;

    Worker(BlockingQueue<Runnable> taskQueue,
           AtomicInteger activeWorkerCount,
           FixedThreadPool pool,
           int workerId) {
        this.taskQueue = taskQueue;
        this.activeWorkerCount = activeWorkerCount;
        this.pool = pool;
        this.workerName = "worker-" + workerId;
    }

    @Override
    public void run() {
        log.fine(() -> workerName + " started");

        try {
            workerLoop();
        } finally {
            // Notify pool that this worker has exited.
            // Pool uses this to detect full termination.
            pool.onWorkerExit();
            log.fine(() -> workerName + " terminated");
        }
    }

    private void workerLoop() {
        while (true) {
            // Check if we should stop before trying to fetch a task.
            // STOP state (shutdownNow) exits immediately, even with tasks remaining.
            if (pool.getState().isAtLeast(PoolState.STOP)) {
                return;
            }

            // SHUTDOWN state: keep running until queue is drained.
            // RUNNING state: keep running indefinitely.
            Runnable task = fetchTask();

            if (task == null) {
                // poll() timed out — re-check state on next iteration.
                // If pool is SHUTDOWN and queue is empty, exit.
                if (pool.getState().isAtLeast(PoolState.SHUTDOWN) && taskQueue.isEmpty()) {
                    return;
                }
                continue;
            }

            executeTask(task);
        }
    }

    private Runnable fetchTask() {
        try {
            // Poll with timeout — allows periodic state checks.
            // Does NOT block indefinitely like take() would.
            return taskQueue.poll(POLL_TIMEOUT_MS, TimeUnit.MILLISECONDS);
        } catch (InterruptedException e) {
            // shutdownNow() interrupted us — exit the loop.
            Thread.currentThread().interrupt(); // restore interrupt flag
            return null;
        }
    }

    private void executeTask(Runnable task) {
        activeWorkerCount.incrementAndGet();
        try {
            task.run();
        } catch (Exception e) {
            // CRITICAL: catch all exceptions from the task.
            // An uncaught exception would kill the worker thread permanently,
            // reducing pool capacity without any visible signal.
            log.log(Level.WARNING, workerName + " caught exception from task", e);
        } finally {
            // Always decrement — even if task threw.
            activeWorkerCount.decrementAndGet();
        }
    }
}