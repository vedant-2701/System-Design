package handler

import (
	"encoding/json"
	"fmt"

	"http-server/internal/parser"
)

// Health is a liveness probe endpoint.
// Load balancers and Kubernetes call this to know if the server is alive.
func Health(req *parser.Request) *parser.Response {
	body, _ := json.Marshal(map[string]string{
		"status": "ok",
	})
	return parser.OK(body, "application/json")
}

// Echo reflects the request details back — useful for debugging the parser.
func Echo(req *parser.Request) *parser.Response {
	body, _ := json.Marshal(map[string]any{
		"method":  req.Method,
		"path":    req.Path,
		"headers": req.Headers,
		"body":    string(req.Body),
		"remote":  req.RemoteAddr,
	})
	return parser.OK(body, "application/json")
}

// Hello demonstrates a simple parameterless GET handler.
func Hello(req *parser.Request) *parser.Response {
	body := []byte(fmt.Sprintf("Hello from scratch-built HTTP server! You called %s %s\n", req.Method, req.Path))
	return parser.OK(body, "text/plain")
}