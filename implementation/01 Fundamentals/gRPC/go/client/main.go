// client/main.go — gRPC client demonstrating all RPC patterns
//
// Exercises:
//   1. Unary: GetOrder
//   2. Unary: CreateOrder
//   3. Server streaming: WatchOrders (runs concurrently with Creates)
package main

import (
	"context"
	"io"
	"log/slog"
	"os"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/metadata"

	pb "grpc-demo/gen/order/proto"
)

const serverAddr = "localhost:50051"

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})))

	// ── Connection setup ─────────────────────────────────────────────────────
	//
	// In production:
	//   - Replace insecure.NewCredentials() with TLS credentials
	//   - Use dns:///service-name for client-side load balancing
	//   - Configure round_robin policy for gRPC load balancing
	//
	// Example for production:
	//   grpc.Dial("dns:///order-service:50051",
	//       grpc.WithDefaultServiceConfig(`{"loadBalancingPolicy":"round_robin"}`),
	//       grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{})),
	//   )
	conn, err := grpc.NewClient(
		serverAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:                10 * time.Second, // send PING after 10s of inactivity
			Timeout:             5 * time.Second,  // declare connection dead if no PING ACK
			PermitWithoutStream: true,             // PING even with no active RPCs
		}),
	)
	if err != nil {
		slog.Error("failed to connect", "error", err)
		os.Exit(1)
	}
	defer conn.Close()

	client := pb.NewOrderServiceClient(conn)

	// ── Build outgoing metadata (equivalent of HTTP headers) ─────────────────
	// Auth token + correlation ID attached to all RPCs from this client.
	md := metadata.Pairs(
		"authorization", "Bearer valid-token",
		"x-request-id", "client-demo-001",
	)
	baseCtx := metadata.NewOutgoingContext(context.Background(), md)

	// ── 1. Unary: GetOrder (seed order) ─────────────────────────────────────
	slog.Info("=== GetOrder (seed) ===")
	getCtx, getCancel := context.WithTimeout(baseCtx, 5*time.Second)
	defer getCancel()

	order, err := client.GetOrder(getCtx, &pb.GetOrderRequest{OrderId: 0})
	if err != nil {
		slog.Error("GetOrder failed", "error", err)
	} else {
		slog.Info("got order",
			"order_id", order.OrderId,
			"status", order.Status,
			"total_cents", order.TotalCents,
			"items", len(order.Items),
		)
	}

	// ── 2. Server Streaming: WatchOrders ────────────────────────────────────
	// Start the watcher BEFORE creating orders so it catches those events.
	slog.Info("=== WatchOrders (streaming, runs for 20s) ===")

	// Watcher context with 20-second lifetime.
	// When this context is cancelled, the stream closes cleanly.
	watchCtx, watchCancel := context.WithTimeout(baseCtx, 20*time.Second)
	defer watchCancel()

	stream, err := client.WatchOrders(watchCtx, &pb.WatchOrdersRequest{})
	if err != nil {
		slog.Error("WatchOrders failed to start", "error", err)
		os.Exit(1)
	}

	// Receive events in a goroutine — non-blocking relative to the main goroutine.
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			event, err := stream.Recv()
			if err == io.EOF {
				slog.Info("stream closed by server")
				return
			}
			if err != nil {
				// Distinguish between cancellation (expected) and actual errors.
				if watchCtx.Err() != nil {
					slog.Info("stream closed: context done", "reason", watchCtx.Err())
					return
				}
				slog.Error("stream recv error", "error", err)
				return
			}
			// Skip heartbeat events (order_id == 0 with no status transition)
			if event.OrderId == 0 {
				slog.Info("heartbeat received")
				continue
			}
			slog.Info("order event received",
				"order_id", event.OrderId,
				"old_status", event.OldStatus,
				"new_status", event.NewStatus,
			)
		}
	}()

	// ── 3. Unary: CreateOrder (several times) ────────────────────────────────
	// Small delay so WatchOrders goroutine is ready.
	time.Sleep(200 * time.Millisecond)

	orders := []*pb.CreateOrderRequest{
		{Items: []*pb.Item{
			{Name: "Keyboard", Quantity: 1, PriceCents: 12900},
			{Name: "Mouse", Quantity: 2, PriceCents: 4900},
		}},
		{Items: []*pb.Item{
			{Name: "Monitor", Quantity: 1, PriceCents: 39900},
		}},
		{Items: []*pb.Item{
			{Name: "USB Hub", Quantity: 3, PriceCents: 2999},
		}},
	}

	slog.Info("=== CreateOrder (3 orders) ===")
	for i, req := range orders {
		createCtx, createCancel := context.WithTimeout(baseCtx, 5*time.Second)

		resp, err := client.CreateOrder(createCtx, req)
		createCancel()

		if err != nil {
			slog.Error("CreateOrder failed", "i", i, "error", err)
			continue
		}
		slog.Info("order created",
			"order_id", resp.OrderId,
			"total_cents", resp.TotalCents,
		)
		time.Sleep(500 * time.Millisecond) // space out creates so events are visible
	}

	// ── 4. Demonstrate NOT_FOUND error handling ───────────────────────────────
	slog.Info("=== GetOrder (non-existent, expect NOT_FOUND) ===")
	errCtx, errCancel := context.WithTimeout(baseCtx, 5*time.Second)
	defer errCancel()

	_, err = client.GetOrder(errCtx, &pb.GetOrderRequest{OrderId: 9999})
	if err != nil {
		// In production: use status.FromError(err) to extract the code
		// and decide whether to retry, surface to user, or log as error.
		slog.Info("received expected error", "error", err)
	}

	// Wait for watcher to finish its 20-second window or all events received
	slog.Info("waiting for watcher to finish...")
	<-done
	slog.Info("client demo complete")
}