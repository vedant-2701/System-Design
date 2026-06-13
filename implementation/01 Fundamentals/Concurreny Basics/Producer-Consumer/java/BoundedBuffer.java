// BoundedBuffer.java

import java.util.concurrent.Semaphore;
import java.util.concurrent.locks.ReentrantLock;
import java.util.concurrent.TimeUnit;
import java.util.logging.Logger;

/**
 * A thread-safe bounded buffer for producer-consumer coordination.
 *
 * Design decisions:
 * - Two semaphores (spaces + items) handle backpressure without busy-waiting.
 * - A single ReentrantLock protects the circular array — no read-write lock
 *   needed because every hot-path operation is a write (insert or remove).
 * - Semaphore acquire happens BEFORE lock acquisition to prevent deadlock:
 *   holding a lock while blocking on a semaphore would prevent other threads
 *   from acquiring the lock to make progress.
 * - Circular array (not LinkedList) — fixed capacity is known upfront,
 *   array gives better cache locality and zero per-item allocation overhead.
 * - Poison pill shutdown: N sentinel values unblock exactly N waiting consumers
 *   without sleep/timeout polling.
 *
 * Ordering invariant (must never be violated):
 *   Producer: spaces.acquire() → lock → insert → unlock → items.release()
 *   Consumer: items.acquire()  → lock → remove → unlock → spaces.release()
 */
public class BoundedBuffer<T> {

    private static final Logger log = Logger.getLogger(BoundedBuffer.class.getName());

    // Sentinel value inserted during shutdown to unblock waiting consumers.
    // Using a typed wrapper avoids unchecked cast warnings on the generic array.
    private static final Object POISON_PILL = new Object();

    private final Object[] buffer;
    private final int capacity;

    // Circular buffer pointers
    private int head = 0; // next write position
    private int tail = 0; // next read position

    // spaces: how many empty slots remain. Producers acquire (block when 0).
    // items:  how many filled slots exist.  Consumers acquire (block when 0).
    private final Semaphore spaces;
    private final Semaphore items;

    // Protects head, tail, and buffer array mutations.
    // Fair lock prevents producer or consumer starvation under sustained load.
    private final ReentrantLock lock = new ReentrantLock(true);

    // Volatile bool - must be volatile to ensure visibility across threads.
    // If not volatile, threads may cache the value and never see the shutdown signal.
    private volatile boolean shutdown = false;

    public BoundedBuffer(int capacity) {
        if (capacity <= 0) {
            throw new IllegalArgumentException("Capacity must be positive, got: " + capacity);
        }
        this.capacity = capacity;
        this.buffer = new Object[capacity];
        this.spaces = new Semaphore(capacity, true); // starts full — all slots empty
        this.items  = new Semaphore(0,        true); // starts empty — no items yet
    }

    /**
     * Inserts an item, blocking until space is available.
     * Returns false immediately if the buffer is shut down.
     *
     * @throws InterruptedException if the thread is interrupted while waiting
     */
    public boolean put(T item) throws InterruptedException {
        if (item == null) throw new IllegalArgumentException("Null items not allowed");
        if (shutdown) return false;

        // Block here — outside the lock — until a slot is free.
        // If interrupted, propagate cleanly rather than swallowing.
        spaces.acquire();

        if (shutdown) {
            // Slot acquired but we're shutting down — return the permit.
            spaces.release();
            return false;
        }

        lock.lock();
        try {
            buffer[head] = item;
            head = (head + 1) % capacity;
        } finally {
            lock.unlock();
        }

        items.release(); // signal that one more item is available
        return true;
    }

    /**
     * Inserts an item, waiting up to {@code timeout} for space.
     * Returns false if timed out or shut down.
     */
    public boolean offer(T item, long timeout, TimeUnit unit) throws InterruptedException {
        if (item == null) throw new IllegalArgumentException("Null items not allowed");
        if (shutdown) return false;

        if (!spaces.tryAcquire(timeout, unit)) {
            return false; // timed out — backpressure signal to caller
        }

        if (shutdown) {
            spaces.release();
            return false;
        }

        lock.lock();
        try {
            buffer[head] = item;
            head = (head + 1) % capacity;
        } finally {
            lock.unlock();
        }

        items.release();
        return true;
    }

    /**
     * Removes and returns the next item, blocking until one is available.
     * Returns null only when shut down and no items remain.
     */
    @SuppressWarnings("unchecked")
    public T take() throws InterruptedException {
        // Block here — outside the lock — until an item is available.
        items.acquire();

        lock.lock();
        Object item;
        try {
            item = buffer[tail];
            buffer[tail] = null; // allow GC of dequeued item
            tail = (tail + 1) % capacity;
        } finally {
            lock.unlock();
        }

        spaces.release(); // signal that one more slot is free

        // Poison pill detected — re-insert for other consumers and signal shutdown
        if (item == POISON_PILL) {
            return null;
        }

        return (T) item;
    }

    /**
     * Initiates graceful shutdown.
     *
     * Producers will stop accepting new items immediately (put() returns false).
     * Consumers will drain remaining items, then receive null (poison pill) and exit.
     *
     * Inserts one poison pill per consumer count to unblock all waiting consumers.
     *
     * @param consumerCount number of consumer threads to unblock
     */
    public void shutdown(int consumerCount) {
        shutdown = true;
        log.info("Shutdown initiated — inserting " + consumerCount + " poison pills");

        // Insert poison pills to unblock each blocked consumer.
        // Each consumer that receives null will exit its processing loop.
        for (int i = 0; i < consumerCount; i++) {
            try {
                // Use spaces semaphore to insert poison pill into the buffer
                spaces.acquire();
                lock.lock();
                try {
                    buffer[head] = POISON_PILL;
                    head = (head + 1) % capacity;
                } finally {
                    lock.unlock();
                }
                items.release();
            } catch (InterruptedException e) {
                Thread.currentThread().interrupt();
                log.warning("Interrupted while inserting poison pill " + i);
                break;
            }
        }
    }

    // ---- Observability ----

    /** Approximate number of items currently in the buffer. Not guaranteed consistent. */
    public int size() {
        return items.availablePermits();
    }

    /** Approximate number of empty slots. Not guaranteed consistent. */
    public int remainingCapacity() {
        return spaces.availablePermits();
    }

    public int capacity() {
        return capacity;
    }

    public boolean isShutdown() {
        return shutdown;
    }
}