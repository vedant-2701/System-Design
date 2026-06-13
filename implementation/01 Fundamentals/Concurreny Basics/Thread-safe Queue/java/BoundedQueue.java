// BoundedQueue.java

import java.util.concurrent.TimeUnit;

/**
 * A thread-safe bounded queue with blocking and timed-blocking semantics.
 *
 * Design decisions:
 * - Bounded to prevent unbounded memory growth under backpressure
 * - Blocking API is the primary path — callers that can't make progress should wait
 * - Timed API (offer/poll with timeout) is the recommended production path —
 *   indefinite blocking is a resource leak if the other side dies
 * - Graceful shutdown drains in-flight items; force shutdown stops immediately
 *
 * @param <T> type of items held in this queue
 */
public interface BoundedQueue<T> {

    /**
     * Inserts an item, blocking indefinitely until space is available.
     * Use only when the caller can safely wait forever.
     * Prefer {@link #offer(Object, long, TimeUnit)} in production.
     *
     * @throws InterruptedException if the thread is interrupted while waiting
     */
    void put(T item) throws InterruptedException;

    /**
     * Inserts an item, waiting up to the given timeout for space.
     *
     * @return true if the item was inserted, false if timeout elapsed
     * @throws InterruptedException if the thread is interrupted while waiting
     */
    boolean offer(T item, long timeout, TimeUnit unit) throws InterruptedException;

    /**
     * Removes and returns an item, blocking indefinitely until one is available.
     * Prefer {@link #poll(long, TimeUnit)} in production.
     *
     * @throws InterruptedException if the thread is interrupted while waiting
     */
    T take() throws InterruptedException;

    /**
     * Removes and returns an item, waiting up to the given timeout.
     *
     * @return the item, or null if timeout elapsed
     * @throws InterruptedException if the thread is interrupted while waiting
     */
    T poll(long timeout, TimeUnit unit) throws InterruptedException;

    /**
     * Initiates graceful shutdown. No new items accepted after this point.
     * Existing items in the queue can still be consumed.
     */
    void shutdown();

    /**
     * Returns true if the queue has been shut down and is empty.
     * Consumers should stop polling when this returns true.
     */
    boolean isTerminated();

    /** Current number of items in the queue. */
    int size();

    /** Maximum capacity of the queue. */
    int capacity();

    /** True if the queue contains no items. */
    boolean isEmpty();

    /** True if the queue is at maximum capacity. */
    boolean isFull();
}