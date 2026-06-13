// LRUCacheTest.java
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import java.util.concurrent.*;
import java.util.concurrent.atomic.AtomicInteger;
import static org.junit.jupiter.api.Assertions.*;

class LRUCacheTest {

    private LRUCache<String, Integer> cache;

    @BeforeEach
    void setUp() {
        cache = new LRUCache<>(3);
    }

    // --- Functional correctness ---

    @Test
    void get_returnsNull_whenKeyNotPresent() {
        assertNull(cache.get("missing"));
    }

    @Test
    void put_and_get_basicOperation() {
        cache.put("a", 1);
        assertEquals(1, cache.get("a"));
    }

    @Test
    void put_updatesExistingKey_withoutGrowingCache() {
        cache.put("a", 1);
        cache.put("a", 99);
        assertEquals(99, cache.get("a"));
        assertEquals(1, cache.size());
    }

    @Test
    void eviction_removesLeastRecentlyUsed() {
        cache.put("a", 1);
        cache.put("b", 2);
        cache.put("c", 3);
        // cache full: a(LRU) ← b ← c(MRU)

        cache.put("d", 4); // should evict "a"

        assertNull(cache.get("a"));  // evicted
        assertEquals(2, cache.get("b"));
        assertEquals(3, cache.get("c"));
        assertEquals(4, cache.get("d"));
    }

    @Test
    void get_refreshesRecency_preventingEviction() {
        cache.put("a", 1);
        cache.put("b", 2);
        cache.put("c", 3);

        cache.get("a"); // "a" is now MRU — "b" becomes LRU

        cache.put("d", 4); // should evict "b", not "a"

        assertNull(cache.get("b"));  // evicted
        assertEquals(1, cache.get("a")); // still present
    }

    @Test
    void eviction_worksCorrectly_atCapacityOne() {
        LRUCache<String, Integer> singleSlot = new LRUCache<>(1);
        singleSlot.put("a", 1);
        singleSlot.put("b", 2); // evicts "a"

        assertNull(singleSlot.get("a"));
        assertEquals(2, singleSlot.get("b"));
    }

    @Test
    void constructor_throwsException_onZeroCapacity() {
        assertThrows(IllegalArgumentException.class, () -> new LRUCache<>(0));
    }

    @Test
    void constructor_throwsException_onNegativeCapacity() {
        assertThrows(IllegalArgumentException.class, () -> new LRUCache<>(-5));
    }

    @Test
    void put_throwsException_onNullKey() {
        assertThrows(IllegalArgumentException.class, () -> cache.put(null, 1));
    }

    @Test
    void clear_emptiesCache() {
        cache.put("a", 1);
        cache.put("b", 2);
        cache.clear();
        assertEquals(0, cache.size());
        assertNull(cache.get("a"));
    }

    // --- Concurrency correctness ---

    @Test
    void concurrentPuts_doNotCorruptCache() throws InterruptedException {
        LRUCache<Integer, Integer> concurrentCache = new LRUCache<>(100);
        int threadCount = 20;
        int operationsPerThread = 500;

        ExecutorService executor = Executors.newFixedThreadPool(threadCount);
        CountDownLatch startLatch = new CountDownLatch(1);
        CountDownLatch doneLatch = new CountDownLatch(threadCount);

        for (int t = 0; t < threadCount; t++) {
            final int threadId = t;
            executor.submit(() -> {
                try {
                    startLatch.await(); // all threads start simultaneously
                    for (int i = 0; i < operationsPerThread; i++) {
                        concurrentCache.put(threadId * operationsPerThread + i, i);
                    }
                } catch (InterruptedException e) {
                    Thread.currentThread().interrupt();
                } finally {
                    doneLatch.countDown();
                }
            });
        }

        startLatch.countDown(); // release all threads at once
        doneLatch.await(10, TimeUnit.SECONDS);
        executor.shutdown();

        // Cache must not exceed capacity — no corruption
        assertTrue(concurrentCache.size() <= 100);
    }

    @Test
    void concurrentGetsAndPuts_produceNoExceptions() throws InterruptedException {
        LRUCache<String, Integer> concurrentCache = new LRUCache<>(50);
        AtomicInteger errorCount = new AtomicInteger(0);

        // Pre-populate
        for (int i = 0; i < 50; i++) {
            concurrentCache.put("key" + i, i);
        }

        ExecutorService executor = Executors.newFixedThreadPool(10);
        CountDownLatch done = new CountDownLatch(100);

        // 50 readers, 50 writers
        for (int i = 0; i < 50; i++) {
            final int idx = i;
            executor.submit(() -> {
                try {
                    concurrentCache.get("key" + idx);
                } catch (Exception e) {
                    errorCount.incrementAndGet();
                } finally {
                    done.countDown();
                }
            });
            executor.submit(() -> {
                try {
                    concurrentCache.put("key" + idx, idx * 2);
                } catch (Exception e) {
                    errorCount.incrementAndGet();
                } finally {
                    done.countDown();
                }
            });
        }

        done.await(10, TimeUnit.SECONDS);
        executor.shutdown();
        assertEquals(0, errorCount.get());
    }
}