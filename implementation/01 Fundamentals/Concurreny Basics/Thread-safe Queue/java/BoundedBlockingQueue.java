// BoundedBlockingQueue.java

import java.util.ArrayDeque;
import java.util.Deque;
import java.util.concurrent.Semaphore;
import java.util.concurrent.TimeUnit;
import java.util.concurrent.atomic.AtomicBoolean;
import java.util.logging.Logger;

/**
 * Thread-safe bounded blocking queue using two semaphores + one mutex.
 *
 * Concurrency model:
 *   availableSlots — counting semaphore initialized to capacity.
 *                    Producers acquire before inserting (blocks when queue is full).
 *   availableItems — counting semaphore initialized to 0.
 *                    Consumers acquire before removing (blocks when queue is empty).
 *   mutex          — binary semaphore (fair) protecting actual array access.
 *
 * Critical acquisition order (deadlock prevention via lock ordering):
 *   Producer: availableSlots → mutex → [insert] → mutex release → availableItems
 *   Consumer: availableItems → mutex → [remove] → mutex release → availableSlots
 *
 * Never acquire mutex before semaphore — that ordering causes deadlock when
 * the queue is full/empty and the other side needs the mutex to make progress.
 *
 * @param <T> type of items held in the queue
 */
public class BoundedBlockingQueue<T> implements BoundedQueue<T> {

    private static final Logger log = Logger.getLogger(BoundedBlockingQueue.class.getName());

    private final Deque<T> store;
    private final int capacity;

    // Semaphores for producer/consumer coordination
    private final Semaphore availableSlots;   // producers acquire: blocks when full
    private final Semaphore availableItems;   // consumers acquire: blocks when empty

    // Fair binary semaphore as mutex — fair prevents starvation under high contention
    private final Semaphore mutex;

    // Shutdown state — volatile for visibility across threads without full synchronization
    private final AtomicBoolean shutdownInitiated = new AtomicBoolean(false);

    /**
     * @param capacity maximum number of items the queue can hold
     * @throws IllegalArgumentException if capacity <= 0
     */
    public BoundedBlockingQueue(int capacity) {
        if (capacity <= 0) {
            throw new IllegalArgumentException("Capacity must be positive, got: " + capacity);
        }
        this.capacity = capacity;
        this.store = new ArrayDeque<>(capacity);
        this.availableSlots = new Semaphore(capacity, true);  // fair
        this.availableItems = new Semaphore(0, true);          // fair
        this.mutex = new Semaphore(1, true);                   // fair binary semaphore
    }

    // -------------------------------------------------------------------------
    // Producer API
    // -------------------------------------------------------------------------

    @Override
    public void put(T item) throws InterruptedException {
        validateItem(item);
        checkNotShutdown();

        availableSlots.acquire();          // block if queue is full
        insertUnderLock(item);
        availableItems.release();          // signal a consumer
    }

    @Override
    public boolean offer(T item, long timeout, TimeUnit unit) throws InterruptedException {
        validateItem(item);
        checkNotShutdown();

        // Try to acquire a slot within the timeout
        boolean slotAcquired = availableSlots.tryAcquire(timeout, unit);
        if (!slotAcquired) {
            log.fine(() -> "offer() timed out — queue full, capacity=" + capacity);
            return false;
        }

        insertUnderLock(item);
        availableItems.release();
        return true;
    }

    // -------------------------------------------------------------------------
    // Consumer API
    // -------------------------------------------------------------------------

    @Override
    public T take() throws InterruptedException {
        availableItems.acquire();          // block if queue is empty
        T item = removeUnderLock();
        availableSlots.release();          // signal a producer
        return item;
    }

    @Override
    public T poll(long timeout, TimeUnit unit) throws InterruptedException {
        boolean itemAvailable = availableItems.tryAcquire(timeout, unit);
        if (!itemAvailable) {
            log.fine("poll() timed out — queue empty");
            return null;
        }

        T item = removeUnderLock();
        availableSlots.release();
        return item;
    }

    // -------------------------------------------------------------------------
    // Shutdown
    // -------------------------------------------------------------------------

    @Override
    public void shutdown() {
        if (shutdownInitiated.compareAndSet(false, true)) {
            log.info("Queue shutdown initiated — no new items will be accepted");
        }
    }

    @Override
    public boolean isTerminated() {
        return shutdownInitiated.get() && isEmpty();
    }

    // -------------------------------------------------------------------------
    // State queries
    // -------------------------------------------------------------------------

    @Override
    public int size() {
        // availableItems permits = current number of items in queue
        return availableItems.availablePermits();
    }

    @Override
    public int capacity() {
        return capacity;
    }

    @Override
    public boolean isEmpty() {
        return availableItems.availablePermits() == 0;
    }

    @Override
    public boolean isFull() {
        return availableSlots.availablePermits() == 0;
    }

    // -------------------------------------------------------------------------
    // Internal helpers
    // -------------------------------------------------------------------------

    /**
     * Inserts an item into the store under mutex protection.
     * Caller must have already acquired availableSlots.
     */
    private void insertUnderLock(T item) throws InterruptedException {
        mutex.acquire();
        try {
            store.addLast(item);
        } finally {
            mutex.release();   // always release — even if addLast somehow throws
        }
    }

    /**
     * Removes and returns the head item from the store under mutex protection.
     * Caller must have already acquired availableItems.
     */
    private T removeUnderLock() throws InterruptedException {
        mutex.acquire();
        try {
            return store.pollFirst();
        } finally {
            mutex.release();
        }
    }

    private void validateItem(T item) {
        if (item == null) {
            // Null items break poll()'s null-means-timeout contract
            throw new IllegalArgumentException("Null items are not permitted");
        }
    }

    private void checkNotShutdown() {
        if (shutdownInitiated.get()) {
            throw new IllegalStateException("Queue has been shut down — no new items accepted");
        }
    }

    @Override
    public String toString() {
        return String.format("BoundedBlockingQueue{size=%d, capacity=%d, shutdown=%s}",
                size(), capacity, shutdownInitiated.get());
    }
}