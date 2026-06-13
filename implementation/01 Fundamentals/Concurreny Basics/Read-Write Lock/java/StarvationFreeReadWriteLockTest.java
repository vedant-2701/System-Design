// StarvationFreeReadWriteLockTest.java

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
 * Tests for StarvationFreeReadWriteLock.
 *
 * Test categories:
 *   1. Basic correctness — acquire/release mechanics
 *   2. Concurrency correctness — multiple readers, exclusive writer
 *   3. Starvation prevention — writers not starved by continuous readers
 *   4. Timeout behaviour — tryLock returns false when lock unavailable
 *   5. Error handling — illegal state exceptions
 *   6. Interrupt handling — interrupted threads clean up state correctly
 */
class StarvationFreeReadWriteLockTest {

    private StarvationFreeReadWriteLock lock;

    @BeforeEach
    void setUp() {
        lock = new StarvationFreeReadWriteLock();
    }

    // -------------------------------------------------------------------------
    // 1. Basic Correctness
    // -------------------------------------------------------------------------

    @Test
    void singleReaderAcquiresAndReleases() throws InterruptedException {
        lock.lockRead();
        var state = lock.snapshot();
        assertEquals(1, state.activeReaders());
        assertFalse(state.isWriting());
        lock.unlockRead();

        state = lock.snapshot();
        assertEquals(0, state.activeReaders());
    }

    @Test
    void singleWriterAcquiresAndReleases() throws InterruptedException {
        lock.lockWrite();
        var state = lock.snapshot();
        assertTrue(state.isWriting());
        assertEquals(0, state.activeReaders());
        lock.unlockWrite();

        state = lock.snapshot();
        assertFalse(state.isWriting());
    }

    @Test
    void multipleReadersAcquireSimultaneously() throws InterruptedException {
        int readerCount = 5;
        CountDownLatch allReading = new CountDownLatch(readerCount);
        CountDownLatch release   = new CountDownLatch(1);
        ExecutorService pool = Executors.newFixedThreadPool(readerCount);

        for (int i = 0; i < readerCount; i++) {
            pool.submit(() -> {
                try {
                    lock.lockRead();
                    allReading.countDown();
                    release.await();
                    lock.unlockRead();
                } catch (InterruptedException e) {
                    Thread.currentThread().interrupt();
                }
            });
        }

        assertTrue(allReading.await(2, TimeUnit.SECONDS),
            "All readers should acquire lock simultaneously");

        assertEquals(readerCount, lock.snapshot().activeReaders());

        release.countDown();
        pool.shutdown();
        pool.awaitTermination(2, TimeUnit.SECONDS);
    }

    // -------------------------------------------------------------------------
    // 2. Concurrency Correctness — Writer Exclusivity
    // -------------------------------------------------------------------------

    @Test
    @Timeout(5)
    void writerExcludesAllReaders() throws InterruptedException {
        // Writer holds lock; reader must block until writer releases
        CountDownLatch writerHolding = new CountDownLatch(1);
        CountDownLatch readerDone    = new CountDownLatch(1);
        List<String> events = Collections.synchronizedList(new ArrayList<>());

        Thread writer = new Thread(() -> {
            try {
                lock.lockWrite();
                events.add("writer-acquired");
                writerHolding.countDown();
                Thread.sleep(100);
                events.add("writer-releasing");
                lock.unlockWrite();
            } catch (InterruptedException e) {
                Thread.currentThread().interrupt();
            }
        });

        Thread reader = new Thread(() -> {
            try {
                writerHolding.await();
                lock.lockRead();
                events.add("reader-acquired");
                lock.unlockRead();
                readerDone.countDown();
            } catch (InterruptedException e) {
                Thread.currentThread().interrupt();
            }
        });

        writer.start();
        reader.start();
        readerDone.await();

        assertEquals(List.of("writer-acquired", "writer-releasing", "reader-acquired"), events,
            "Reader must not acquire lock while writer holds it");
    }

    @Test
    @Timeout(5)
    void writersAreExclusiveOfEachOther() throws InterruptedException {
        // Second writer must wait for first to finish
        CountDownLatch firstWriterHolding = new CountDownLatch(1);
        CountDownLatch bothDone           = new CountDownLatch(2);
        List<String> events = Collections.synchronizedList(new ArrayList<>());

        Thread w1 = new Thread(() -> {
            try {
                lock.lockWrite();
                events.add("w1-acquired");
                firstWriterHolding.countDown();
                Thread.sleep(100);
                events.add("w1-releasing");
                lock.unlockWrite();
                bothDone.countDown();
            } catch (InterruptedException e) {
                Thread.currentThread().interrupt();
            }
        });

        Thread w2 = new Thread(() -> {
            try {
                firstWriterHolding.await();
                lock.lockWrite();
                events.add("w2-acquired");
                lock.unlockWrite();
                bothDone.countDown();
            } catch (InterruptedException e) {
                Thread.currentThread().interrupt();
            }
        });

        w1.start();
        w2.start();
        bothDone.await();

        assertEquals(List.of("w1-acquired", "w1-releasing", "w2-acquired"), events);
    }

    // -------------------------------------------------------------------------
    // 3. Starvation Prevention
    // -------------------------------------------------------------------------

