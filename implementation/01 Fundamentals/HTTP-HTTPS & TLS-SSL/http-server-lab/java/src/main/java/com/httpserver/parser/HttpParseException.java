package com.httpserver.parser;

/**
 * Signals a protocol-level parse failure — malformed HTTP, size limits exceeded, etc.
 *
 * Checked exception by design: callers (ConnectionHandler) must explicitly decide
 * what to do when parsing fails. Unchecked would let it silently propagate.
 *
 * Java vs Go: Go returns (nil, error). Java throws checked exceptions.
 * Both enforce error handling at compile time.
 */
public class HttpParseException extends Exception {
    public HttpParseException(String message) {
        super(message);
    }
}