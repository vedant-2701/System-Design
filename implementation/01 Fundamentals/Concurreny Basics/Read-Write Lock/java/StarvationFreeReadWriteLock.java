// StarvationFreeReadWriteLock.java

import java.util.concurrent.TimeUnit;
import java.util.concurrent.locks.Condition;
import java.util.concurrent.locks.ReentrantLock;

/**
 * Read-Write Lock implementation with writer starvation prevention.
 *
 * State model:
 *   activeReaders  — threads currently reading
 *   waitingWriters — threads queued to write (drives starvation prevention)
 *   isWriting      — a writer currently holds the lock
 *
 * Acquisition rules (derived from state model):
 *   Reader acquires when: !isWriting && waitingWriters == 0
 *   Writer acquires when: !isWriting && activeReaders == 0
 *
 * All state transitions are guarded by a single ReentrantLock.
 * Threads wait on Conditions and re-check guards in while loops
 * to handle spurious wakeups correctly.
 */
public class StarvationFreeReadWriteLock implements ReadWriteLock {

    private final ReentrantLock mutex = new ReentrantLock();

    // Signalled when a reader releases — writers may proceed
    private final Condition writerReady = mutex.newCondition();

    // Signalled when a writer releases — readers and next writer may proceed
    private final Condition readerReady = mutex.newCondition();

    private int activeReaders  = 0;
    private int waitingWriters = 0;
    private boolean isWriting  = false;

    // -------------------------------------------------------------------------
    // Read Lock
    // -------------------------------------------------------------------------

    @Override
    public void lockRead() throws InterruptedException {
        mutex.lockInterruptibly();
        try {
            // Block new readers while writers are queued or active.
            // while loop — not if — to handle spurious wakeups correctly.
            while (isWriting || waitingWriters > 0) {
                readerReady.await();
            }
            activeReaders++;
        } finally {
            mutex.unlock();
        }
    }

    @Override
    public boolean tryLockRead(long timeout, TimeUnit unit) throws InterruptedException {
        long remainingNanos = unit.toNanos(timeout);
        mutex.lockInterruptibly();
        try {
            while (isWriting || waitingWriters > 0) {
                if (remainingNanos <= 0) {
                    return false;
                }
                remainingNanos = readerReady.awaitNanos(remainingNanos);
            }
            activeReaders++;
            return true;
        } finally {
            mutex.unlock();
        }
    }

    @Override
    public void unlockRead() {
        mutex.lock();
        try {
            if (activeReaders <= 0) {
                throw new IllegalStateException(
                    "unlockRead() called but no read lock is held. " +
                    "Ensure lockRead() was called before unlockRead()."
                );
            }
            activeReaders--;

            // Last reader leaving — signal waiting writers.
            // No point signalling readers here; they are blocked by waitingWriters > 0.
            if (activeReaders == 0 && waitingWriters > 0) {
                writerReady.signal();  // only one writer can proceed — signal, not signalAll
            }
        } finally {
            mutex.unlock();
        }
    }

    // -------------------------------------------------------------------------
    // Write Lock
    // -------------------------------------------------------------------------

    @Override
    public void lockWrite() throws InterruptedException {
        mutex.lockInterruptibly();
        try {
            waitingWriters++;  // register intent — blocks new readers immediately
            try {
                while (isWriting || activeReaders > 0) {
                    writerReady.await();
                }
            } finally {
                // Decrement regardless of whether await threw InterruptedException.
                // Failing to decrement would permanently suppress new readers.
                waitingWriters--;
            }
            isWriting = true;
        } finally {
            mutex.unlock();
        }
    }

    @Override
    public boolean tryLockWrite(long timeout, TimeUnit unit) throws InterruptedException {
        long remainingNanos = unit.toNanos(timeout);
        mutex.lockInterruptibly();
        try {
            waitingWriters++;
            try {
                while (isWriting || activeReaders > 0) {
                    if (remainingNanos <= 0) {
                        return false;
                    }
                    remainingNanos = writerReady.awaitNanos(remainingNanos);
                }
            } finally {
                waitingWriters--;
            }
            isWriting = true;
            return true;
        } finally {
            mutex.unlock();
        }
    }

    @Override
    public void unlockWrite() {
        mutex.lock();
        try {
            if (!isWriting) {
                throw new IllegalStateException(
                    "unlockWrite() called but write lock is not held. " +
                    "Ensure lockWrite() was called before unlockWrite()."
                );
            }
            isWriting = false;

            if (waitingWriters > 0) {
                // Prefer queued writers over new readers — writer-preference policy.
                // Prevents newly arriving readers from starving the writer queue.
                writerReady.signal();
            } else {
                // No writers waiting — wake all blocked readers.
                readerReady.signalAll();
            }
        } finally {
            mutex.unlock();
        }
    }

    // -------------------------------------------------------------------------
    // Diagnostics — useful for testing and debugging
    // -------------------------------------------------------------------------

    /**
     * Returns a snapshot of internal lock state.
     * Not guaranteed to be consistent with actual state after the call returns.
     * Intended for logging, debugging, and testing only.
     */
    public LockState snapshot() {
        mutex.lock();
        try {
            return new LockState(activeReaders, waitingWriters, isWriting);
        } finally {
            mutex.unlock();
        }
    }

    public record LockState(int activeReaders, int waitingWriters, boolean isWriting) {
        @Override
        public String toString() {
            return String.format(
                "LockState{activeReaders=%d, waitingWriters=%d, isWriting=%b}",
                activeReaders, waitingWriters, isWriting
            );
        }
    }
}