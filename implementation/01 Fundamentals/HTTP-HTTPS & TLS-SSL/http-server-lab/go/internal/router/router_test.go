package router_test

import (
	"testing"

	"http-server/internal/parser"
	"http-server/internal/router"
)

func TestRouter_ExactMatch(t *testing.T) {
	r := router.New()
	r.GET("/health", func(req *parser.Request) *parser.Response {
		return parser.OK([]byte("ok"), "text/plain")
	})

	handler, errResp := r.Lookup("GET", "/health")
	if errResp != nil {
		t.Fatalf("expected handler, got error response: %d", errResp.StatusCode)
	}
	if handler == nil {
		t.Fatal("expected non-nil handler")
	}
}

func TestRouter_NotFound(t *testing.T) {
	r := router.New()

	_, errResp := r.Lookup("GET", "/nonexistent")
	if errResp == nil {
		t.Fatal("expected 404 response, got nil")
	}
	if errResp.StatusCode != 404 {
		t.Errorf("expected 404, got %d", errResp.StatusCode)
	}
}

// Critical: path exists but wrong method must return 405, not 404.
// Returning 404 here confuses clients — they think the resource doesn't exist.
func TestRouter_MethodNotAllowed(t *testing.T) {
	r := router.New()
	r.GET("/users", func(req *parser.Request) *parser.Response {
		return parser.OK([]byte("[]"), "application/json")
	})

	_, errResp := r.Lookup("DELETE", "/users")
	if errResp == nil {
		t.Fatal("expected 405 response, got nil")
	}
	if errResp.StatusCode != 405 {
		t.Errorf("expected 405, got %d", errResp.StatusCode)
	}
}

func TestRouter_MultipleMethodsSamePath(t *testing.T) {
	r := router.New()
	r.GET("/items", func(req *parser.Request) *parser.Response {
		return parser.OK([]byte("get"), "text/plain")
	})
	r.POST("/items", func(req *parser.Request) *parser.Response {
		return parser.OK([]byte("post"), "text/plain")
	})

	getHandler, _ := r.Lookup("GET", "/items")
	postHandler, _ := r.Lookup("POST", "/items")

	if getHandler == nil || postHandler == nil {
		t.Fatal("expected both handlers to be registered")
	}

	getResp := getHandler(&parser.Request{})
	postResp := postHandler(&parser.Request{})

	if string(getResp.Body) != "get" {
		t.Errorf("GET handler returned wrong body: %q", getResp.Body)
	}
	if string(postResp.Body) != "post" {
		t.Errorf("POST handler returned wrong body: %q", postResp.Body)
	}
}

func TestRouter_DuplicateRegistrationPanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic on duplicate route registration, got none")
		}
	}()

	r := router.New()
	r.GET("/dup", func(req *parser.Request) *parser.Response { return nil })
	r.GET("/dup", func(req *parser.Request) *parser.Response { return nil }) // should panic
}