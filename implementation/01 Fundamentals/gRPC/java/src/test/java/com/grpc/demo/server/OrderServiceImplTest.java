package com.example.grpc.server;

import com.example.grpc.gen.*;
import io.grpc.ManagedChannel;
import io.grpc.Status;
import io.grpc.StatusRuntimeException;
import io.grpc.inprocess.InProcessChannelBuilder;
import io.grpc.inprocess.InProcessServerBuilder;
import io.grpc.stub.StreamObserver;
import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.Timeout;

import java.time.Duration;
import java.util.ArrayList;
import java.util.HashSet;
import java.util.List;
import java.util.concurrent.*;
import java.util.concurrent.atomic.AtomicReference;

import static org.junit.jupiter.api.Assertions.*;

/**
 * Unit and concurrency tests for OrderServiceImpl, using a REAL gRPC server
 * and channel via InProcessServerBuilder / InProcessChannelBuilder.
 *
 * Why in-process rather than calling service methods directly (as the Go
 * tests call s.GetOrder() / s.CreateOrder() directly):
 *
 * grpc-java's own documentation explicitly warns: "DO NOT MOCK" StreamObserver
 * or ServerCallStreamObserver — the API is too complex to reliably hand-roll.
 * InProcessServerBuilder gives us the REAL gRPC machinery (interceptors,
 * StreamObserver wiring, cancellation propagation, flow control) without a
 * real network socket — fast, deterministic, and correct.
 *
 * We deliberately do NOT use directExecutor() (common in grpc-java examples
 * for simple tests). directExecutor() runs all callbacks synchronously on the
 * calling thread — which would serialize our concurrency tests and hide the
 * exact backpressure/threading bugs we're testing for. We use the real
 * virtual-thread executor instead, matching ServerMain's production config.
 */
class OrderServiceImplTest {

    private io.grpc.Server server;
    private ManagedChannel channel;
    private OrderServiceGrpc.OrderServiceBlockingStub blockingStub;
    private OrderServiceGrpc.OrderServiceStub asyncStub;
    private OrderServiceImpl service;
    private ExecutorService serverExecutor;
    private ExecutorService channelExecutor;

    @BeforeEach
    void setUp() throws Exception {
        String serverName = InProcessServerBuilder.generateName();

        service = new OrderServiceImpl();

        // Real virtual-thread-per-task executors on both sides — matches
        // ServerMain's production configuration. NOT directExecutor(), which
        // would serialize callbacks and mask the threading behavior we test.
        serverExecutor = Executors.newVirtualThreadPerTaskExecutor();
        channelExecutor = Executors.newVirtualThreadPerTaskExecutor();

        server = InProcessServerBuilder
                .forName(serverName)
                .executor(serverExecutor)
                .addService(service)
                .build()
                .start();

        channel = InProcessChannelBuilder
                .forName(serverName)
                .executor(channelExecutor)
                .build();

        blockingStub = OrderServiceGrpc.newBlockingStub(channel);
        asyncStub = OrderServiceGrpc.newStub(channel);
    }

    @AfterEach
    void tearDown() throws InterruptedException {
        channel.shutdownNow();
        server.shutdownNow();
        server.awaitTermination(5, TimeUnit.SECONDS);
        service.shutdown();
        serverExecutor.shutdownNow();
        channelExecutor.shutdownNow();
    }

    // ── Unit Tests: GetOrder ──────────────────────────────────────────────────

    @Test
    void getOrder_success() {
        Order order = blockingStub.getOrder(GetOrderRequest.newBuilder().setOrderId(0).build());

        assertEquals(0, order.getOrderId());
        assertEquals(OrderStatus.ORDER_STATUS_CONFIRMED, order.getStatus());
    }

    @Test
    void getOrder_notFound() {
        // Verify the EXACT status code — a test that only checks "throws"
        // would pass even if the server returned INTERNAL, which is a real
        // production bug (client can't distinguish "doesn't exist" from
        // "server is broken").
        StatusRuntimeException ex = assertThrows(StatusRuntimeException.class,
                () -> blockingStub.getOrder(GetOrderRequest.newBuilder().setOrderId(9999).build()));

        assertEquals(Status.Code.NOT_FOUND, ex.getStatus().getCode());
    }

    // ── Unit Tests: CreateOrder ───────────────────────────────────────────────

