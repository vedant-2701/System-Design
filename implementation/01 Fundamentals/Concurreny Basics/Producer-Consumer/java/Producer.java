// Producer.java

import java.util.concurrent.TimeUnit;
import java.util.concurrent.atomic.AtomicLong;
import java.util.function.Supplier;
import java.util.logging.Logger;

/**
 * A producer that pulls items from a supplier and pushes them into a BoundedBuffer.
 *
 * Design decisions:
 * - Supplier<T> decouples item generation from queue mechanics — testable and extensible.
 * - AtomicLong counters expose observability without locking.
 * - Distinguishes InterruptedException (external stop signal) from supplier errors
 *   (recoverable per-item failures that should not kill the producer thread).
 * - producedCount and rejectedCount give operators visibility into throughput
 *   and backpressure effectiveness.
 */
public class Producer<T> implements Runnable {

    private static final Logger log = Logger.getLogger(Producer.class.getName());

    private final String name;
    private final BoundedBuffer<T> buffer;
    private final Supplier<T> itemSupplier;
    private final long produceIntervalMs;

    // Observability counters — AtomicLong avoids synchronization on hot path
    private final AtomicLong producedCount  = new AtomicLong(0);
    private final AtomicLong rejectedCount  = new AtomicLong(0);  // buffer full or shut down
    private final AtomicLong errorCount     = new AtomicLong(0);  // supplier errors

    /**
     * @param name              human-readable name for logging
     * @param buffer            target bounded buffer
     * @param itemSupplier      produces the next item to enqueue
     * @param produceIntervalMs delay between produce attempts (0 = as fast as possible)
     */
    public Producer(String name, BoundedBuffer<T> buffer,
                    Supplier<T> itemSupplier, long produceIntervalMs) {
        this.name = name;
        this.buffer = buffer;
        this.itemSupplier = itemSupplier;
        this.produceIntervalMs = produceIntervalMs;
    }

    @Override
    public void run() {
        log.info(name + " started");

        try {
            while (!Thread.currentThread().isInterrupted() && !buffer.isShutdown()) {
                T item;
                try {
                    item = itemSupplier.get();
                } catch (Exception e) {
                    // Supplier error — log and continue; don't kill the producer thread
                    errorCount.incrementAndGet();
                    log.warning(name + " supplier error: " + e.getMessage());
                    continue;
                }

                // offer() with timeout provides backpressure: if buffer stays full
                // for 500ms, we record a rejection and loop — checking shutdown again.
                // This prevents producers from blocking indefinitely during slow shutdown.
                boolean accepted = buffer.offer(item, 500, TimeUnit.MILLISECONDS);

                if (accepted) {
                    producedCount.incrementAndGet();
                    log.fine(name + " produced: " + item);
                } else {
                    rejectedCount.incrementAndGet();
                    log.fine(name + " rejected (buffer full or shutdown): " + item);
                }

                if (produceIntervalMs > 0) {
                    Thread.sleep(produceIntervalMs);
                }
            }
        } catch (InterruptedException e) {
            // Interrupted — clean exit, restore interrupt flag for callers
            Thread.currentThread().interrupt();
            log.info(name + " interrupted — stopping");
        }

        log.info(name + " stopped. produced=" + producedCount
                + " rejected=" + rejectedCount
                + " errors=" + errorCount);
    }

    // ---- Observability ----

    public long getProducedCount()  { return producedCount.get(); }
    public long getRejectedCount()  { return rejectedCount.get(); }
    public long getErrorCount()     { return errorCount.get(); }
    public String getName()         { return name; }
}