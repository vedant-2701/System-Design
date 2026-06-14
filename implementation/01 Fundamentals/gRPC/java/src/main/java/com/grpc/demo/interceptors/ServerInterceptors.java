package com.example.grpc.interceptors;

import io.grpc.*;

import java.util.logging.Logger;

/**
 * gRPC interceptors in Java.
 *
 * Java interceptors use a different API than Go:
 *
 * Go:   func(ctx, req, info, handler) → wraps handler with a closure
 * Java: ServerInterceptor.interceptCall(call, headers, next)
 *       → wraps ServerCall with a ForwardingServerCall
 *       → wraps ServerCallListener with a ForwardingServerCallListener
 *
 * The Java model is more verbose but also more flexible:
 * - ServerCall lets you inspect/modify the response
 * - ServerCallListener lets you intercept individual stream events
 *   (onMessage, onHalfClose, onCancel, onComplete)
 *
 * INTERCEPTOR CHAIN ORDER:
 * The FIRST interceptor registered is the OUTERMOST (runs first on ingress).
 * In ServerMain we register: [Recovery, Logging, Auth]
 * So execution order is: Recovery → Logging → Auth → Handler
 *
 * This matches our Go implementation's chain order.
 */
public class ServerInterceptors {

    private static final Logger log = Logger.getLogger(ServerInterceptors.class.getName());

    // ── Context Keys ─────────────────────────────────────────────────────────
    // Context.Key is Java gRPC's equivalent of Go's context.WithValue keys.
    // Typed keys prevent collisions between interceptors.
    public static final Context.Key<String> USER_ID_KEY =
            Context.key("user-id");
    public static final Context.Key<String> USER_ROLE_KEY =
            Context.key("user-role");

    // ── Metadata Keys ────────────────────────────────────────────────────────
    // Metadata.Key is Java gRPC's equivalent of Go's metadata.MD map keys.
    private static final Metadata.Key<String> AUTHORIZATION_KEY =
            Metadata.Key.of("authorization", Metadata.ASCII_STRING_MARSHALLER);
    private static final Metadata.Key<String> REQUEST_ID_KEY =
            Metadata.Key.of("x-request-id", Metadata.ASCII_STRING_MARSHALLER);

    // ── Logging Interceptor ───────────────────────────────────────────────────

    /**
     * Logs every RPC: method, duration, status code.
     * Placed before Auth so even rejected calls are logged.
     */
    public static ServerInterceptor loggingInterceptor() {
        return new ServerInterceptor() {
            @Override
            public <ReqT, RespT> ServerCall.Listener<ReqT> interceptCall(
                    ServerCall<ReqT, RespT> call,
                    Metadata headers,
                    ServerCallHandler<ReqT, RespT> next) {

                String method = call.getMethodDescriptor().getFullMethodName();
                String requestId = headers.get(REQUEST_ID_KEY);
                long startNanos = System.nanoTime();

                log.info("RPC started: method=" + method +
                        (requestId != null ? " request_id=" + requestId : ""));

                // Wrap the call to intercept the response/close event
                ServerCall<ReqT, RespT> wrappedCall = new ForwardingServerCall
                        .SimpleForwardingServerCall<>(call) {
                    @Override
                    public void close(Status status, Metadata trailers) {
                        long durationMs = (System.nanoTime() - startNanos) / 1_000_000;
                        String logMsg = "RPC finished: method=" + method
                                + " duration_ms=" + durationMs
                                + " code=" + status.getCode();
                        if (!status.isOk()) {
                            log.warning(logMsg + " description=" + status.getDescription());
                        } else {
                            log.info(logMsg);
                        }
                        super.close(status, trailers);
                    }
                };

                return next.startCall(wrappedCall, headers);
            }
        };
    }

    // ── Auth Interceptor ──────────────────────────────────────────────────────

