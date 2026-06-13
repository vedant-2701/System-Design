package parser

// Request represents a parsed HTTP/1.1 request.
// We own this type — not using net/http.Request — so we understand every field.
type Request struct {
	Method     string
	Path       string
	Version    string            // "HTTP/1.1"
	Headers    map[string]string // lowercase keys for case-insensitive lookup
	Body       []byte
	RemoteAddr string // set by the server after accept()
}

// Response represents an HTTP response to be serialized.
type Response struct {
	StatusCode int
	StatusText string
	Headers    map[string]string
	Body       []byte
}

// Common status constructors — reduces magic numbers in handler code.
func OK(body []byte, contentType string) *Response {
	return &Response{
		StatusCode: 200,
		StatusText: "OK",
		Headers:    map[string]string{"Content-Type": contentType},
		Body:       body,
	}
}

func NotFound() *Response {
	body := []byte("404 Not Found")
	return &Response{
		StatusCode: 404,
		StatusText: "Not Found",
		Headers:    map[string]string{"Content-Type": "text/plain"},
		Body:       body,
	}
}

func MethodNotAllowed() *Response {
	body := []byte("405 Method Not Allowed")
	return &Response{
		StatusCode: 405,
		StatusText: "Method Not Allowed",
		Headers:    map[string]string{"Content-Type": "text/plain"},
		Body:       body,
	}
}

func BadRequest(reason string) *Response {
	return &Response{
		StatusCode: 400,
		StatusText: "Bad Request",
		Headers:    map[string]string{"Content-Type": "text/plain"},
		Body:       []byte(reason),
	}
}

func InternalServerError() *Response {
	return &Response{
		StatusCode: 500,
		StatusText: "Internal Server Error",
		Headers:    map[string]string{"Content-Type": "text/plain"},
		Body:       []byte("500 Internal Server Error"),
	}
}