// main.go — gRPC server entry point
//
// Wires together:
//   - gRPC server with interceptor chain
//   - OrderService handler
//   - Health check service (standard gRPC health protocol)
//   - Graceful shutdown on SIGINT/SIGTERM
package main

import (
	"context"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/reflection"

	pb "grpc-demo/gen/order/proto"
	"grpc-demo/interceptors"
	"grpc-demo/server"
)

const addr = ":50051"

func main() {
	// Structured JSON logging — parseable by log aggregation systems
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})))

	// ── TCP listener ──────────────────────────────────────────────────────────
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		slog.Error("failed to listen", "addr", addr, "error", err)
		os.Exit(1)
	}

	// ── gRPC server with interceptor chain ───────────────────────────────────
	//
	// Interceptor order (outermost → innermost):
	//   Recovery → Logging → Auth → Handler
	//
	// ChainUnaryInterceptor executes them left-to-right on the way in,
	// and right-to-left on the way out (standard middleware onion model).
	grpcServer := grpc.NewServer(
		// Keepalive: detect dead connections and clean them up.
		// Without this, a client that crashes without closing the connection
		// leaves a zombie connection on the server indefinitely.
		grpc.KeepaliveParams(keepalive.ServerParameters{
			MaxConnectionIdle: 15 * time.Second, // close connections idle > 15s
			Time:              5 * time.Second,  // PING the client every 5s
			Timeout:           1 * time.Second,  // close if no PING ACK within 1s
		}),
		grpc.KeepaliveEnforcementPolicy(keepalive.EnforcementPolicy{
			MinTime:             5 * time.Second, // minimum time between client PINGs
			PermitWithoutStream: true,            // allow PINGs even with no active streams
		}),

		// Interceptor chain — unary RPCs
		grpc.ChainUnaryInterceptor(
			interceptors.UnaryRecovery, // outermost: catches panics everywhere below
			interceptors.UnaryLogging,  // logs every request including rejected ones
			interceptors.UnaryAuth,     // rejects unauthenticated requests
		),

		// Interceptor chain — streaming RPCs
		grpc.ChainStreamInterceptor(
			interceptors.StreamRecovery,
			interceptors.StreamLogging,
			// Note: auth for streams would go here too in production
		),
	)

	// ── Register services ────────────────────────────────────────────────────

	// Business service
	orderServer := server.NewOrderServer()
	pb.RegisterOrderServiceServer(grpcServer, orderServer)

	// Standard gRPC health check protocol.
	// Kubernetes, load balancers, and service meshes call this to determine
	// whether to route traffic to this instance.
	healthServer := health.NewServer()
	healthpb.RegisterHealthServer(grpcServer, healthServer)

	// Mark the service as healthy initially.
	// In production: update this when your DB connection is lost/recovered.
	healthServer.SetServingStatus("order.OrderService", healthpb.HealthCheckResponse_SERVING)
	healthServer.SetServingStatus("", healthpb.HealthCheckResponse_SERVING) // overall server

	// Reflection: allows tools like grpcurl to discover services without .proto files.
	// Enable in development; consider disabling in production for security.
	reflection.Register(grpcServer)

	// ── Start serving ────────────────────────────────────────────────────────
	slog.Info("gRPC server starting", "addr", addr)

	go func() {
		if err := grpcServer.Serve(lis); err != nil {
			slog.Error("server stopped", "error", err)
			os.Exit(1)
		}
	}()

	// ── Graceful shutdown ────────────────────────────────────────────────────
	//
	// On SIGINT or SIGTERM:
	//   1. Mark service as NOT_SERVING (load balancers stop routing new traffic)
	//   2. GracefulStop waits for in-flight RPCs to complete (up to a deadline)
	//   3. If deadline exceeded, force-stop remaining connections
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit

	slog.Info("shutdown signal received", "signal", sig.String())

	// Stop accepting new connections / RPCs
	healthServer.SetServingStatus("order.OrderService", healthpb.HealthCheckResponse_NOT_SERVING)
	healthServer.SetServingStatus("", healthpb.HealthCheckResponse_NOT_SERVING)

	// Give in-flight RPCs 30 seconds to complete
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	stopped := make(chan struct{})
	go func() {
		grpcServer.GracefulStop()
		close(stopped)
	}()

	select {
	case <-stopped:
		slog.Info("server stopped gracefully")
	case <-ctx.Done():
		slog.Warn("graceful shutdown timeout exceeded, forcing stop")
		grpcServer.Stop()
	}
}