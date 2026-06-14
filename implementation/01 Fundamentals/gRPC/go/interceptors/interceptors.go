// Package interceptors provides production-quality gRPC middleware.
//
// Interceptor chain order (outermost → innermost):
//   Recovery → Logging → Auth → RateLimiter → Handler
//
// Recovery is outermost so it catches panics from all inner interceptors.
// Logging is before Auth so every request — including rejected ones — is recorded.
package interceptors

import (
	"context"
	"log/slog"
	"runtime/debug"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// ─── Recovery Interceptor ─────────────────────────────────────────────────────

// UnaryRecovery catches panics in unary handlers and converts them to
// INTERNAL gRPC errors. Without this, a panic crashes the entire server process.
func UnaryRecovery(
	ctx context.Context,
	req any,
	info *grpc.UnaryServerInfo,
	handler grpc.UnaryHandler,
) (resp any, err error) {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("panic recovered in unary handler",
				"method", info.FullMethod,
				"panic", r,
				"stack", string(debug.Stack()),
			)
			// Return a safe error — never expose panic details to the client.
			err = status.Errorf(codes.Internal, "internal server error")
		}
	}()
	return handler(ctx, req)
}

// StreamRecovery catches panics in streaming handlers.
func StreamRecovery(
	srv any,
	ss grpc.ServerStream,
	info *grpc.StreamServerInfo,
	handler grpc.StreamHandler,
) (err error) {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("panic recovered in stream handler",
				"method", info.FullMethod,
				"panic", r,
				"stack", string(debug.Stack()),
			)
			err = status.Errorf(codes.Internal, "internal server error")
		}
	}()
	return handler(srv, ss)
}

// ─── Logging Interceptor ──────────────────────────────────────────────────────

// UnaryLogging logs every RPC: method name, duration, status code, and any error.
// Placed AFTER recovery but BEFORE auth so that even rejected/panicking calls
// produce log entries — critical for security forensics and debugging.
func UnaryLogging(
	ctx context.Context,
	req any,
	info *grpc.UnaryServerInfo,
	handler grpc.UnaryHandler,
) (any, error) {
	start := time.Now()

	// Extract request metadata (equivalent of HTTP headers)
	md, _ := metadata.FromIncomingContext(ctx)
	requestID := mdFirst(md, "x-request-id")

	resp, err := handler(ctx, req)

	code := codes.OK
	if err != nil {
		code = status.Code(err)
	}

	// Structured logging — parseable by log aggregation systems (Loki, ELK)
	attrs := []any{
		"method", info.FullMethod,
		"duration_ms", time.Since(start).Milliseconds(),
		"code", code.String(),
	}
	if requestID != "" {
		attrs = append(attrs, "request_id", requestID)
	}
	if err != nil {
		attrs = append(attrs, "error", err.Error())
	}

	if err != nil {
		slog.Warn("RPC completed with error", attrs...)
	} else {
		slog.Info("RPC completed", attrs...)
	}

	return resp, err
}

// StreamLogging logs the start and end of every streaming RPC.
func StreamLogging(
	srv any,
	ss grpc.ServerStream,
	info *grpc.StreamServerInfo,
	handler grpc.StreamHandler,
) error {
	start := time.Now()
	slog.Info("stream started",
		"method", info.FullMethod,
		"is_client_stream", info.IsClientStream,
		"is_server_stream", info.IsServerStream,
	)

	err := handler(srv, ss)

	code := codes.OK
	if err != nil {
		code = status.Code(err)
	}

	slog.Info("stream finished",
		"method", info.FullMethod,
		"duration_ms", time.Since(start).Milliseconds(),
		"code", code.String(),
	)
	return err
}

// ─── Auth Interceptor ─────────────────────────────────────────────────────────

// contextKey is an unexported type for context keys in this package.
// Avoids collisions with keys from other packages.
type contextKey string

const claimsContextKey contextKey = "claims"

// Claims holds validated token data injected into the context.
type Claims struct {
	UserID string
	Role   string
}

// UnaryAuth validates a Bearer token from gRPC metadata.
// Placed AFTER logging so that auth failures are recorded with full metadata.
//
// In production: replace validateToken with real JWT validation (RS256 signature
// verification, expiry check, issuer check).
func UnaryAuth(
	ctx context.Context,
	req any,
	info *grpc.UnaryServerInfo,
	handler grpc.UnaryHandler,
) (any, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing metadata")
	}

	tokens := md["authorization"]
	if len(tokens) == 0 {
		return nil, status.Error(codes.Unauthenticated, "missing authorization token")
	}

	claims, err := validateToken(tokens[0])
	if err != nil {
		// Return Unauthenticated — not Internal. The caller's token is the problem.
		return nil, status.Errorf(codes.Unauthenticated, "invalid token: %v", err)
	}

	// Inject claims into context so handlers can access user identity
	// without re-parsing the token.
	ctx = context.WithValue(ctx, claimsContextKey, claims)
	return handler(ctx, req)
}

// ClaimsFromContext extracts validated claims injected by UnaryAuth.
// Returns nil if auth interceptor was not in the chain (e.g., in tests).
func ClaimsFromContext(ctx context.Context) *Claims {
	v := ctx.Value(claimsContextKey)
	if v == nil {
		return nil
	}
	return v.(*Claims)
}

// validateToken is a stub. Replace with real JWT validation in production.
func validateToken(token string) (*Claims, error) {
	if token == "Bearer valid-token" {
		return &Claims{UserID: "user-42", Role: "customer"}, nil
	}
	return nil, status.Error(codes.Unauthenticated, "token validation failed")
}

// mdFirst returns the first value for a metadata key, or empty string.
func mdFirst(md metadata.MD, key string) string {
	vals := md[key]
	if len(vals) == 0 {
		return ""
	}
	return vals[0]
}