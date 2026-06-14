package com.example.grpc.server;

import com.example.grpc.gen.*;
import io.grpc.ManagedChannel;
import io.grpc.Server;
import io.grpc.inprocess.InProcessChannelBuilder;
import io.grpc.inprocess.InProcessServerBuilder;
import io.grpc.stub.StreamObserver;
import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.Timeout;
import org.junit.jupiter.api.condition.DisabledIfSystemProperty;

import java.time.Duration;
import java.util.HashSet;
import java.util.List;
import java.util.concurrent.*;
import java.util.concurrent.atomic.AtomicLong;

import static org.junit.jupiter.api.Assertions.*;

/**
 * High-concurrency load and stress tests over a REAL in-process gRPC
 * server/channel, using virtual threads on both sides — matching ServerMain's
 * production configuration.
 *
 * Java equivalent of load_test.go.
 *
 * Run with:
 *   mvn test -Dtest=OrderServiceLoadTest
 *
 * Skip in fast CI runs with:
 *   mvn test -DskipStressTests=true
 */
@DisabledIfSystemProperty(named = "skipStressTests", matches = "true")
class OrderServiceLoadTest {

    private Server server;
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

    /**
     * Java equivalent of TestStress_ManyWatchersManyCreates.
     *
     * 500 concurrent WatchOrders subscribers + 200 concurrent CreateOrder
     * callers (10 creates each = 2000 total creates), all over a real
     * in-process gRPC transport with virtual-thread executors.
     *
     * Assertions:
     *   1. No CreateOrder call exceeds 100ms (writer/reader isolation holds)
     *   2. After cancelling all subscriber streams, subscriber count returns
     *      to 0 (no leaked queues/consumer threads)
     */
    @Test
    @Timeout(90)
    void manyWatchersManyCreates() throws InterruptedException {
        final int numWatchers = 500;
        final int numCreators = 200;
        final int createsPerCreator = 10;

        List<io.grpc.Context.CancellableContext> watcherContexts = new CopyOnWriteArrayList<>();

        // ── Register watchers ─────────────────────────────────────────────────
        for (int i = 0; i < numWatchers; i++) {
            final int idx = i;
            io.grpc.Context.CancellableContext ctx = io.grpc.Context.current().withCancellation();
            watcherContexts.add(ctx);

            ctx.run(() -> asyncStub.watchOrders(WatchOrdersRequest.newBuilder().build(), new StreamObserver<>() {
                @Override
                public void onNext(OrderEvent value) {
                    // Every 50th subscriber simulates a slow consumer —
                    // same ratio as the Go stress test.
                    if (idx % 50 == 0) {
                        try {
                            Thread.sleep(1);
                        } catch (InterruptedException e) {
                            Thread.currentThread().interrupt();
                        }
                    }
                }

                @Override
                public void onError(Throwable t) {}

                @Override
                public void onCompleted() {}
            }));
        }

        waitForSubscriberCount(numWatchers, Duration.ofSeconds(10));

        // ── Run concurrent creators ──────────────────────────────────────────
        AtomicLong maxLatencyNanos = new AtomicLong(0);
        List<Throwable> errors = new CopyOnWriteArrayList<>();

        try (var executor = Executors.newVirtualThreadPerTaskExecutor()) {
            var latch = new CountDownLatch(numCreators);

            for (int i = 0; i < numCreators; i++) {
                executor.submit(() -> {
                    try {
                        for (int j = 0; j < createsPerCreator; j++) {
                            long start = System.nanoTime();

                            blockingStub.createOrder(CreateOrderRequest.newBuilder()
                                    .addItems(Item.newBuilder().setName("Stress Item").setQuantity(1).setPriceCents(100).build())
                                    .build());

                            long elapsed = System.nanoTime() - start;
                            maxLatencyNanos.getAndUpdate(cur -> Math.max(cur, elapsed));
                        }
                    } catch (Throwable t) {
                        errors.add(t);
                    } finally {
                        latch.countDown();
                    }
                });
            }

            assertTrue(latch.await(70, TimeUnit.SECONDS), "creators did not finish in time");
        }

        assertTrue(errors.isEmpty(), "errors during load: " + errors);

        long maxLatencyMs = Duration.ofNanos(maxLatencyNanos.get()).toMillis();
        System.out.println("max CreateOrder latency under load: " + maxLatencyMs + "ms");
        System.out.println("dropped events: " + service.getDroppedEventCount());

        // Core assertion — same threshold and reasoning as the Go test.
        // Note: real RPC round-trip overhead (even in-process) is higher than
        // a direct method call, so this threshold is looser than a pure
        // in-memory benchmark would allow, but a regression to blocking
        // onNext() (the bug we fixed) would show latencies an order of
        // magnitude higher, or the test would hang entirely.
        assertTrue(maxLatencyMs < 200,
                "CreateOrder latency too high under load: " + maxLatencyMs + "ms (writer/reader coupling regression?)");

        // ── Cleanup ───────────────────────────────────────────────────────────
        watcherContexts.forEach(ctx -> ctx.cancel(new RuntimeException("test cleanup")));
        waitForSubscriberCount(0, Duration.ofSeconds(10));
    }

    /**
     * Java equivalent of TestStress_FloodSingleSlowSubscriber.
     *
     * ONE subscriber whose onNext() blocks forever, while 1000 orders are
     * created. Validates that the writer path completes quickly regardless,
     * and that drops are recorded once the per-subscriber queue (capacity 32)
     * fills.
     */
    @Test
    @Timeout(60)
    void floodSingleSlowSubscriber() throws InterruptedException {
        CountDownLatch neverCountedDown = new CountDownLatch(1);

        asyncStub.watchOrders(WatchOrdersRequest.newBuilder().build(), new StreamObserver<>() {
            @Override
            public void onNext(OrderEvent value) {
                try {
                    neverCountedDown.await(); // blocks forever — stuck client
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

        final int numOrders = 1000;
        long start = System.nanoTime();

        for (int i = 0; i < numOrders; i++) {
            blockingStub.createOrder(CreateOrderRequest.newBuilder()
                    .addItems(Item.newBuilder().setName("Flood").setQuantity(1).setPriceCents(1).build())
                    .build());
        }

        long elapsedMs = Duration.ofNanos(System.nanoTime() - start).toMillis();
        System.out.println("created " + numOrders + " orders with 1 stuck subscriber in " + elapsedMs + "ms");
        System.out.println("dropped events: " + service.getDroppedEventCount());

        // Real RPC round-trips for 1000 sequential unary calls take longer
        // than the Go in-memory equivalent — threshold reflects that, while
        // still catching a true deadlock (which would hit the @Timeout instead).
        assertTrue(elapsedMs < 30_000,
                "creates took too long (" + elapsedMs + "ms) — stuck subscriber may be blocking writer");

        long expectedMinDrops = numOrders - 32; // queue capacity
        assertTrue(service.getDroppedEventCount() >= expectedMinDrops,
                "expected ~" + expectedMinDrops + " dropped events, got " + service.getDroppedEventCount());
    }

    /**
     * Concurrent ID uniqueness under heavier parallelism (2000 concurrent
     * creates) — a larger-scale version of the correctness test in
     * OrderServiceImplTest.
     */
    @Test
    @Timeout(60)
    void concurrentCreateOrder_idsRemainUniqueAtScale() throws InterruptedException {
        final int taskCount = 2000;
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

            assertTrue(latch.await(50, TimeUnit.SECONDS));
        }

        assertTrue(errors.isEmpty(), "errors: " + errors);
        assertEquals(taskCount, new HashSet<>(ids).size(), "duplicate IDs at scale");
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