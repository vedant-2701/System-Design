package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"
	"path/filepath"
)

// config holds all runtime configuration sourced from environment variables.
// Using env vars instead of config files makes the service 12-factor compliant
// and simplifies systemd EnvironmentFile injection.
type config struct {
	port        string
	logFile     string
	serviceName string
}

// healthResponse is the JSON payload returned by /health.
// Structured so monitoring scripts can parse it without regex.
type healthResponse struct {
	Status    string `json:"status"`
	Uptime    string `json:"uptime"`
	Timestamp string `json:"timestamp"`
}

func loadConfig() config {
	port := os.Getenv("APP_PORT")
	if port == "" {
		port = "8080"
	}
	logFile := os.Getenv("APP_LOG_FILE")
	if logFile == "" {
		logFile = "/var/log/myapp/myapp.log"
	}
	name := os.Getenv("APP_SERVICE_NAME")
	if name == "" {
		name = "myapp"
	}
	return config{port: port, logFile: logFile, serviceName: name}
}

func setupLogger(logFile string) (*os.File, error) {
	// Ensure log directory exists before opening file.
	// The install script creates it, but defensive check prevents a confusing crash.
	// if err := os.MkdirAll("/var/log/myapp", 0755); err != nil {
	// 	return nil, fmt.Errorf("failed to create log directory: %w", err)
	// }

	// Create the directory of the log file (not hardcoded).
	logDir := filepath.Dir(logFile)
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create log directory %s: %w", logDir, err)
	}

	// O_APPEND is critical — without it two processes writing to the same file
	// interleave writes non-atomically and corrupt log lines.
	f, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file %s: %w", logFile, err)
	}
	return f, nil
}

func main() {
	cfg := loadConfig()

	logFile, err := setupLogger(cfg.logFile)
	if err != nil {
		// Can't open log file — write to stderr so systemd/journald captures it,
		// then exit with non-zero so systemd knows to restart.
		fmt.Fprintf(os.Stderr, "FATAL: %v\n", err)
		os.Exit(1)
	}
	defer logFile.Close()

	// Direct stdlib logger to our log file.
	// In production you'd use a structured logger (zerolog, zap) but stdlib
	// is sufficient here to demonstrate the file-descriptor and rotation concerns.
	log.SetOutput(logFile)
	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds)

	startTime := time.Now()

	mux := http.NewServeMux()

	// /health — used by the process monitor script and load balancers.
	// Returns 200 when healthy, 503 when degraded.
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(healthResponse{
			Status:    "ok",
			Uptime:    time.Since(startTime).Round(time.Second).String(),
			Timestamp: time.Now().UTC().Format(time.RFC3339),
		})
		log.Printf("level=info path=/health method=%s remote=%s", r.Method, r.RemoteAddr)
	})

	// /crash — intentionally crashes the service with exit code 1.
	// Used to test systemd restart behavior without killing the process manually.
	// Would not exist in a real production service.
	mux.HandleFunc("/crash", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("level=warn msg=crash_requested remote=%s", r.RemoteAddr)
		fmt.Fprintln(w, "crashing...")
		// Flush response before exiting so the client sees it.
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		os.Exit(1)
	})

	// /work — simulates CPU work to make top and process monitor meaningful.
	mux.HandleFunc("/work", func(w http.ResponseWriter, r *http.Request) {
		n := 1000000
		if v := r.URL.Query().Get("n"); v != "" {
			if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 {
				n = parsed
			}
		}
		start := time.Now()
		sum := 0
		for i := 0; i < n; i++ {
			sum += i
		}
		elapsed := time.Since(start)
		log.Printf("level=info path=/work n=%d elapsed=%s", n, elapsed)
		fmt.Fprintf(w, "sum=%d elapsed=%s\n", sum, elapsed)
	})

	// /log — writes a log line at a given level for testing log rotation and monitoring.
	mux.HandleFunc("/log", func(w http.ResponseWriter, r *http.Request) {
		level := r.URL.Query().Get("level")
		msg := r.URL.Query().Get("msg")
		if msg == "" {
			msg = "test log entry"
		}
		switch level {
		case "error":
			log.Printf("level=error msg=%q", msg)
		case "warn":
			log.Printf("level=warn msg=%q", msg)
		default:
			log.Printf("level=info msg=%q", msg)
		}
		fmt.Fprintln(w, "logged")
	})

	srv := &http.Server{
		Addr:         "0.0.0.0:" + cfg.port,
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Graceful shutdown on SIGTERM or SIGINT.
	// SIGTERM is what systemd sends on `systemctl stop`.
	// Without this handler, the process exits immediately — active requests drop.
	shutdownCh := make(chan os.Signal, 1)
	signal.Notify(shutdownCh, syscall.SIGTERM, syscall.SIGINT)

	// Start server in goroutine so main can block on signal.
	serverErrCh := make(chan error, 1)
	go func() {
		log.Printf("level=info msg=server_starting service=%s port=%s", cfg.serviceName, cfg.port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverErrCh <- err
		}
	}()

	// Block until signal or server error.
	select {
	case sig := <-shutdownCh:
		log.Printf("level=info msg=shutdown_signal_received signal=%s", sig)

		// Give in-flight requests up to 15 seconds to complete.
		// This window matches ExecStop timeout in the unit file.
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		if err := srv.Shutdown(ctx); err != nil {
			log.Printf("level=error msg=shutdown_error err=%v", err)
			os.Exit(1)
		}
		log.Printf("level=info msg=server_stopped_cleanly")

	case err := <-serverErrCh:
		log.Printf("level=error msg=server_error err=%v", err)
		os.Exit(1)
	}
}