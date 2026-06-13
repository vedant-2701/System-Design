// RetryingProducer.java

import java.util.concurrent.TimeUnit;
import java.util.logging.Logger;

/**
 * Demonstrates production-grade producer pattern with exponential backoff.
 *
 * Never call put() blindly in production — if the consumer side dies,
 * the producer blocks forever holding a thread and its stack memory.
 *
 * Instead: use offer() with timeout + exponential backoff + max retries.
 * After max retries, the caller gets a clear failure signal and can:
 *   - drop the item and increment a dropped-items metric
 *   - write to a dead-letter store
 *   - propagate backpressure to the upstream caller (e.g. return HTTP 429)
 */
public class RetryingProducer<T> {

    private static final Logger log = Logger.getLogger(RetryingProducer.class.getName());

    private final BoundedQueue<T> queue;
    private final int maxRetries;
    private final long initialDelayMs;
    private final long maxDelayMs;

    public RetryingProducer(BoundedQueue<T> queue, int maxRetries,
                            long initialDelayMs, long maxDelayMs) {
        this.queue = queue;
        this.maxRetries = maxRetries;
        this.initialDelayMs = initialDelayMs;
        this.maxDelayMs = maxDelayMs;
    }

    /**
     * Attempts to enqueue an item with exponential backoff on failure.
     *
     * @return true if enqueued successfully, false after all retries exhausted
     * @throws InterruptedException if interrupted during backoff sleep
     */
    public boolean produce(T item) throws InterruptedException {
        long delayMs = initialDelayMs;

        for (int attempt = 1; attempt <= maxRetries; attempt++) {
            boolean accepted = queue.offer(item, delayMs, TimeUnit.MILLISECONDS);
            if (accepted) {
                if (attempt > 1) {
                    log.fine(() -> "Item accepted on attempt " + attempt);
                }
                return true;
            }

            if (attempt == maxRetries) {
                log.warning(() -> String.format(
                        "Failed to enqueue after %d attempts — queue full. " +
                        "Consider: scaling consumers, increasing capacity, or shedding load.",
                        maxRetries));
                return false;
            }

            log.fine(() -> String.format("Queue full on attempt %d — backing off %dms", attempt, delayMs));
            Thread.sleep(delayMs);

            // Exponential backoff capped at maxDelayMs
            delayMs = Math.min(delayMs * 2, maxDelayMs);
        }

        return false; // unreachable but compiler requires it
    }
}