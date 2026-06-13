package router

import (
	"http-server/internal/parser"
)

// HandlerFunc is the signature every route handler must satisfy.
// Simple, composable — same pattern as net/http.HandlerFunc.
type HandlerFunc func(req *parser.Request) *parser.Response

// route holds a registered handler with its allowed method.
type route struct {
	method  string
	handler HandlerFunc
}

// Router maps (method, path) pairs to handlers.
//
// Design decision: exact matching only (see README for rationale).
// The map key is "METHOD /path" — simple, O(1) lookup, zero dependencies.
//
// Extensibility: to add prefix/pattern matching later, replace the map
// with a trie and update the Lookup method. The HandlerFunc interface stays identical.
type Router struct {
	routes map[string]HandlerFunc // key: "METHOD /path"
	paths  map[string]bool        // key: "/path" — for 405 detection
}

func New() *Router {
	return &Router{
		routes: make(map[string]HandlerFunc),
		paths:  make(map[string]bool),
	}
}

// Register adds a handler for a specific method and path combination.
// Panics on duplicate registration — fail fast at startup, not at request time.
func (r *Router) Register(method, path string, handler HandlerFunc) {
	key := method + " " + path
	if _, exists := r.routes[key]; exists {
		panic("duplicate route registration: " + key)
	}
	r.routes[key] = handler
	r.paths[path] = true
}

// Lookup finds the handler for a request.
// Returns (handler, true) on match.
// Returns (nil, false) with appropriate response guidance via status.
//
// Separating lookup from response generation keeps the router pure —
// it doesn't know about response serialization.
func (r *Router) Lookup(method, path string) (HandlerFunc, *parser.Response) {
	key := method + " " + path

	if handler, ok := r.routes[key]; ok {
		return handler, nil
	}

	// Path exists but method not registered → 405, not 404.
	// This distinction matters: 404 means "resource doesn't exist",
	// 405 means "resource exists but you used the wrong verb".
	if r.paths[path] {
		return nil, parser.MethodNotAllowed()
	}

	return nil, parser.NotFound()
}

// Convenience registration methods — reduces boilerplate in main/handler setup.
func (r *Router) GET(path string, handler HandlerFunc)    { r.Register("GET", path, handler) }
func (r *Router) POST(path string, handler HandlerFunc)   { r.Register("POST", path, handler) }
func (r *Router) PUT(path string, handler HandlerFunc)    { r.Register("PUT", path, handler) }
func (r *Router) DELETE(path string, handler HandlerFunc) { r.Register("DELETE", path, handler) }