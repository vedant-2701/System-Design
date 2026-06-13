// PoolState.java

/**
 * Lifecycle states of the thread pool.
 *
 * Valid transitions (one-way — no going back):
 *
 *   RUNNING → SHUTDOWN → TERMINATED
 *   RUNNING → STOP     → TERMINATED
 *
 * RUNNING:    Accepts new tasks. Workers are processing.
 * SHUTDOWN:   No new tasks accepted. Queued tasks still execute. Workers drain queue.
 * STOP:       No new tasks accepted. Queue cleared. Workers interrupted.
 * TERMINATED: All workers have exited. Pool is fully stopped.
 *
 * State is stored in an AtomicInteger in the pool for CAS-based transitions.
 * Using ordinal() comparison allows "at least SHUTDOWN" checks without
 * enumerating states explicitly.
 */
public enum PoolState {
    RUNNING,
    SHUTDOWN,
    STOP,
    TERMINATED;

    public boolean isAtLeast(PoolState other) {
        return this.ordinal() >= other.ordinal();
    }
}