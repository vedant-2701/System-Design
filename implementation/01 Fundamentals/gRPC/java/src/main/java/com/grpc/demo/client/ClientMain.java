package com.example.grpc.client;

import com.example.grpc.gen.*;
import io.grpc.*;
import io.grpc.netty.shaded.io.grpc.netty.NettyChannelBuilder;
import io.grpc.stub.StreamObserver;

import java.util.List;
import java.util.concurrent.CountDownLatch;
import java.util.concurrent.TimeUnit;
import java.util.logging.Logger;

/**
 * Client demonstrating all three RPC patterns.
 *
 * KEY DIFFERENCE FROM GO CLIENT — STREAMING:
 *
 * Go: stream.Recv() is called in an explicit for-loop. You pull events.
 * Java: you provide a StreamObserver with onNext/onError/onCompleted callbacks.
 *       The framework pushes events to you. This is the async stub pattern.
 *
 * Java also offers a BLOCKING stub (OrderServiceGrpc.newBlockingStub) where
 * server-streaming calls return an Iterator<OrderEvent> — closer to Go's pull
 * model. We use the async stub here because it's the more common production
 * pattern and demonstrates the callback model explicitly.
 */
public class ClientMain {

    private static final Logger log = Logger.getLogger(ClientMain.class.getName());

    // Metadata keys — equivalent of Go's metadata.Pairs()
    private static final Metadata.Key<String> AUTHORIZATION_KEY =
            Metadata.Key.of("authorization", Metadata.ASCII_STRING_MARSHALLER);
    private static final Metadata.Key<String> REQUEST_ID_KEY =
            Metadata.Key.of("x-request-id", Metadata.ASCII_STRING_MARSHALLER);

