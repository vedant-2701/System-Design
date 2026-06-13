// BoundedBlockingQueueTest.java

import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.Timeout;

import java.util.ArrayList;
import java.util.Collections;
import java.util.List;
import java.util.concurrent.*;
import java.util.concurrent.atomic.AtomicInteger;

import static org.junit.jupiter.api.Assertions.*;

/**
 * Tests cover:
 *  - Basic correctness (put/take ordering)
 *  - Blocking semantics (producer blocks on full, consumer blocks on empty)
 *  - Timeout semantics (offer/poll return correctly on timeout)
 *  - Concurrent correctness (no lost items, no duplicates under high concurrency)
 *  - Shutdown semantics (no new items after shutdown, existing items drainable)
 *  - Edge cases (null rejection, zero capacity, interruption handling)
 */
class BoundedBlockingQueueTest {

    private BoundedBlockingQueue<Integer> queue;

    @BeforeEach
    void setUp() {
        queue = new BoundedBlockingQueue<>(5);
    }

    // -------------------------------------------------------------------------
    // Basic correctness
    // -------------------------------------------------------------------------

    @Test
    void shouldPreserveFifoOrdering() throws InterruptedException {
        queue.put(1);
        queue.put(2);
        queue.put(3);

        assertEquals(1, queue.take());
        assertEquals(2, queue.take());
        assertEquals(3, queue.take());
    }

    @Test
    void shouldReportCorrectSizeAfterOperations() throws InterruptedException {
        assertEquals(0, queue.size());
        queue.put(1);
        assertEquals(1, queue.size());
        queue.put(2);
        assertEquals(2, queue.size());
        queue.take();
        assertEquals(1, queue.size());
    }

    @Test
    void shouldReportFullAndEmptyCorrectly() throws InterruptedException {
        assertTrue(queue.isEmpty());
        assertFalse(queue.isFull());

        for (int i = 0; i < 5; i++) queue.put(i);

        assertFalse(queue.isEmpty());
        assertTrue(queue.isFull());
    }

    // -------------------------------------------------------------------------
    // Blocking semantics
    // -------------------------------------------------------------------------

    @Test
    @Timeout(3)
    void producerShouldBlockWhenFull() throws InterruptedException {
        // Fill the queue
        for (int i = 0; i < 5; i++) queue.put(i);

        AtomicInteger insertedCount = new AtomicInteger(0);
        Thread producer = new Thread(() -> {
            try {
                queue.put(99);  // should block — queue is full
                insertedCount.incrementAndGet();
            } catch (InterruptedException e) {
                Thread.currentThread().interrupt();
            }
        });

        producer.start();
        Thread.sleep(200);  // give producer time to block

        // Producer must still be blocked — nothing consumed yet
        assertEquals(0, insertedCount.get());
        assertTrue(producer.isAlive());

        // Consume one item — producer should unblock
        queue.take();
        producer.join(1000);

        assertEquals(1, insertedCount.get());
        assertFalse(producer.isAlive());
    }

    @Test
    @Timeout(3)
    void consumerShouldBlockWhenEmpty() throws InterruptedException {
        AtomicInteger consumedCount = new AtomicInteger(0);
        Thread consumer = new Thread(() -> {
            try {
                queue.take();   // should block — queue is empty
                consumedCount.incrementAndGet();
            } catch (InterruptedException e) {
                Thread.currentThread().interrupt();
            }
        });

        consumer.start();
        Thread.sleep(200);

        assertEquals(0, consumedCount.get());
        assertTrue(consumer.isAlive());

        // Produce one item — consumer should unblock
        queue.put(42);
        consumer.join(1000);

        assertEquals(1, consumedCount.get());
    }

    // -------------------------------------------------------------------------
    // Timeout semantics
    // -------------------------------------------------------------------------

    @Test
    @Timeout(3)
    void offerShouldReturnFalseOnTimeout() throws InterruptedException {
        for (int i = 0; i < 5; i++) queue.put(i);  // fill queue

        boolean accepted = queue.offer(99, 200, TimeUnit.MILLISECONDS);
        assertFalse(accepted, "offer() should return false when queue is full and timeout elapses");
    }

    @Test
    @Timeout(3)
    void pollShouldReturnNullOnTimeout() throws InterruptedException {
        Integer result = queue.poll(200, TimeUnit.MILLISECONDS);
        assertNull(result, "poll() should return null when queue is empty and timeout elapses");
    }

