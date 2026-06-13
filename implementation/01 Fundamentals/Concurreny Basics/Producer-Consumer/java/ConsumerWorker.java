// ConsumerWorker.java

import java.util.concurrent.atomic.AtomicLong;
import java.util.function.Consumer;
import java.util.logging.Logger;

/**
 * A consumer that pulls items from a BoundedBuffer and processes them.
 *
 * Design decisions:
 * - Consumer<T> handler decouples processing logic from queue mechanics.
 * - null return from buffer.take() is the poison pill signal — consumer exits cleanly.
 * - Handler errors are caught per-item — one bad item does not kill the consumer thread.
 *   In production, failed items would go to a dead-letter queue rather than being dropped.
 * - consumedCount and errorCount provide observability into processing health.
 */
public class ConsumerWorker<T> implements Runnable {

    private static final Logger log = Logger.getLogger(ConsumerWorker.class.getName());

    private final String name;
    private final BoundedBuffer<T> buffer;
    private final Consumer<T> handler;

    private final AtomicLong consumedCount = new AtomicLong(0);
    private final AtomicLong errorCount    = new AtomicLong(0);

    /**
     * @param name    human-readable name for logging
     * @param buffer  source bounded buffer
     * @param handler processes each dequeued item
     */
    public ConsumerWorker(String name, BoundedBuffer<T> buffer, Consumer<T> handler) {
        this.name = name;
        this.buffer = buffer;
        this.handler = handler;
    }

    @Override
    public void run() {
        log.info(name + " started");

        try {
            while (true) {
                // take() blocks until an item is available.
                // Returns null on poison pill — signals clean shutdown.
                T item = buffer.take();

                if (item == null) {
                    log.info(name + " received shutdown signal (poison pill) — stopping");
                    break;
                }

                try {
                    handler.accept(item);
                    consumedCount.incrementAndGet();
                    log.fine(name + " consumed: " + item);
                } catch (Exception e) {
                    // Handler error — log, increment error counter, continue.
                    // Production systems would route to a dead-letter queue here.
                    errorCount.incrementAndGet();
                    log.warning(name + " handler error for item [" + item + "]: " + e.getMessage());
                }
            }
        } catch (InterruptedException e) {
            Thread.currentThread().interrupt();
            log.info(name + " interrupted — stopping");
        }

        log.info(name + " stopped. consumed=" + consumedCount + " errors=" + errorCount);
    }

    // ---- Observability ----

    public long getConsumedCount() { return consumedCount.get(); }
    public long getErrorCount()    { return errorCount.get(); }
    public String getName()        { return name; }
}