    @Test
    void createOrder_success() {
        CreateOrderResponse resp = blockingStub.createOrder(CreateOrderRequest.newBuilder()
                .addItems(Item.newBuilder().setName("Widget").setQuantity(3).setPriceCents(1000).build())
                .build());

        assertEquals(3000, resp.getTotalCents());

        // Verify the order is actually retrievable — tests the write landed,
        // not just that the unary response looked right.
        Order order = blockingStub.getOrder(GetOrderRequest.newBuilder().setOrderId(resp.getOrderId()).build());
        assertEquals(OrderStatus.ORDER_STATUS_PENDING, order.getStatus());
    }

    @Test
    void createOrder_emptyItems_returnsInvalidArgument() {
        StatusRuntimeException ex = assertThrows(StatusRuntimeException.class,
                () -> blockingStub.createOrder(CreateOrderRequest.newBuilder().build()));

        assertEquals(Status.Code.INVALID_ARGUMENT, ex.getStatus().getCode());
    }

    @Test
    void createOrder_zeroQuantity_returnsInvalidArgument() {
        StatusRuntimeException ex = assertThrows(StatusRuntimeException.class,
                () -> blockingStub.createOrder(CreateOrderRequest.newBuilder()
                        .addItems(Item.newBuilder().setName("Bad").setQuantity(0).setPriceCents(100).build())
                        .build()));

        assertEquals(Status.Code.INVALID_ARGUMENT, ex.getStatus().getCode());
    }

    // ── Concurrency Tests: CreateOrder under contention ───────────────────────

    /**
     * Verifies concurrent CreateOrder calls never produce duplicate order IDs.
     * Java equivalent of TestCreateOrder_ConcurrentIDsAreUnique.
     *
     * Each call goes through the REAL gRPC stack (in-process transport,
     * virtual-thread executor on both client and server sides) — this
     * exercises the same AtomicLong-based ID generator under genuine
     * concurrent RPC dispatch, not just concurrent Java method calls.
     */
    @Test
    @Timeout(15)
    void createOrder_concurrentIdsAreUnique() throws InterruptedException {
        final int taskCount = 100;
        var ids = new ConcurrentLinkedQueue<Integer>();
        var errors = new ConcurrentLinkedQueue<Throwable>();

        try (var executor = Executors.newVirtualThreadPerTaskExecutor()) {
            var latch = new CountDownLatch(taskCount);

            for (int i = 0; i < taskCount; i++) {
                executor.submit(() -> {
                    try {
                        CreateOrderResponse resp = blockingStub.createOrder(CreateOrderRequest.newBuilder()
                                .addItems(Item.newBuilder().setName("Item").setQuantity(1).setPriceCents(100).build())
                                .build());
                        ids.add(resp.getOrderId());
                    } catch (Throwable t) {
                        errors.add(t);
                    } finally {
                        latch.countDown();
                    }
                });
            }

            assertTrue(latch.await(10, TimeUnit.SECONDS), "tasks did not complete in time");
        }

        assertTrue(errors.isEmpty(), "errors occurred: " + errors);
        assertEquals(taskCount, new HashSet<>(ids).size(), "duplicate order IDs generated");
    }

    // ── Concurrency Tests: WatchOrders fan-out ────────────────────────────────

    /**
     * Verifies that a single CreateOrder delivers an event to ALL active
     * subscribers — real streaming over the in-process transport.
     *
     * Java equivalent of TestWatchOrders_FanOutToMultipleSubscribers.
     */
    @Test
    @Timeout(15)
    void watchOrders_fanOutToMultipleSubscribers() throws InterruptedException {
        final int numSubscribers = 5;
        List<BlockingQueue<OrderEvent>> received = new ArrayList<>();

        for (int i = 0; i < numSubscribers; i++) {
            BlockingQueue<OrderEvent> queue = new LinkedBlockingQueue<>();
            received.add(queue);

            asyncStub.watchOrders(WatchOrdersRequest.newBuilder().build(), new StreamObserver<>() {
                @Override
                public void onNext(OrderEvent value) {
                    queue.offer(value);
                }

                @Override
                public void onError(Throwable t) {}

                @Override
                public void onCompleted() {}
            });
        }

        waitForSubscriberCount(numSubscribers, Duration.ofSeconds(5));

        blockingStub.createOrder(CreateOrderRequest.newBuilder()
                .addItems(Item.newBuilder().setName("Broadcast Test").setQuantity(1).setPriceCents(500).build())
                .build());

        for (int i = 0; i < numSubscribers; i++) {
            OrderEvent event = received.get(i).poll(5, TimeUnit.SECONDS);
            assertNotNull(event, "subscriber " + i + " did not receive event");
            assertEquals(OrderStatus.ORDER_STATUS_PENDING, event.getNewStatus());
        }
    }