    @Test
    @Timeout(3)
    void offerShouldSucceedIfSpaceOpensWithinTimeout() throws InterruptedException {
        for (int i = 0; i < 5; i++) queue.put(i);

        // Consumer removes one item after 100ms — offer should succeed within 500ms
        new Thread(() -> {
            try {
                Thread.sleep(100);
                queue.take();
            } catch (InterruptedException e) {
                Thread.currentThread().interrupt();
            }
        }).start();

        boolean accepted = queue.offer(99, 500, TimeUnit.MILLISECONDS);
        assertTrue(accepted);
    }

    // -------------------------------------------------------------------------
    // Concurrent correctness — the most important tests
    // -------------------------------------------------------------------------

    @Test
    @Timeout(10)
    void shouldNotLoseItemsUnderHighConcurrency() throws InterruptedException {
        int itemCount = 1000;
        int producerCount = 5;
        int consumerCount = 5;
        BoundedBlockingQueue<Integer> concurrentQueue = new BoundedBlockingQueue<>(50);

        List<Integer> consumed = Collections.synchronizedList(new ArrayList<>());
        CountDownLatch allConsumed = new CountDownLatch(itemCount);

        // Producers: each produces itemCount/producerCount items
        ExecutorService producers = Executors.newFixedThreadPool(producerCount);
        for (int p = 0; p < producerCount; p++) {
            final int producerId = p;
            producers.submit(() -> {
                for (int i = 0; i < itemCount / producerCount; i++) {
                    try {
                        concurrentQueue.put(producerId * 1000 + i);
                    } catch (InterruptedException e) {
                        Thread.currentThread().interrupt();
                    }
                }
            });
        }

        // Consumers: drain all items
        ExecutorService consumers = Executors.newFixedThreadPool(consumerCount);
        for (int c = 0; c < consumerCount; c++) {
            consumers.submit(() -> {
                while (true) {
                    try {
                        Integer item = concurrentQueue.poll(500, TimeUnit.MILLISECONDS);
                        if (item != null) {
                            consumed.add(item);
                            allConsumed.countDown();
                        } else if (allConsumed.getCount() == 0) {
                            break;
                        }
                    } catch (InterruptedException e) {
                        Thread.currentThread().interrupt();
                        break;
                    }
                }
            });
        }

        boolean completed = allConsumed.await(8, TimeUnit.SECONDS);
        assertTrue(completed, "Not all items consumed within timeout");
        assertEquals(itemCount, consumed.size(), "Item count mismatch — items were lost or duplicated");

        producers.shutdownNow();
        consumers.shutdownNow();
    }

    // -------------------------------------------------------------------------
    // Shutdown semantics
    // -------------------------------------------------------------------------

    @Test
    void shouldRejectNewItemsAfterShutdown() throws InterruptedException {
        queue.put(1);
        queue.shutdown();

        assertThrows(IllegalStateException.class, () -> queue.put(2));
        assertThrows(IllegalStateException.class,
                () -> queue.offer(2, 100, TimeUnit.MILLISECONDS));
    }

    @Test
    void shouldAllowDrainingAfterShutdown() throws InterruptedException {
        queue.put(1);
        queue.put(2);
        queue.shutdown();

        // Existing items should still be consumable
        assertEquals(1, queue.take());
        assertEquals(2, queue.take());
        assertTrue(queue.isTerminated());
    }

    // -------------------------------------------------------------------------
    // Edge cases
    // -------------------------------------------------------------------------

    @Test
    void shouldRejectNullItems() {
        assertThrows(IllegalArgumentException.class, () -> queue.put(null));
        assertThrows(IllegalArgumentException.class,
                () -> queue.offer(null, 100, TimeUnit.MILLISECONDS));
    }

    @Test
    void shouldRejectZeroCapacity() {
        assertThrows(IllegalArgumentException.class, () -> new BoundedBlockingQueue<>(0));
        assertThrows(IllegalArgumentException.class, () -> new BoundedBlockingQueue<>(-1));
    }

    @Test
    @Timeout(3)
    void shouldHandleInterruptionGracefully() throws InterruptedException {
        // Consumer blocks on empty queue, gets interrupted
        Thread consumer = new Thread(() -> {
            try {
                queue.take();
                fail("Expected InterruptedException");
            } catch (InterruptedException e) {
                // Correct — thread should propagate or handle interruption
                Thread.currentThread().interrupt();
            }
        });

        consumer.start();
        Thread.sleep(100);
        consumer.interrupt();
        consumer.join(1000);

        assertFalse(consumer.isAlive());
    }
}