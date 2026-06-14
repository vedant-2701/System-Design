#!/usr/bin/env bash
# setup.sh — Sets up and runs the gRPC demo locally
#
# Prerequisites:
#   - Go 1.21+  (https://go.dev/dl/)
#   - protoc    (https://grpc.io/docs/protoc-installation/)
#
# Install protoc Go plugins (one-time):
#   go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
#   go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
#
# Then run this script.

set -euo pipefail

echo "=== gRPC Demo Setup ==="

# ── Step 1: Generate Go code from .proto ─────────────────────────────────────
echo "Generating protobuf code..."
mkdir -p gen/order

protoc \
  --go_out=gen/order \
  --go_opt=paths=source_relative \
  --go-grpc_out=gen/order \
  --go-grpc_opt=paths=source_relative \
  proto/order.proto

echo "Generated:"
ls -l gen/order

# ── Step 2: Download dependencies ────────────────────────────────────────────
echo "Downloading dependencies..."
go mod tidy

# ── Step 3: Build ─────────────────────────────────────────────────────────────
echo "Building server..."
go build -o bin/server ./main.go

echo "Building client..."
go build -o bin/client ./client/main.go

echo ""
echo "=== Build complete ==="
echo ""
echo "To run:"
echo "  Terminal 1: ./bin/server"
echo "  Terminal 2: ./bin/client"
echo ""
echo "Or with grpcurl (if installed):"
echo "  grpcurl -plaintext localhost:50051 list"
echo "  grpcurl -plaintext -d '{\"order_id\": 0}' localhost:50051 order.OrderService/GetOrder"