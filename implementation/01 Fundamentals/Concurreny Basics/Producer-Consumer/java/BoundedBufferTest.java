// BoundedBufferTest.java

import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.Timeout;

import java.util.Collections;
import java.util.HashSet;
import java.util.Set;
import java.util.concurrent.*;
import java.util.concurrent.atomic.AtomicInteger;
import java.util.concurrent.atomic.AtomicLong;

import static org.junit.jupiter.api.Assertions.*;

/**
 * Tests cover:
 * - Basic correctness: items inserted are items received, in order
 * - Backpressure: producer blocks when buffer is full
 * - Concurrency: no items lost or duplicated under concurrent access
 * - Shutdown: poison pill unblocks waiting consumers
 * - Edge cases: null rejection, zero capacity, single item
 */
class BoundedBufferTest {

    // ---- Basic correctness ----

    @Test
    void singleProducerSingleConsumer_itemsReceivedInOrder() throws InterruptedException {
        BoundedBuffer<Integer> buffer = new BoundedBuffer<>(10);

        for (int i = 0; i < 5; i++) buffer.put(i);

        for (int i = 0; i < 5; i++) {
            assertEquals(i, buffer.take());
        }
    }

    @Test
    void nullItemRejected() {
        BoundedBuffer<String> buffer = new BoundedBuffer<>(10);
        assertThrows(IllegalArgumentException.class, () -> buffer.put(null));
    }

    @Test
    void zeroCategoryCapacityRejected() {
        assertThrows(IllegalArgumentException.class, () -> new BoundedBuffer<>(0));
        assertThrows(IllegalArgumentException.class, () -> new BoundedBuffer<>(-1));
    }

    // ---- Backpressure ----

    @Test
    @Timeout(3) // must complete within 3 seconds — confirms no infinite block
    void offer_returnsFalse_whenBufferFullAndTimeout() throws InterruptedException {
        BoundedBuffer<Integer> buffer = new BoundedBuffer<>(2);
        buffer.put(1);
        buffer.put(2); // buffer now full

        // offer with 100ms timeout — should return false quickly, not block forever
        boolean accepted = buffer.offer(3, 100, TimeUnit.MILLISECONDS);
        assertFalse(accepted, "offer should return false when buffer is full");
    }

    @Test
    @Timeout(3)
    void producer_blocksUntilConsumerDrains() throws InterruptedException {
        BoundedBuffer<Integer> buffer = new BoundedBuffer<>(1);
        buffer.put(1); // fill the buffer

        AtomicLong insertedAt = new AtomicLong(-1);

        Thread producer = new Thread(() -> {
            try {
                buffer.put(2); // should block until buffer has space
                insertedAt.set(System.currentTimeMillis());
            } catch (InterruptedException e) {
                Thread.currentThread().interrupt();
            }
        });

        producer.start();
        Thread.sleep(200); // let producer block

        long beforeDrain = System.currentTimeMillis();
        buffer.take(); // drain — gives producer space
        producer.join(1000);

        assertTrue(insertedAt.get() >= beforeDrain,
                "Producer should have unblocked after consumer drained");
    }

    // ---- Concurrency correctness ----

