package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"http-server/internal/handler"
	"http-server/internal/logger"
	"http-server/internal/router"
	"http-server/internal/server"
)

func main() {
	log := logger.New(logger.INFO)

	// Wire router
	r := router.New()
	r.GET("/health", handler.Health)
	r.GET("/hello", handler.Hello)
	r.GET("/echo", handler.Echo)
	r.POST("/echo", handler.Echo)

	// Wire server
	cfg := server.DefaultConfig()
	srv := server.New(cfg, r, log)

	// OS signal handling for graceful shutdown.
	// SIGTERM: sent by Kubernetes, systemd, docker stop
	// SIGINT:  sent by Ctrl+C during local development
	// Both should trigger graceful shutdown, not hard kill.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)

	// Start server in background goroutine so main can listen for signals.
	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.Start()
	}()

	// Block until signal or startup error.
	select {
	case err := <-errCh:
		if err != nil {
			log.Error("server failed to start", map[string]any{"error": err})
			os.Exit(1)
		}
	case sig := <-sigCh:
		log.Info("received signal", map[string]any{"signal": sig.String()})

		// Give in-flight requests 15 seconds to complete before forcing close.
		// This value should match your longest expected request duration.
		// Kubernetes default terminationGracePeriodSeconds is 30s — stay under it.
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		if err := srv.Shutdown(ctx); err != nil {
			log.Error("shutdown error", map[string]any{"error": err})
			os.Exit(1)
		}
	}
}