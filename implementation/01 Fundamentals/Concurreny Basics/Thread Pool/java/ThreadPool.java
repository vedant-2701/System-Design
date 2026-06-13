//ThreadPool.java

import java.util.concurrent.TimeUnit;

/**
 * A bounded thread pool with configurable rejection handling and graceful shutdown.
 *
 * Design contract:
 * - submit() is thread-safe and may be called from multiple threads concurrently
 * - shutdown() stops accepting new tasks; already-queued tasks still execute
 * - shutdownNow() interrupts running workers; returns without waiting
 * - awaitTermination() blocks the caller until all workers finish or timeout elapses
 *
 * Rejection policy is applied when the task queue is full.
 */
public interface ThreadPool {

    /**
     * Submit a task for execution.
     *
     * @throws RejectedTaskException if the pool is shut down or the queue is full
     *                               and the rejection policy throws
     * @throws NullPointerException  if task is null
     */
    void submit(Runnable task);

    /**
     * Initiate graceful shutdown.
     * No new tasks accepted. Queued tasks continue to execute.
     * Returns immediately — does not wait for tasks to complete.
     */
    void shutdown();

    /**
     * Initiate immediate shutdown.
     * Interrupts all running workers. Queued tasks are abandoned.
     * Returns immediately.
     */
    void shutdownNow();

    /**
     * Block until all workers have terminated or the timeout elapses.
     *
     * @return true if terminated cleanly, false if timeout elapsed first
     */
    boolean awaitTermination(long timeout, TimeUnit unit) throws InterruptedException;

    /**
     * True if shutdown() or shutdownNow() has been called.
     */
    boolean isShutdown();

    /**
     * True if all workers have finished after shutdown.
     */
    boolean isTerminated();

    /**
     * Current number of tasks waiting in the queue.
     */
    int getQueueSize();

    /**
     * Number of workers currently executing a task.
     */
    int getActiveWorkerCount();
}