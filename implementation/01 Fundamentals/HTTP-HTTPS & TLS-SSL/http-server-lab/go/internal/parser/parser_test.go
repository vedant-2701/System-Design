package parser_test

import (
	"bufio"
	"fmt"
	"strings"
	"testing"

	"http-server/internal/parser"
)

// helper builds a bufio.Reader from a raw HTTP request string.
// CRLF must be explicit in test strings — this is intentional.
// It forces test authors to think about the wire format, not paper over it.
func requestReader(raw string) *bufio.Reader {
	return bufio.NewReader(strings.NewReader(raw))
}

// --- Happy path tests ---

func TestParseRequest_SimpleGET(t *testing.T) {
	raw := "GET /hello HTTP/1.1\r\nHost: localhost\r\n\r\n"
	req, err := parser.ParseRequest(requestReader(raw))

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if req.Method != "GET" {
		t.Errorf("method: got %q, want %q", req.Method, "GET")
	}
	if req.Path != "/hello" {
		t.Errorf("path: got %q, want %q", req.Path, "/hello")
	}
	if req.Version != "HTTP/1.1" {
		t.Errorf("version: got %q, want %q", req.Version, "HTTP/1.1")
	}
	if req.Headers["host"] != "localhost" {
		t.Errorf("host header: got %q, want %q", req.Headers["host"], "localhost")
	}
}

func TestParseRequest_POSTWithBody(t *testing.T) {
	body := `{"name":"vedant"}`
	raw := "POST /users HTTP/1.1\r\n" +
		"Host: localhost\r\n" +
		"Content-Type: application/json\r\n" +
		"Content-Length: " + strings.Repeat("", 0) + "17\r\n" +
		"\r\n" +
		body

	req, err := parser.ParseRequest(requestReader(raw))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(req.Body) != body {
		t.Errorf("body: got %q, want %q", string(req.Body), body)
	}
	if req.Headers["content-type"] != "application/json" {
		t.Errorf("content-type: got %q", req.Headers["content-type"])
	}
}

// Header names must be normalized to lowercase regardless of wire format.
// RFC 7230: header field names are case-insensitive.
func TestParseRequest_HeaderCaseNormalization(t *testing.T) {
	raw := "GET / HTTP/1.1\r\n" +
		"CONTENT-TYPE: text/plain\r\n" +
		"X-Custom-Header: MyValue\r\n" +
		"\r\n"

	req, err := parser.ParseRequest(requestReader(raw))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if req.Headers["content-type"] != "text/plain" {
		t.Errorf("expected lowercase header lookup to work, got: %q", req.Headers["content-type"])
	}
	if req.Headers["x-custom-header"] != "MyValue" {
		t.Errorf("expected lowercase header lookup to work, got: %q", req.Headers["x-custom-header"])
	}
}

func TestParseRequest_NoBody_NoContentLength(t *testing.T) {
	raw := "GET /health HTTP/1.1\r\nHost: localhost\r\n\r\n"
	req, err := parser.ParseRequest(requestReader(raw))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(req.Body) != 0 {
		t.Errorf("expected empty body, got %d bytes", len(req.Body))
	}
}

// --- Malformed request tests ---
// These are not edge cases — they are guaranteed to happen in production.
// Every public HTTP server receives malformed requests constantly.

func TestParseRequest_MalformedRequestLine(t *testing.T) {
	cases := []struct {
		name string
		raw  string
	}{
		{"missing version", "GET /path\r\n\r\n"},
		{"missing path", "GET HTTP/1.1\r\n\r\n"},
		{"empty line", "\r\n"},
		{"garbage", "this is not http\r\n\r\n"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := parser.ParseRequest(requestReader(tc.raw))
			if err == nil {
				t.Error("expected error for malformed request, got nil")
			}
		})
	}
}