    /**
     * The test that validates the corrected backpressure design: a subscriber
     * whose onNext() blocks indefinitely must NOT block CreateOrder.
     *
     * Java equivalent of TestWatchOrders_SlowSubscriberDoesNotBlockCreateOrder.
     *
     * Without the per-subscriber BlockingQueue + dedicated consumer virtual
     * thread fix, this test would time out — the consumer thread would block
     * on observer.onNext() (which here blocks on our CountDownLatch.await()),
     * but critically that block must stay confined to the consumer thread and
     * never propagate to fanOut() or CreateOrder().
     */
    @Test
    @Timeout(20)
    void watchOrders_slowSubscriberDoesNotBlockCreateOrder() throws InterruptedException {
        // A subscriber whose onNext() blocks FOREVER — simulates a stuck client
        // that stopped reading from its stream.
        CountDownLatch neverCountedDown = new CountDownLatch(1);

        asyncStub.watchOrders(WatchOrdersRequest.newBuilder().build(), new StreamObserver<>() {
            @Override
            public void onNext(OrderEvent value) {
                try {
                    neverCountedDown.await(); // blocks the consumer thread forever
                } catch (InterruptedException e) {
                    Thread.currentThread().interrupt();
                }
            }

            @Override
            public void onError(Throwable t) {}

            @Override
            public void onCompleted() {}
        });

        waitForSubscriberCount(1, Duration.ofSeconds(5));

        final int numOrders = 40; // > per-subscriber queue capacity (32)

        for (int i = 0; i < numOrders; i++) {
            final int idx = i;
            var future = CompletableFuture.runAsync(() ->
                    blockingStub.createOrder(CreateOrderRequest.newBuilder()
                            .addItems(Item.newBuilder().setName("Flood").setQuantity(1).setPriceCents(100).build())
                            .build())
            );

            try {
                future.get(1, TimeUnit.SECONDS);
            } catch (TimeoutException | ExecutionException e) {
                fail("CreateOrder " + idx + " blocked or failed — slow subscriber backpressure leaked into writer path: " + e);
            }
        }

        assertTrue(service.getDroppedEventCount() > 0,
                "expected dropped events once the 32-capacity queue filled — fan-out non-blocking path not exercised");
    }

    /**
     * Verifies cleanup on client cancellation — Java equivalent of
     * TestWatchOrders_ContextCancellationCleansUpSubscriber.
     *
     * Real cancellation: we cancel the client-side call via a CancellableContext,
     * which propagates a CANCELLED status to the server, triggering
     * ServerCallStreamObserver's onCancelHandler — exactly the production path.
     */
    @Test
    @Timeout(15)
    void watchOrders_cancellationCleansUpSubscriber() throws InterruptedException {
        var receivedFirst = new CountDownLatch(1);

        io.grpc.Context.CancellableContext withCancel = io.grpc.Context.current().withCancellation();
        withCancel.run(() -> asyncStub.watchOrders(WatchOrdersRequest.newBuilder().build(), new StreamObserver<>() {
            @Override
            public void onNext(OrderEvent value) {
                receivedFirst.countDown();
            }

            @Override
            public void onError(Throwable t) {}

            @Override
            public void onCompleted() {}
        }));

        waitForSubscriberCount(1, Duration.ofSeconds(5));

        withCancel.cancel(new RuntimeException("client disconnect"));

        waitForSubscriberCount(0, Duration.ofSeconds(5));
    }

    // ── Helpers ───────────────────────────────────────────────────────────────

    private void waitForSubscriberCount(int want, Duration timeout) throws InterruptedException {
        long deadline = System.nanoTime() + timeout.toNanos();
        while (System.nanoTime() < deadline) {
            if (service.getSubscriberCount() == want) return;
            Thread.sleep(10);
        }
        fail("timed out waiting for subscriber count " + want + ", got " + service.getSubscriberCount());
    }
}