    /**
     * Validates Bearer token from metadata.
     * Injects validated user identity into gRPC Context so handlers can
     * access it via Context.current().withValue() / USER_ID_KEY.get().
     *
     * gRPC Context vs Java ThreadLocal:
     * - ThreadLocal is dangerous with virtual threads (threads are reused)
     * - gRPC Context propagates across the call chain automatically
     * - Handlers access it via Context.current() — no parameter threading needed
     */
    public static ServerInterceptor authInterceptor() {
        return new ServerInterceptor() {
            @Override
            public <ReqT, RespT> ServerCall.Listener<ReqT> interceptCall(
                    ServerCall<ReqT, RespT> call,
                    Metadata headers,
                    ServerCallHandler<ReqT, RespT> next) {

                String authHeader = headers.get(AUTHORIZATION_KEY);

                if (authHeader == null || authHeader.isEmpty()) {
                    call.close(
                            Status.UNAUTHENTICATED.withDescription("missing authorization token"),
                            new Metadata()
                    );
                    // Return a no-op listener — the call is already closed
                    return new ServerCall.Listener<>() {};
                }

                try {
                    UserClaims claims = validateToken(authHeader);

                    // Inject claims into gRPC Context
                    // All downstream handlers and interceptors access this via:
                    //   USER_ID_KEY.get(Context.current())
                    Context ctx = Context.current()
                            .withValue(USER_ID_KEY, claims.userId())
                            .withValue(USER_ROLE_KEY, claims.role());

                    // attach() switches to this context for this call's thread
                    // detach() restores the previous context — ALWAYS in a finally
                    Context previous = ctx.attach();
                    try {
                        return Contexts.interceptCall(ctx, call, headers, next);
                    } finally {
                        ctx.detach(previous);
                    }

                } catch (StatusRuntimeException e) {
                    call.close(e.getStatus(), new Metadata());
                    return new ServerCall.Listener<>() {};
                }
            }
        };
    }

    // ── Recovery Interceptor ──────────────────────────────────────────────────

    /**
     * Catches unchecked exceptions from downstream interceptors and handlers.
     * Returns INTERNAL status rather than crashing the server thread.
     *
     * In Java, an unhandled exception in a gRPC handler causes the framework
     * to close the call with UNKNOWN status and a potentially leaky stack trace.
     * This interceptor catches it before that happens and returns a safe INTERNAL.
     */
    public static ServerInterceptor recoveryInterceptor() {
        return new ServerInterceptor() {
            @Override
            public <ReqT, RespT> ServerCall.Listener<ReqT> interceptCall(
                    ServerCall<ReqT, RespT> call,
                    Metadata headers,
                    ServerCallHandler<ReqT, RespT> next) {

                ServerCall.Listener<ReqT> delegate;
                try {
                    delegate = next.startCall(call, headers);
                } catch (Exception e) {
                    log.severe("Panic in interceptor chain: " + e.getMessage());
                    call.close(Status.INTERNAL.withDescription("internal server error"), new Metadata());
                    return new ServerCall.Listener<>() {};
                }

                // Also wrap the listener to catch panics during message processing
                return new ForwardingServerCallListener.SimpleForwardingServerCallListener<>(delegate) {
                    @Override
                    public void onMessage(ReqT message) {
                        try {
                            super.onMessage(message);
                        } catch (Exception e) {
                            log.severe("Panic processing message: " + e.getMessage());
                            call.close(Status.INTERNAL.withDescription("internal server error"), new Metadata());
                        }
                    }

                    @Override
                    public void onHalfClose() {
                        try {
                            super.onHalfClose();
                        } catch (Exception e) {
                            log.severe("Panic in onHalfClose: " + e.getMessage());
                            call.close(Status.INTERNAL.withDescription("internal server error"), new Metadata());
                        }
                    }
                };
            }
        };
    }

    // ── Token validation stub ─────────────────────────────────────────────────

    private record UserClaims(String userId, String role) {}

    private static UserClaims validateToken(String token) {
        if ("Bearer valid-token".equals(token)) {
            return new UserClaims("user-42", "customer");
        }
        throw Status.UNAUTHENTICATED
                .withDescription("invalid token")
                .asRuntimeException();
    }
}