func TestParseRequest_UnsupportedMethod(t *testing.T) {
	raw := "BREW /coffee HTTP/1.1\r\nHost: localhost\r\n\r\n"
	_, err := parser.ParseRequest(requestReader(raw))
	if err == nil {
		t.Error("expected error for unsupported method, got nil")
	}
}

func TestParseRequest_InvalidContentLength(t *testing.T) {
	cases := []struct {
		name          string
		contentLength string
	}{
		{"non-numeric", "abc"},
		{"negative", "-1"},
		{"float", "1.5"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			raw := "POST /data HTTP/1.1\r\n" +
				"Content-Length: " + tc.contentLength + "\r\n" +
				"\r\n"
			_, err := parser.ParseRequest(requestReader(raw))
			if err == nil {
				t.Errorf("expected error for Content-Length %q, got nil", tc.contentLength)
			}
		})
	}
}

func TestParseRequest_BodyExceedsMaxSize(t *testing.T) {
	// Claim a body larger than MaxBodySize — parser must reject before reading
	raw := "POST /upload HTTP/1.1\r\n" +
		"Content-Length: 999999999\r\n" +
		"\r\n"
	_, err := parser.ParseRequest(requestReader(raw))
	if err == nil {
		t.Error("expected error for oversized body, got nil")
	}
}

func TestParseRequest_HeadersTooLarge(t *testing.T) {
	// Build a request with a single enormous header value
	bigValue := strings.Repeat("x", parser.MaxHeaderSize+1)
	raw := "GET / HTTP/1.1\r\n" +
		"X-Big: " + bigValue + "\r\n" +
		"\r\n"
	_, err := parser.ParseRequest(requestReader(raw))
	if err == nil {
		t.Error("expected error for oversized headers, got nil")
	}
}

func TestParseRequest_TooManyHeaders(t *testing.T) {
	var sb strings.Builder
	sb.WriteString("GET / HTTP/1.1\r\n")
	// Each header must have a unique name — map deduplication would otherwise
	// hide the count from the parser's MaxHeaderCount check.
	for i := 0; i <= parser.MaxHeaderCount; i++ {
		sb.WriteString(fmt.Sprintf("X-Header-%d: value\r\n", i))
	}
	sb.WriteString("\r\n")

	_, err := parser.ParseRequest(requestReader(sb.String()))
	if err == nil {
		t.Error("expected error for too many headers, got nil")
	}
}

func TestParseRequest_MalformedHeader(t *testing.T) {
	// Header line missing colon separator
	raw := "GET / HTTP/1.1\r\n" +
		"InvalidHeaderWithoutColon\r\n" +
		"\r\n"
	_, err := parser.ParseRequest(requestReader(raw))
	if err == nil {
		t.Error("expected error for malformed header line, got nil")
	}
}

// --- Serialization tests ---

func TestSerializeResponse_StatusLineAndHeaders(t *testing.T) {
	var buf strings.Builder
	resp := parser.OK([]byte("hello"), "text/plain")

	err := parser.SerializeResponse(&buf, resp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.HasPrefix(output, "HTTP/1.1 200 OK\r\n") {
		t.Errorf("expected status line prefix, got: %q", output[:30])
	}
	if !strings.Contains(output, "Content-Length: 5") {
		t.Errorf("expected Content-Length header, output: %q", output)
	}
	if !strings.Contains(output, "\r\n\r\n") {
		t.Error("expected blank line separating headers from body")
	}
	if !strings.HasSuffix(output, "hello") {
		t.Errorf("expected body at end, output: %q", output)
	}
}

func TestSerializeResponse_EmptyBody(t *testing.T) {
	var buf strings.Builder
	resp := &parser.Response{
		StatusCode: 204,
		StatusText: "No Content",
		Headers:    map[string]string{},
		Body:       nil,
	}

	err := parser.SerializeResponse(&buf, resp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "Content-Length: 0") {
		t.Errorf("expected Content-Length: 0 for empty body, output: %q", output)
	}
}