    volatile boolean[] stop    = {false};
    @Test
    @Timeout(10)
    void writerNotStarvedByContinuousReaders() throws InterruptedException {
        // Continuous stream of readers — writer must still eventually proceed
        AtomicInteger readerCount  = new AtomicInteger(0);
        CountDownLatch writerDone  = new CountDownLatch(1);

        // Flood of readers
        ExecutorService readers = Executors.newFixedThreadPool(10);
        for (int i = 0; i < 10; i++) {
            readers.submit(() -> {
                while (!stop[0]) {
                    try {
                        if (lock.tryLockRead(10, TimeUnit.MILLISECONDS)) {
                            readerCount.incrementAndGet();
                            Thread.sleep(5);
                            lock.unlockRead();
                        }
                    } catch (InterruptedException e) {
                        Thread.currentThread().interrupt();
                        return;
                    }
                }
            });
        }

        Thread.sleep(50); // let readers establish themselves

        // Writer should acquire within reasonable time despite active readers
        Thread writer = new Thread(() -> {
            try {
                lock.lockWrite();
                lock.unlockWrite();
                writerDone.countDown();
            } catch (InterruptedException e) {
                Thread.currentThread().interrupt();
            }
        });

        writer.start();

        boolean writerSucceeded = writerDone.await(5, TimeUnit.SECONDS);

        stop[0] = true;
        readers.shutdown();
        readers.awaitTermination(2, TimeUnit.SECONDS);

        assertTrue(writerSucceeded, "Writer was starved — starvation prevention is broken");
    }

    // -------------------------------------------------------------------------
    // 4. Timeout Behaviour
    // -------------------------------------------------------------------------

    @Test
    @Timeout(5)
    void tryLockReadReturnsFalseWhenWriterHoldsLock() throws InterruptedException {
        CountDownLatch writerHolding = new CountDownLatch(1);
        CountDownLatch releaseWriter = new CountDownLatch(1);

        Thread writer = new Thread(() -> {
            try {
                lock.lockWrite();
                writerHolding.countDown();
                releaseWriter.await();
                lock.unlockWrite();
            } catch (InterruptedException e) {
                Thread.currentThread().interrupt();
            }
        });

        writer.start();
        writerHolding.await();

        boolean acquired = lock.tryLockRead(100, TimeUnit.MILLISECONDS);
        assertFalse(acquired, "tryLockRead should return false when writer holds lock");

        releaseWriter.countDown();
        writer.join();
    }

    @Test
    @Timeout(5)
    void tryLockWriteReturnsFalseWhenReadersActive() throws InterruptedException {
        CountDownLatch readerHolding = new CountDownLatch(1);
        CountDownLatch releaseReader = new CountDownLatch(1);

        Thread reader = new Thread(() -> {
            try {
                lock.lockRead();
                readerHolding.countDown();
                releaseReader.await();
                lock.unlockRead();
            } catch (InterruptedException e) {
                Thread.currentThread().interrupt();
            }
        });

        reader.start();
        readerHolding.await();

        boolean acquired = lock.tryLockWrite(100, TimeUnit.MILLISECONDS);
        assertFalse(acquired, "tryLockWrite should return false when readers hold lock");

        releaseReader.countDown();
        reader.join();
    }

    @Test
    @Timeout(5)
    void tryLockReadSucceedsWhenLockFree() throws InterruptedException {
        boolean acquired = lock.tryLockRead(100, TimeUnit.MILLISECONDS);
        assertTrue(acquired);
        lock.unlockRead();
    }

    // -------------------------------------------------------------------------
    // 5. Error Handling
    // -------------------------------------------------------------------------

    @Test
    void unlockReadWithoutLockThrows() {
        assertThrows(IllegalStateException.class, () -> lock.unlockRead());
    }

    @Test
    void unlockWriteWithoutLockThrows() {
        assertThrows(IllegalStateException.class, () -> lock.unlockWrite());
    }

    @Test
    void doubleUnlockReadThrows() throws InterruptedException {
        lock.lockRead();
        lock.unlockRead();
        assertThrows(IllegalStateException.class, () -> lock.unlockRead());
    }

    // -------------------------------------------------------------------------
    // 6. Interrupt Handling — state must be consistent after interrupt
    // -------------------------------------------------------------------------

    @Test
    @Timeout(5)
    void interruptedWriterDoesNotCorruptWaitingWritersCount() throws InterruptedException {
        // A writer acquires lock; second writer waits and gets interrupted.
        // After interrupt, waitingWriters must return to 0 — readers must unblock.
        CountDownLatch w1Holding   = new CountDownLatch(1);
        CountDownLatch w2Waiting   = new CountDownLatch(1);
        CountDownLatch readerDone  = new CountDownLatch(1);

        Thread w1 = new Thread(() -> {
            try {
                lock.lockWrite();
                w1Holding.countDown();
                Thread.sleep(500); // hold lock while w2 queues and gets interrupted
                lock.unlockWrite();
            } catch (InterruptedException e) {
                Thread.currentThread().interrupt();
            }
        });

        Thread w2 = new Thread(() -> {
            try {
                w1Holding.await();
                w2Waiting.countDown();
                lock.lockWrite(); // will block — w1 holds lock
                lock.unlockWrite();
            } catch (InterruptedException e) {
                // Expected — interrupted while waiting
                // waitingWriters must be decremented in finally block
            }
        });

        w1.start();
        w2.start();
        w2Waiting.await();
        Thread.sleep(50); // ensure w2 is blocked in await()
        w2.interrupt();
        w2.join();

        // After w1 releases, a reader should be able to acquire.
        // If waitingWriters leaked (stayed at 1), reader would be permanently blocked.
        Thread reader = new Thread(() -> {
            try {
                lock.lockRead();
                lock.unlockRead();
                readerDone.countDown();
            } catch (InterruptedException e) {
                Thread.currentThread().interrupt();
            }
        });

        reader.start();
        boolean readerSucceeded = readerDone.await(3, TimeUnit.SECONDS);
        assertTrue(readerSucceeded,
            "Reader blocked permanently — waitingWriters leaked after interrupt");

        w1.join();
        reader.join();
    }
}