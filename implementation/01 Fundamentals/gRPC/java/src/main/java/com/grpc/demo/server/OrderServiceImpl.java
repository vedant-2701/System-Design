package com.example.grpc.server;

import com.example.grpc.gen.*;
import com.google.protobuf.Timestamp;
import io.grpc.Status;
import io.grpc.stub.StreamObserver;

import java.time.Instant;
import java.util.Map;
import java.util.concurrent.*;
import java.util.concurrent.atomic.AtomicLong;
import java.util.logging.Logger;

/**
 * Java implementation of OrderService.
 *
 * Key differences from the Go implementation:
 *
 * 1. STREAMING MODEL: Go uses imperative loops (stream.Send() in a for loop).
 *    Java uses the StreamObserver CALLBACK pattern.
 *
 * 2. CONCURRENCY MODEL: Go uses goroutines. Java 21 uses virtual threads
 *    (Project Loom) — configured in ServerMain's executor.
 *
 * 3. BACKPRESSURE / FAN-OUT (corrected):
 *    Go's buffered channel + non-blocking select/default is reproduced here as:
 *      - a bounded ArrayBlockingQueue per subscriber (explicit buffer, capacity 32)
 *      - fanOut() uses queue.offer() — non-blocking, returns false if full → drop + log
 *      - each subscriber has a DEDICATED virtual thread that drains its queue
 *        and calls observer.onNext(). A slow onNext() (HTTP/2 backpressure from
 *        a slow client) blocks ONLY that subscriber's consumer thread — never
 *        fanOut() itself, and never CreateOrder.
 *
 *    Without this, naive onNext() calls rely on Netty's implicit, unbounded
 *    outbound buffer — unbounded memory growth under slow consumers, and no
 *    isolation between subscribers during fan-out.
 */
public class OrderServiceImpl extends OrderServiceGrpc.OrderServiceImplBase {

    private static final Logger log = Logger.getLogger(OrderServiceImpl.class.getName());

    private static final int SUBSCRIBER_QUEUE_CAPACITY = 32; // same as Go's buffer size

    // ── State ─────────────────────────────────────────────────────────────────

    private final Map<Long, Order> orders = new ConcurrentHashMap<>();
    private final AtomicLong nextOrderId = new AtomicLong(1);

    /**
     * Per-subscriber bounded queue — the Java equivalent of Go's
     *   ch := make(chan *pb.OrderEvent, 32)
     */
    private final Map<Long, BlockingQueue<OrderEvent>> subscriberQueues = new ConcurrentHashMap<>();

    /** Each subscriber's StreamObserver — only ever touched by its own consumer thread. */
    private final Map<Long, StreamObserver<OrderEvent>> subscribers = new ConcurrentHashMap<>();

    /** Consumer threads per subscriber, so we can interrupt them on disconnect/shutdown. */
    private final Map<Long, Thread> consumerThreads = new ConcurrentHashMap<>();

    private final AtomicLong nextSubId = new AtomicLong(0);

    // Metric: count of dropped events due to full subscriber queues.
    // In production this would be a Prometheus counter, exposed for alerting.
    private final AtomicLong droppedEvents = new AtomicLong(0);

    private final ScheduledExecutorService heartbeatScheduler =
            Executors.newSingleThreadScheduledExecutor(
                    Thread.ofVirtual().name("heartbeat").factory()
            );

    public OrderServiceImpl() {
        orders.put(0L, Order.newBuilder()
                .setOrderId(0)
                .addItems(Item.newBuilder()
                        .setName("Laptop")
                        .setQuantity(1)
                        .setPriceCents(99900)
                        .build())
                .setStatus(OrderStatus.ORDER_STATUS_CONFIRMED)
                .setCreatedAt(nowTimestamp())
                .setTotalCents(99900)
                .build());

        heartbeatScheduler.scheduleAtFixedRate(
                this::sendHeartbeats, 10, 10, TimeUnit.SECONDS
        );
    }

    // ── Unary: GetOrder ───────────────────────────────────────────────────────

    @Override
    public void getOrder(GetOrderRequest request, StreamObserver<Order> responseObserver) {
        long orderId = request.getOrderId();

        Order order = orders.get(orderId);
        if (order == null) {
            responseObserver.onError(
                    Status.NOT_FOUND
                            .withDescription("order " + orderId + " not found")
                            .asRuntimeException()
            );
            return;
        }

        responseObserver.onNext(order);
        responseObserver.onCompleted();
    }

    // ── Unary: CreateOrder ────────────────────────────────────────────────────