    @Test
    @Timeout(10)
    void multipleProducersConsumers_noItemsLostOrDuplicated() throws InterruptedException {
        int bufferCapacity = 50;
        int itemsPerProducer = 1000;
        int producerCount = 4;
        int consumerCount = 4;
        int totalItems = itemsPerProducer * producerCount;

        BoundedBuffer<Integer> buffer = new BoundedBuffer<>(bufferCapacity);
        Set<Integer> received = Collections.synchronizedSet(new HashSet<>());
        AtomicInteger consumedCount = new AtomicInteger(0);
        CountDownLatch doneLatch = new CountDownLatch(totalItems);

        ExecutorService exec = Executors.newFixedThreadPool(producerCount + consumerCount);

        // Consumers — run until they've received all expected items
        for (int c = 0; c < consumerCount; c++) {
            exec.submit(() -> {
                try {
                    while (consumedCount.get() < totalItems) {
                        Integer item = buffer.take();
                        if (item == null) break; // poison pill
                        received.add(item);
                        consumedCount.incrementAndGet();
                        doneLatch.countDown();
                    }
                } catch (InterruptedException e) {
                    Thread.currentThread().interrupt();
                }
            });
        }

        // Producers — each produces a distinct range of integers
        for (int p = 0; p < producerCount; p++) {
            final int base = p * itemsPerProducer;
            exec.submit(() -> {
                for (int i = 0; i < itemsPerProducer; i++) {
                    try {
                        buffer.put(base + i);
                    } catch (InterruptedException e) {
                        Thread.currentThread().interrupt();
                    }
                }
            });
        }

        boolean completed = doneLatch.await(8, TimeUnit.SECONDS);
        exec.shutdownNow();

        assertTrue(completed, "All items should be consumed within timeout");
        assertEquals(totalItems, received.size(),
                "No items should be duplicated — set size must equal total produced");
    }

    // ---- Shutdown correctness ----

    @Test
    @Timeout(3)
    void shutdown_unblocksWaitingConsumers() throws InterruptedException {
        BoundedBuffer<Integer> buffer = new BoundedBuffer<>(10);
        // buffer is empty — consumer will block on take()

        AtomicInteger nullsReceived = new AtomicInteger(0);
        int consumerCount = 3;
        CountDownLatch exitLatch = new CountDownLatch(consumerCount);

        for (int i = 0; i < consumerCount; i++) {
            new Thread(() -> {
                try {
                    Integer item = buffer.take();
                    if (item == null) nullsReceived.incrementAndGet();
                } catch (InterruptedException e) {
                    Thread.currentThread().interrupt();
                } finally {
                    exitLatch.countDown();
                }
            }).start();
        }

        Thread.sleep(100); // let consumers block
        buffer.shutdown(consumerCount); // insert 3 poison pills

        boolean allExited = exitLatch.await(2, TimeUnit.SECONDS);
        assertTrue(allExited, "All consumers should exit after shutdown");
        assertEquals(consumerCount, nullsReceived.get(),
                "Each consumer should receive exactly one poison pill");
    }

    @Test
    @Timeout(3)
    void shutdown_itemsAlreadyInBuffer_consumedBeforeExit() throws InterruptedException {
        BoundedBuffer<Integer> buffer = new BoundedBuffer<>(10);
        buffer.put(1);
        buffer.put(2);
        buffer.put(3);

        AtomicInteger consumed = new AtomicInteger(0);
        CountDownLatch exitLatch = new CountDownLatch(1);

        Thread consumer = new Thread(() -> {
            try {
                while (true) {
                    Integer item = buffer.take();
                    if (item == null) break;
                    consumed.incrementAndGet();
                }
            } catch (InterruptedException e) {
                Thread.currentThread().interrupt();
            } finally {
                exitLatch.countDown();
            }
        });
        consumer.start();

        Thread.sleep(50); // let consumer drain
        buffer.shutdown(1);

        exitLatch.await(2, TimeUnit.SECONDS);
        assertEquals(3, consumed.get(), "All 3 items should be consumed before consumer exits");
    }

    @Test
    void putAfterShutdown_returnsFalse() throws InterruptedException {
        BoundedBuffer<Integer> buffer = new BoundedBuffer<>(10);
        buffer.shutdown(0);
        assertFalse(buffer.put(1), "put() should return false after shutdown");
    }

    // ---- Observability ----

    @Test
    void sizeAndRemainingCapacity_reflectState() throws InterruptedException {
        BoundedBuffer<Integer> buffer = new BoundedBuffer<>(5);
        assertEquals(0, buffer.size());
        assertEquals(5, buffer.remainingCapacity());

        buffer.put(1);
        buffer.put(2);
        assertEquals(2, buffer.size());
        assertEquals(3, buffer.remainingCapacity());

        buffer.take();
        assertEquals(1, buffer.size());
        assertEquals(4, buffer.remainingCapacity());
    }
}