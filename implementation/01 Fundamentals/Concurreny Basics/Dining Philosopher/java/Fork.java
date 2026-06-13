// Fork.java

import java.util.concurrent.locks.ReentrantLock;

/**
 * Represents a single fork on the table.
 *
 * A fork is modeled as a ReentrantLock rather than a boolean flag.
 * A boolean flag requires a separate synchronization mechanism to be
 * read and written atomically. A lock handles both — the lock state
 * IS the fork state. No extra synchronization needed.
 *
 * The fork has an ID purely for logging/debugging — identifying
 * which fork is causing contention in a production system matters.
 */
public class Fork {

    private final int id;
    private final ReentrantLock lock;

    public Fork(int id) {
        this.id = id;
        this.lock = new ReentrantLock();
    }

    /**
     * Acquire this fork. Blocks until available.
     * The calling thread owns this fork until pickDown() is called.
     */
    public void pickUp() {
        lock.lock();
    }

    /**
     * Release this fork. Only the thread that acquired it may call this.
     * ReentrantLock enforces ownership — an unrelated thread cannot release it.
     */
    public void putDown() {
        lock.unlock();
    }

    public int getId() {
        return id;
    }

    @Override
    public String toString() {
        return "Fork-" + id;
    }
}