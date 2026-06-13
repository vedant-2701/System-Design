// ProducerConsumerSystem.java

import java.util.ArrayList;
import java.util.List;
import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;
import java.util.concurrent.TimeUnit;
import java.util.function.Consumer;
import java.util.function.Supplier;
import java.util.logging.Logger;

/**
 * Orchestrates producers, consumers, and the shared buffer lifecycle.
 *
 * Shutdown sequence:
 *   1. Mark buffer as shutdown — producers stop accepting new items immediately.
 *   2. Insert N poison pills — one per consumer — to unblock waiting consumers.
 *   3. Await consumer thread termination — consumers drain remaining items then exit.
 *   4. Shut down executor — all threads are done.
 *
 * This guarantees:
 *   - No items are produced after shutdown is called.
 *   - All items already in the buffer are consumed before threads exit.
 *   - No consumer waits forever on an empty buffer after shutdown.
 */
public class ProducerConsumerSystem<T> {

    private static final Logger log = Logger.getLogger(ProducerConsumerSystem.class.getName());

    private final BoundedBuffer<T> buffer;
    private final ExecutorService executor;
    private final List<Producer<T>> producers = new ArrayList<>();
    private final List<ConsumerWorker<T>> consumers = new ArrayList<>();
    private final int consumerCount;

    public ProducerConsumerSystem(int bufferCapacity, int producerCount, int consumerCount) {
        this.buffer = new BoundedBuffer<>(bufferCapacity);
        this.consumerCount = consumerCount;
        // Total threads = producers + consumers
        this.executor = Executors.newFixedThreadPool(producerCount + consumerCount);
    }

    public ProducerConsumerSystem<T> addProducer(String name, Supplier<T> supplier,
                                                  long intervalMs) {
        Producer<T> producer = new Producer<>(name, buffer, supplier, intervalMs);
        producers.add(producer);
        return this;
    }

    public ProducerConsumerSystem<T> addConsumer(String name, Consumer<T> handler) {
        ConsumerWorker<T> consumer = new ConsumerWorker<>(name, buffer, handler);
        consumers.add(consumer);
        return this;
    }

    /** Starts all producers and consumers. */
    public void start() {
        log.info("Starting system: "
                + producers.size() + " producers, "
                + consumers.size() + " consumers, "
                + "buffer capacity=" + buffer.capacity());

        // Start consumers first — they should be ready before producers emit items
        consumers.forEach(executor::submit);
        producers.forEach(executor::submit);
    }

    /**
     * Graceful shutdown.
     *
     * @param timeoutSeconds maximum time to wait for consumers to drain the buffer
     */
    public void shutdown(long timeoutSeconds) {
        log.info("Initiating graceful shutdown...");

        // Step 1: stop producers — buffer.isShutdown() will return true,
        // producers exit their loop on next iteration.
        // Step 2: insert poison pills — unblocks any consumer waiting on empty buffer.
        buffer.shutdown(consumerCount);

        // Step 3: stop accepting new tasks and wait for in-flight work to complete
        executor.shutdown();
        try {
            if (!executor.awaitTermination(timeoutSeconds, TimeUnit.SECONDS)) {
                log.warning("Timeout waiting for shutdown — forcing termination");
                executor.shutdownNow();
            }
        } catch (InterruptedException e) {
            Thread.currentThread().interrupt();
            executor.shutdownNow();
        }

        logStats();
    }

    private void logStats() {
        long totalProduced = producers.stream().mapToLong(Producer::getProducedCount).sum();
        long totalRejected = producers.stream().mapToLong(Producer::getRejectedCount).sum();
        long totalConsumed = consumers.stream().mapToLong(ConsumerWorker::getConsumedCount).sum();

        log.info("=== Shutdown complete ===");
        log.info("Total produced : " + totalProduced);
        log.info("Total rejected : " + totalRejected);
        log.info("Total consumed : " + totalConsumed);
        log.info("Buffer remaining: " + buffer.size());
    }

    public BoundedBuffer<T> getBuffer() { return buffer; }
}