    public static void main(String[] args) throws InterruptedException {

        // ── Channel setup ─────────────────────────────────────────────────────
        // ManagedChannel is the Java equivalent of Go's grpc.ClientConn.
        // Like Go, this is a long-lived HTTP/2 connection reused across RPCs.
        ManagedChannel channel = NettyChannelBuilder.forAddress("localhost", 50051)
                .usePlaintext()  // no TLS — dev only
                .keepAliveTime(10, TimeUnit.SECONDS)
                .keepAliveTimeout(5, TimeUnit.SECONDS)
                .keepAliveWithoutCalls(true)
                .build();

        // ── Attach metadata to all calls via a client interceptor ────────────
        // This is the Java equivalent of Go's metadata.NewOutgoingContext().
        // Instead of passing metadata per-call via context, we intercept and
        // inject headers into every outgoing call.
        Channel interceptedChannel = ClientInterceptors.intercept(channel,
                new ClientInterceptor() {
                    @Override
                    public <ReqT, RespT> ClientCall<ReqT, RespT> interceptCall(
                            MethodDescriptor<ReqT, RespT> method,
                            CallOptions callOptions,
                            Channel next) {
                        return new ForwardingClientCall.SimpleForwardingClientCall<>(
                                next.newCall(method, callOptions)) {
                            @Override
                            public void start(Listener<RespT> responseListener, Metadata headers) {
                                headers.put(AUTHORIZATION_KEY, "Bearer valid-token");
                                headers.put(REQUEST_ID_KEY, "java-client-demo-001");
                                super.start(responseListener, headers);
                            }
                        };
                    }
                });

        OrderServiceGrpc.OrderServiceBlockingStub blockingStub =
                OrderServiceGrpc.newBlockingStub(interceptedChannel);
        OrderServiceGrpc.OrderServiceStub asyncStub =
                OrderServiceGrpc.newStub(interceptedChannel);

        // ── 1. Unary: GetOrder (seed order) ──────────────────────────────────
        log.info("=== GetOrder (seed) ===");
        try {
            // Blocking stub: deadline set per-call via withDeadlineAfter()
            // Equivalent to Go's context.WithTimeout()
            Order order = blockingStub
                    .withDeadlineAfter(5, TimeUnit.SECONDS)
                    .getOrder(GetOrderRequest.newBuilder().setOrderId(0).build());

            log.info("got order: id=" + order.getOrderId()
                    + " status=" + order.getStatus()
                    + " total_cents=" + order.getTotalCents()
                    + " items=" + order.getItemsCount());

        } catch (StatusRuntimeException e) {
            // Java exposes gRPC errors as StatusRuntimeException.
            // e.getStatus().getCode() gives you the same codes as Go's status.Code(err).
            log.warning("GetOrder failed: " + e.getStatus());
        }

        // ── 2. Server Streaming: WatchOrders ─────────────────────────────────
        log.info("=== WatchOrders (streaming, runs for 20s) ===");

        // CountDownLatch lets the main thread wait for the stream to finish —
        // equivalent of Go's <-done channel pattern.
        CountDownLatch streamDone = new CountDownLatch(1);

        asyncStub.watchOrders(WatchOrdersRequest.newBuilder().build(),
                new StreamObserver<OrderEvent>() {
                    @Override
                    public void onNext(OrderEvent event) {
                        if (event.getOrderId() == 0) {
                            log.info("heartbeat received");
                            return;
                        }
                        log.info("order event received: order_id=" + event.getOrderId()
                                + " old_status=" + event.getOldStatus()
                                + " new_status=" + event.getNewStatus());
                    }

                    @Override
                    public void onError(Throwable t) {
                        // CANCELLED is expected when we close the stream ourselves.
                        Status status = Status.fromThrowable(t);
                        if (status.getCode() == Status.Code.CANCELLED) {
                            log.info("stream cancelled (expected)");
                        } else {
                            log.warning("stream error: " + status);
                        }
                        streamDone.countDown();
                    }

                    @Override
                    public void onCompleted() {
                        log.info("stream completed by server");
                        streamDone.countDown();
                    }
                });

        // ── 3. Unary: CreateOrder (several times) ────────────────────────────
        Thread.sleep(200); // let the watcher subscribe first

        log.info("=== CreateOrder (3 orders) ===");

        List<CreateOrderRequest> requests = List.of(
                CreateOrderRequest.newBuilder()
                        .addItems(Item.newBuilder().setName("Keyboard").setQuantity(1).setPriceCents(12900).build())
                        .addItems(Item.newBuilder().setName("Mouse").setQuantity(2).setPriceCents(4900).build())
                        .build(),
                CreateOrderRequest.newBuilder()
                        .addItems(Item.newBuilder().setName("Monitor").setQuantity(1).setPriceCents(39900).build())
                        .build(),
                CreateOrderRequest.newBuilder()
                        .addItems(Item.newBuilder().setName("USB Hub").setQuantity(3).setPriceCents(2999).build())
                        .build()
        );

        for (CreateOrderRequest req : requests) {
            try {
                CreateOrderResponse resp = blockingStub
                        .withDeadlineAfter(5, TimeUnit.SECONDS)
                        .createOrder(req);
                log.info("order created: order_id=" + resp.getOrderId()
                        + " total_cents=" + resp.getTotalCents());
            } catch (StatusRuntimeException e) {
                log.warning("CreateOrder failed: " + e.getStatus());
            }
            Thread.sleep(500); // space out creates so events are visible
        }

        // ── 4. Demonstrate NOT_FOUND error handling ──────────────────────────
        log.info("=== GetOrder (non-existent, expect NOT_FOUND) ===");
        try {
            blockingStub
                    .withDeadlineAfter(5, TimeUnit.SECONDS)
                    .getOrder(GetOrderRequest.newBuilder().setOrderId(9999).build());
        } catch (StatusRuntimeException e) {
            // Pattern matching on status codes — equivalent to Go's switch on codes.Code
            switch (e.getStatus().getCode()) {
                case NOT_FOUND -> log.info("received expected NOT_FOUND: " + e.getStatus().getDescription());
                case UNAVAILABLE -> log.warning("service unavailable — would retry with backoff");
                default -> log.warning("unexpected error: " + e.getStatus());
            }
        }

        // Wait for the watcher's 20-second window
        log.info("waiting for watcher to finish...");
        streamDone.await(20, TimeUnit.SECONDS);

        log.info("client demo complete");
        channel.shutdown();
    }
}