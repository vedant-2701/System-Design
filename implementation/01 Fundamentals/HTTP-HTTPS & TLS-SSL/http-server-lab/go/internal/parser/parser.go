package parser

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
)

const (
	// MaxHeaderSize prevents memory exhaustion from malicious clients
	// sending enormous headers. Apache default is 8KB, nginx 8KB.
	MaxHeaderSize = 8 * 1024 // 8KB

	// MaxBodySize prevents OOM from clients streaming infinite bodies.
	// This is a teaching default — production values depend on use case.
	MaxBodySize = 1 * 1024 * 1024 // 1MB

	// MaxHeaderCount prevents header explosion attacks.
	MaxHeaderCount = 100
)

// ParseRequest reads an HTTP/1.1 request from a buffered reader.
//
// HTTP/1.1 wire format (RFC 7230):
//
//	Request-Line CRLF
//	*(Header-Field CRLF)
//	CRLF
//	[message-body]
//
// Design decision: takes a bufio.Reader, not net.Conn directly.
// This makes the parser testable with any io.Reader — including strings.NewReader in tests.
// The server layer owns the connection; the parser owns only byte interpretation.
func ParseRequest(reader *bufio.Reader) (*Request, error) {
	// --- Step 1: Parse request line ---
	// Format: "METHOD /path HTTP/1.1\r\n"
	requestLine, err := reader.ReadString('\n')
	if err != nil {
		if err == io.EOF {
			return nil, fmt.Errorf("connection closed before request line")
		}
		return nil, fmt.Errorf("reading request line: %w", err)
	}

	requestLine = strings.TrimRight(requestLine, "\r\n")
	parts := strings.SplitN(requestLine, " ", 3)
	if len(parts) != 3 {
		return nil, fmt.Errorf("malformed request line: %q", requestLine)
	}

	method, path, version := parts[0], parts[1], parts[2]

	if !isValidMethod(method) {
		return nil, fmt.Errorf("unsupported method: %q", method)
	}
	if version != "HTTP/1.1" && version != "HTTP/1.0" {
		return nil, fmt.Errorf("unsupported HTTP version: %q", version)
	}
	if !strings.HasPrefix(path, "/") {
		return nil, fmt.Errorf("invalid path: %q", path)
	}

	// --- Step 2: Parse headers ---
	// Each line is "Header-Name: header-value\r\n"
	// Terminated by a blank line "\r\n"
	headers := make(map[string]string)
	totalHeaderBytes := 0

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return nil, fmt.Errorf("reading headers: %w", err)
		}

		totalHeaderBytes += len(line)
		if totalHeaderBytes > MaxHeaderSize {
			return nil, fmt.Errorf("headers exceed maximum size of %d bytes", MaxHeaderSize)
		}

		line = strings.TrimRight(line, "\r\n")

		// Blank line = end of headers
		if line == "" {
			break
		}

		if len(headers) >= MaxHeaderCount {
			return nil, fmt.Errorf("too many headers (max %d)", MaxHeaderCount)
		}

		colonIdx := strings.IndexByte(line, ':')
		if colonIdx < 0 {
			return nil, fmt.Errorf("malformed header line: %q", line)
		}

		// RFC 7230: header names are case-insensitive.
		// We normalize to lowercase so handlers never worry about case.
		name := strings.ToLower(strings.TrimSpace(line[:colonIdx]))
		value := strings.TrimSpace(line[colonIdx+1:])

		if name == "" {
			return nil, fmt.Errorf("empty header name in line: %q", line)
		}

		headers[name] = value
	}

	// --- Step 3: Parse body ---
	// Body is only present when Content-Length is set.
	// We do NOT support chunked transfer encoding here (that's an extension exercise).
	var body []byte

	if contentLengthStr, ok := headers["content-length"]; ok {
		contentLength, err := strconv.Atoi(contentLengthStr)
		if err != nil {
			return nil, fmt.Errorf("invalid Content-Length: %q", contentLengthStr)
		}
		if contentLength < 0 {
			return nil, fmt.Errorf("negative Content-Length: %d", contentLength)
		}
		if contentLength > MaxBodySize {
			return nil, fmt.Errorf("body size %d exceeds maximum %d", contentLength, MaxBodySize)
		}

		body = make([]byte, contentLength)
		// io.ReadFull reads exactly len(body) bytes or returns an error.
		// Critical: do NOT use reader.Read() — it may return fewer bytes than requested.
		if _, err := io.ReadFull(reader, body); err != nil {
			return nil, fmt.Errorf("reading body: %w", err)
		}
	}

	return &Request{
		Method:  method,
		Path:    path,
		Version: version,
		Headers: headers,
		Body:    body,
	}, nil
}

// SerializeResponse writes an HTTP/1.1 response to the given writer.
//
// Wire format:
//
//	HTTP/1.1 STATUS_CODE STATUS_TEXT\r\n
//	Header: Value\r\n
//	\r\n
//	body
func SerializeResponse(w io.Writer, resp *Response) error {
	// Status line
	statusLine := fmt.Sprintf("HTTP/1.1 %d %s\r\n", resp.StatusCode, resp.StatusText)
	if _, err := io.WriteString(w, statusLine); err != nil {
		return fmt.Errorf("writing status line: %w", err)
	}

	// Always set Content-Length — required in HTTP/1.1 for keep-alive to work correctly.
	// Without it, the client doesn't know when the body ends and must close the connection.
	resp.Headers["Content-Length"] = strconv.Itoa(len(resp.Body))

	// Server identification header
	resp.Headers["Server"] = "go-http-scratch/1.0"

	// Headers
	for name, value := range resp.Headers {
		line := fmt.Sprintf("%s: %s\r\n", name, value)
		if _, err := io.WriteString(w, line); err != nil {
			return fmt.Errorf("writing header %q: %w", name, err)
		}
	}

	// Blank line separating headers from body
	if _, err := io.WriteString(w, "\r\n"); err != nil {
		return fmt.Errorf("writing header terminator: %w", err)
	}

	// Body
	if len(resp.Body) > 0 {
		if _, err := w.Write(resp.Body); err != nil {
			return fmt.Errorf("writing body: %w", err)
		}
	}

	return nil
}

func isValidMethod(method string) bool {
	switch method {
	case "GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS":
		return true
	default:
		return false
	}
}