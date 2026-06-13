// RejectionPolicy.java

/**
 * Determines what happens when a task cannot be accepted —
 * either because the pool is shut down or the task queue is full.
 *
 * Each policy represents a different tradeoff between throughput,
 * latency, and data loss.
 */
public interface RejectionPolicy {

    /**
     * Called when a task cannot be queued.
     *
     * @param task the task that was rejected
     * @param pool the pool that rejected it (for CallerRuns access)
     */
    void onRejection(Runnable task, ThreadPool pool);


    /**
     * Throws RejectedTaskException.
     * Caller is responsible for handling overload.
     * Appropriate when the caller must know about rejection explicitly.
     */
    class AbortPolicy implements RejectionPolicy {
        @Override
        public void onRejection(Runnable task, ThreadPool pool) {
            throw new RejectedTaskException(
                "Task rejected: pool is shut down or queue is full"
            );
        }
    }

    /**
     * The calling thread executes the task directly.
     *
     * This is the most production-useful policy:
     * - slows down the producer naturally (backpressure)
     * - no tasks are lost
     * - pool is not bypassed — caller runs only when pool is saturated
     */
    class CallerRunsPolicy implements RejectionPolicy {
        @Override
        public void onRejection(Runnable task, ThreadPool pool) {
            if (!pool.isShutdown()) {
                task.run();
            }
            // if pool is shut down, discard silently —
            // running the task on caller after shutdown is unexpected behavior
        }
    }

    /**
     * Silently drops the task. No exception thrown.
     * Only appropriate for truly disposable work (metrics, non-critical events).
     */
    class DiscardPolicy implements RejectionPolicy {
        @Override
        public void onRejection(Runnable task, ThreadPool pool) {
            // intentionally empty — task is dropped
        }
    }
}