    @Override
    public void createOrder(CreateOrderRequest request,
                            StreamObserver<CreateOrderResponse> responseObserver) {
        if (request.getItemsList().isEmpty()) {
            responseObserver.onError(
                    Status.INVALID_ARGUMENT
                            .withDescription("at least one item is required")
                            .asRuntimeException()
            );
            return;
        }

        long total = 0;
        for (Item item : request.getItemsList()) {
            if (item.getQuantity() == 0) {
                responseObserver.onError(
                        Status.INVALID_ARGUMENT
                                .withDescription("item '" + item.getName() + "' has zero quantity")
                                .asRuntimeException()
                );
                return;
            }
            total += (long) item.getPriceCents() * item.getQuantity();
        }

        Timestamp now = nowTimestamp();
        long id = nextOrderId.getAndIncrement();

        Order order = Order.newBuilder()
                .setOrderId((int) id)
                .addAllItems(request.getItemsList())
                .setStatus(OrderStatus.ORDER_STATUS_PENDING)
                .setCreatedAt(now)
                .setTotalCents((int) total)
                .build();

        orders.put(id, order);
        log.fine("Order created: id=" + id + " total_cents=" + total);

        OrderEvent event = OrderEvent.newBuilder()
                .setOrderId((int) id)
                .setOldStatus(OrderStatus.ORDER_STATUS_UNKNOWN)
                .setNewStatus(OrderStatus.ORDER_STATUS_PENDING)
                .setOccurredAt(now)
                .build();
        fanOut(event);

        responseObserver.onNext(CreateOrderResponse.newBuilder()
                .setOrderId((int) id)
                .setTotalCents((int) total)
                .setCreatedAt(now)
                .build());
        responseObserver.onCompleted();
    }

    // ── Server Streaming: WatchOrders ─────────────────────────────────────────

    /**
     * Registers a bounded queue + a dedicated consumer virtual thread for this
     * subscriber. fanOut() only ever does a non-blocking queue.offer() — it
     * NEVER calls onNext() directly. The consumer thread is the only thread
     * that calls onNext(), so we don't need `synchronized` for thread-safety
     * (StreamObserver methods must not be called concurrently — single-writer
     * per observer is now structurally guaranteed).
     */
    @Override
    public void watchOrders(WatchOrdersRequest request,
                            StreamObserver<OrderEvent> responseObserver) {
        long subId = nextSubId.getAndIncrement();
        log.info("Watcher connected: sub_id=" + subId);

        BlockingQueue<OrderEvent> queue = new ArrayBlockingQueue<>(SUBSCRIBER_QUEUE_CAPACITY);
        subscriberQueues.put(subId, queue);
        subscribers.put(subId, responseObserver);

        // Dedicated consumer: drains the queue, calls onNext().
        // A slow onNext() (client-side backpressure) blocks ONLY this thread.
        Thread consumer = Thread.ofVirtual().name("subscriber-" + subId).start(() -> {
            try {
                while (!Thread.currentThread().isInterrupted()) {
                    OrderEvent event = queue.take(); // blocks this virtual thread only
                    responseObserver.onNext(event);
                }
            } catch (InterruptedException e) {
                // Normal shutdown path
            } catch (Exception e) {
                log.warning("Consumer for sub_id=" + subId + " failed: " + e.getMessage());
            }
        });
        consumerThreads.put(subId, consumer);

        // Cleanup on client disconnect
        if (responseObserver instanceof io.grpc.stub.ServerCallStreamObserver<OrderEvent> serverObserver) {
            serverObserver.setOnCancelHandler(() -> {
                log.info("Watcher cancelled by client: sub_id=" + subId);
                removeSubscriber(subId);
            });
        }
    }

    // ── Fan-out ───────────────────────────────────────────────────────────────

    /**
     * Non-blocking offer to each subscriber's queue — equivalent to Go's:
     *   select {
     *   case ch <- event:
     *   default: // drop
     *   }
     *
     * This method NEVER calls onNext() and NEVER blocks. A slow subscriber's
     * full queue only affects that subscriber (dropped event + logged metric).
     */
    private void fanOut(OrderEvent event) {
        subscriberQueues.forEach((subId, queue) -> {
            if (!queue.offer(event)) {
                droppedEvents.incrementAndGet();
                log.warning("subscriber buffer full, dropping event: sub_id=" + subId);
            }
        });
    }

    private void sendHeartbeats() {
        if (subscriberQueues.isEmpty()) return;

        OrderEvent heartbeat = OrderEvent.newBuilder()
                .setOrderId(0)
                .setOccurredAt(nowTimestamp())
                .build();
        fanOut(heartbeat);
    }

    private void removeSubscriber(long subId) {
        subscriberQueues.remove(subId);
        subscribers.remove(subId);
        Thread consumer = consumerThreads.remove(subId);
        if (consumer != null) {
            consumer.interrupt();
        }
    }

    /** Exposed for tests / metrics endpoints. */
    public long getDroppedEventCount() {
        return droppedEvents.get();
    }

    /** Exposed for tests. */
    public int getSubscriberCount() {
        return subscriberQueues.size();
    }

    // ── Lifecycle ─────────────────────────────────────────────────────────────

    public void shutdown() {
        heartbeatScheduler.shutdownNow();

        subscribers.forEach((subId, observer) -> {
            try {
                observer.onCompleted();
            } catch (Exception e) {
                // Already closed — ignore
            }
        });
        consumerThreads.values().forEach(Thread::interrupt);

        subscribers.clear();
        subscriberQueues.clear();
        consumerThreads.clear();
    }

    // ── Helpers ───────────────────────────────────────────────────────────────

    private static Timestamp nowTimestamp() {
        Instant now = Instant.now();
        return Timestamp.newBuilder()
                .setSeconds(now.getEpochSecond())
                .setNanos(now.getNano())
                .build();
    }
}