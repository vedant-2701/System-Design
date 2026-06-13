// ReadWriteLock.java

import java.util.concurrent.TimeUnit;

/**
 * A Read-Write Lock that allows multiple concurrent readers
 * or exactly one exclusive writer.
 *
 * Guarantees:
 * - Multiple readers can hold the lock simultaneously
 * - A writer gets exclusive access — no readers, no other writers
 * - Writer starvation prevention — queued writers block new readers
 *
 * Acquiring threads block until their condition is satisfied.
 * Interrupted threads receive InterruptedException cleanly.
 */
public interface ReadWriteLock {

    /**
     * Acquires the read lock.
     * Blocks if a writer holds the lock or writers are waiting.
     *
     * @throws InterruptedException if the thread is interrupted while waiting
     */
    void lockRead() throws InterruptedException;

    /**
     * Attempts to acquire the read lock within the given timeout.
     *
     * @return true if acquired, false if timeout elapsed
     * @throws InterruptedException if interrupted while waiting
     */
    boolean tryLockRead(long timeout, TimeUnit unit) throws InterruptedException;

    /**
     * Releases the read lock.
     * Must be called by the same thread that called lockRead().
     *
     * @throws IllegalStateException if no read lock is currently held
     */
    void unlockRead();

    /**
     * Acquires the write lock.
     * Blocks until all readers finish and no other writer holds the lock.
     *
     * @throws InterruptedException if the thread is interrupted while waiting
     */
    void lockWrite() throws InterruptedException;

    /**
     * Attempts to acquire the write lock within the given timeout.
     *
     * @return true if acquired, false if timeout elapsed
     * @throws InterruptedException if interrupted while waiting
     */
    boolean tryLockWrite(long timeout, TimeUnit unit) throws InterruptedException;

    /**
     * Releases the write lock.
     *
     * @throws IllegalStateException if the write lock is not currently held
     */
    void unlockWrite();
}