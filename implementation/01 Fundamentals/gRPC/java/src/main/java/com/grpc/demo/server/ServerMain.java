package com.example.grpc.server;

import com.example.grpc.interceptors.ServerInterceptors;
import io.grpc.Server;
import io.grpc.health.v1.HealthCheckResponse;
import io.grpc.protobuf.services.HealthStatusManager;
import io.grpc.protobuf.services.ProtoReflectionService;
import io.grpc.netty.shaded.io.grpc.netty.GrpcSslContexts;
import io.grpc.netty.shaded.io.grpc.netty.NettyServerBuilder;
import io.grpc.netty.shaded.io.netty.channel.ChannelOption;

import java.util.concurrent.Executor;
import java.util.concurrent.Executors;
import java.util.concurrent.TimeUnit;
import java.util.logging.Logger;

/**
 * Server entry point.
 *
 * KEY DIFFERENCE FROM GO — CONCURRENCY MODEL:
 *
 * Go: each RPC handler runs in a goroutine automatically. Goroutines are
 *     cheap (2KB initial stack), the Go runtime schedules them onto OS threads.
 *
 * Java (pre-21): each RPC handler runs on a thread from a fixed-size thread pool
 *     (default grpc-default-executor). Thread pool size limits concurrent RPCs.
 *     A handler that blocks (e.g., synchronous JDBC call) ties up a pool thread —
 *     under load, the pool exhausts and new RPCs queue.
 *
 * Java 21 (Project Loom): Executors.newVirtualThreadPerTaskExecutor() gives
 *     each RPC its own virtual thread. Virtual threads are cheap like goroutines.
 *     A blocking call (JDBC, file I/O) parks the virtual thread WITHOUT blocking
 *     the underlying OS "carrier" thread — the carrier thread picks up other
 *     virtual threads. This closes most of the gap with Go's goroutine model
 *     for I/O-bound workloads.
 *
 * We configure this executor on the gRPC server builder below.
 */
public class ServerMain {

    private static final Logger log = Logger.getLogger(ServerMain.class.getName());
    private static final int PORT = 50051;

    public static void main(String[] args) throws Exception {

        OrderServiceImpl orderService = new OrderServiceImpl();

        // ── Health check service ─────────────────────────────────────────────
        // HealthStatusManager is grpc-java's equivalent of Go's health.NewServer().
        // It implements the standard gRPC Health Checking Protocol.
        HealthStatusManager healthManager = new HealthStatusManager();

        // ── Virtual thread executor ──────────────────────────────────────────
        // Every RPC call's handler code runs on a fresh virtual thread.
        // This replaces the fixed-size default executor.
        Executor virtualThreadExecutor = Executors.newVirtualThreadPerTaskExecutor();

        // ── Build interceptor chain ───────────────────────────────────────────
        // io.grpc.ServerInterceptors.interceptForward applies interceptors in
        // the order given, with the FIRST being OUTERMOST — same semantics
        // as Go's ChainUnaryInterceptor.
        //
        // Order: Recovery (outermost) → Logging → Auth → Handler
        var serviceWithInterceptors = io.grpc.ServerInterceptors.interceptForward(
                orderService,
                ServerInterceptors.recoveryInterceptor(),
                ServerInterceptors.loggingInterceptor(),
                ServerInterceptors.authInterceptor()
        );

        // ── Build server ──────────────────────────────────────────────────────
        Server server = NettyServerBuilder.forPort(PORT)
                .executor(virtualThreadExecutor)
                .addService(serviceWithInterceptors)
                .addService(healthManager.getHealthService())

                // Reflection — same purpose as Go's reflection.Register()
                .addService(ProtoReflectionService.newInstance())

                // ── Keepalive configuration ─────────────────────────────────────
                // Equivalent to Go's grpc.KeepaliveParams / KeepaliveEnforcementPolicy
                .keepAliveTime(5, TimeUnit.SECONDS)       // PING client every 5s
                .keepAliveTimeout(1, TimeUnit.SECONDS)    // close if no ACK in 1s
                .permitKeepAliveWithoutCalls(true)        // allow PINGs with no active RPCs
                .maxConnectionIdle(15, TimeUnit.SECONDS)  // close idle connections

                .build();

        server.start();
        log.info("gRPC server started on port " + PORT);

        // Mark services healthy — load balancers/k8s probes will now pass
        healthManager.setStatus("order.OrderService", HealthCheckResponse.ServingStatus.SERVING);
        healthManager.setStatus("", HealthCheckResponse.ServingStatus.SERVING);

        // ── Graceful shutdown ─────────────────────────────────────────────────
        // Registered as a JVM shutdown hook — triggered on SIGINT/SIGTERM.
        Runtime.getRuntime().addShutdownHook(new Thread(() -> {
            log.info("Shutdown signal received");

            // Step 1: mark NOT_SERVING — load balancers stop routing new traffic
            healthManager.setStatus("order.OrderService", HealthCheckResponse.ServingStatus.NOT_SERVING);
            healthManager.setStatus("", HealthCheckResponse.ServingStatus.NOT_SERVING);

            // Step 2: close active streaming subscriptions
            orderService.shutdown();

            // Step 3: graceful shutdown — wait for in-flight RPCs
            server.shutdown();
            try {
                if (!server.awaitTermination(30, TimeUnit.SECONDS)) {
                    log.warning("Graceful shutdown timeout exceeded, forcing stop");
                    server.shutdownNow();
                }
            } catch (InterruptedException e) {
                server.shutdownNow();
                Thread.currentThread().interrupt();
            }
            log.info("Server stopped");
        }));

        server.awaitTermination();
    }
}