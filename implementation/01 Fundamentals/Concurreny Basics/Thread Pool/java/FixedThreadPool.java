// FixedThreadPool.java

import java.util.concurrent.ArrayBlockingQueue;
import java.util.concurrent.BlockingQueue;
import java.util.concurrent.CountDownLatch;
import java.util.concurrent.TimeUnit;
import java.util.concurrent.atomic.AtomicInteger;
import java.util.concurrent.atomic.AtomicReference;
import java.util.logging.Logger;

/**
 * A fixed-size thread pool with a bounded task queue and pluggable rejection policy.
 *
 * Thread safety model:
 * - poolState: AtomicReference — CAS transitions, no lock needed
 * - activeWorkerCount: AtomicInteger — lock-free increment/decrement
 * - taskQueue: ArrayBlockingQueue — internally thread-safe
 * - terminationLatch: CountDownLatch — workers count down on exit, awaitTermination waits
 *
 * Why no single global lock?
 * A global lock on submit() would serialize all callers — defeating the purpose.
 * Instead, each shared field has its own appropriate synchronization primitive.
 *
 * Why CountDownLatch for termination?
 * awaitTermination() needs to block until ALL workers exit.
 * CountDownLatch initialized to workerCount — each worker decrements on exit.
 * When it reaches 0, all waiting threads are unblocked atomically.
 * Alternative (joining each thread) works but is more complex to manage.
 */
public class FixedThreadPool implements ThreadPool {

    private static final Logger log = Logger.getLogger(FixedThreadPool.class.getName());

    private final int poolSize;
    private final BlockingQueue<Runnable> taskQueue;
    private final RejectionPolicy rejectionPolicy;
    private final AtomicReference<PoolState> poolState;
    private final AtomicInteger activeWorkerCount;
    private final CountDownLatch terminationLatch;
    private final Thread[] workers;

    private FixedThreadPool(Builder builder) {
        this.poolSize = builder.poolSize;
        this.taskQueue = new ArrayBlockingQueue<>(builder.queueCapacity);
        this.rejectionPolicy = builder.rejectionPolicy;
        this.poolState = new AtomicReference<>(PoolState.RUNNING);
        this.activeWorkerCount = new AtomicInteger(0);
        this.terminationLatch = new CountDownLatch(poolSize);
        this.workers = new Thread[poolSize];

        startWorkers();
    }

    private void startWorkers() {
        for (int i = 0; i < poolSize; i++) {
            Worker worker = new Worker(taskQueue, activeWorkerCount, this, i);
            Thread thread = new Thread(worker, "pool-worker-" + i);
            thread.setDaemon(false); // workers must finish — not daemon threads
            workers[i] = thread;
            thread.start();
        }
        log.info(() -> "Thread pool started with " + poolSize + " workers, queue capacity " + taskQueue.remainingCapacity());
    }

    @Override
    public void submit(Runnable task) {
        if (task == null) throw new NullPointerException("task must not be null");

        // Read state once — avoids TOCTOU between check and enqueue.
        // Even so, this is not fully atomic with taskQueue.offer().
        // A task submitted exactly as shutdown() is called may or may not be accepted —
        // this is acceptable behavior, consistent with java.util.concurrent.ThreadPoolExecutor.
        PoolState currentState = poolState.get();
        if (currentState.isAtLeast(PoolState.SHUTDOWN)) {
            rejectionPolicy.onRejection(task, this);
            return;
        }

        // offer() is non-blocking — returns false immediately if queue is full.
        // We do NOT use put() here because blocking the caller on a full queue
        // is the rejection policy's responsibility, not the pool's default behavior.
        boolean accepted = taskQueue.offer(task);
        if (!accepted) {
            rejectionPolicy.onRejection(task, this);
        }
    }

    @Override
    public void shutdown() {
        // CAS: only transition from RUNNING to SHUTDOWN.
        // If already SHUTDOWN or STOP, do nothing.
        if (poolState.compareAndSet(PoolState.RUNNING, PoolState.SHUTDOWN)) {
            log.info("Thread pool shutdown initiated — draining queue");
            // Workers will drain the queue naturally and then exit via workerLoop().
            // No need to interrupt them.
        }
    }

    @Override
    public void shutdownNow() {
        // Allow transition from RUNNING or SHUTDOWN to STOP.
        PoolState previous;
        do {
            previous = poolState.get();
            if (previous.isAtLeast(PoolState.STOP)) return; // already stopping
        } while (!poolState.compareAndSet(previous, PoolState.STOP));

        log.info("Thread pool shutdownNow initiated — interrupting workers");

        // Clear the queue — abandon pending tasks.
        taskQueue.clear();

        // Interrupt all workers. Workers blocked in poll() will get InterruptedException
        // and exit. Workers mid-task will have their interrupt flag set — task may or
        // may not respond, depending on its implementation.
        for (Thread worker : workers) {
            worker.interrupt();
        }
    }

    @Override
    public boolean awaitTermination(long timeout, TimeUnit unit) throws InterruptedException {
        return terminationLatch.await(timeout, unit);
    }

    /**
     * Called by each Worker when it exits its run loop.
     * When all workers have called this, the pool transitions to TERMINATED
     * and all threads waiting in awaitTermination() are released.
     */
    void onWorkerExit() {
        terminationLatch.countDown();

        // If this was the last worker, transition to TERMINATED.
        if (terminationLatch.getCount() == 0) {
            poolState.set(PoolState.TERMINATED);
            log.info("Thread pool terminated — all workers exited");
        }
    }

    PoolState getState() {
        return poolState.get();
    }

    @Override
    public boolean isShutdown() {
        return poolState.get().isAtLeast(PoolState.SHUTDOWN);
    }

    @Override
    public boolean isTerminated() {
        return poolState.get() == PoolState.TERMINATED;
    }

    @Override
    public int getQueueSize() {
        return taskQueue.size();
    }

    @Override
    public int getActiveWorkerCount() {
        return activeWorkerCount.get();
    }

    // -------------------------------------------------------------------------
    // Builder
    // -------------------------------------------------------------------------

    public static Builder builder() {
        return new Builder();
    }

    public static class Builder {
        private int poolSize = Runtime.getRuntime().availableProcessors();
        private int queueCapacity = 100;
        private RejectionPolicy rejectionPolicy = new RejectionPolicy.AbortPolicy();

        public Builder poolSize(int poolSize) {
            if (poolSize <= 0) throw new IllegalArgumentException("poolSize must be > 0");
            this.poolSize = poolSize;
            return this;
        }

        public Builder queueCapacity(int queueCapacity) {
            if (queueCapacity <= 0) throw new IllegalArgumentException("queueCapacity must be > 0");
            this.queueCapacity = queueCapacity;
            return this;
        }

        public Builder rejectionPolicy(RejectionPolicy policy) {
            if (policy == null) throw new NullPointerException("rejectionPolicy must not be null");
            this.rejectionPolicy = policy;
            return this;
        }

        public FixedThreadPool build() {
            return new FixedThreadPool(this);
        }
    }
}