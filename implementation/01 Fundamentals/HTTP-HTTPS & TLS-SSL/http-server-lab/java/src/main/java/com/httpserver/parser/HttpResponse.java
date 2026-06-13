package com.httpserver.parser;

import java.nio.charset.StandardCharsets;
import java.util.HashMap;
import java.util.Map;

/**
 * HTTP response to be serialized to the wire.
 *
 * Mutable by design — SerializeResponse adds Content-Length before writing.
 * Design decision: factory methods enforce correct status text pairing,
 * reducing the risk of mismatched status code + text in handler code.
 */
public final class HttpResponse {
    private final int statusCode;
    private final String statusText;
    private final Map<String, String> headers;
    private final byte[] body;

    public HttpResponse(int statusCode, String statusText,
                        Map<String, String> headers, byte[] body) {
        this.statusCode = statusCode;
        this.statusText = statusText;
        this.headers = new HashMap<>(headers);
        this.body = body == null ? new byte[0] : body;
    }

    // --- Factory methods ---
    // Same pattern as Go's parser.OK(), parser.NotFound(), etc.
    // Keeps handler code readable and type-safe.

    public static HttpResponse ok(byte[] body, String contentType) {
        Map<String, String> headers = new HashMap<>();
        headers.put("Content-Type", contentType);
        return new HttpResponse(200, "OK", headers, body);
    }

    public static HttpResponse ok(String body, String contentType) {
        return ok(body.getBytes(StandardCharsets.UTF_8), contentType);
    }

    public static HttpResponse notFound() {
        return new HttpResponse(404, "Not Found",
                Map.of("Content-Type", "text/plain"),
                "404 Not Found".getBytes(StandardCharsets.UTF_8));
    }

    public static HttpResponse methodNotAllowed() {
        return new HttpResponse(405, "Method Not Allowed",
                Map.of("Content-Type", "text/plain"),
                "405 Method Not Allowed".getBytes(StandardCharsets.UTF_8));
    }

    public static HttpResponse badRequest(String reason) {
        return new HttpResponse(400, "Bad Request",
                Map.of("Content-Type", "text/plain"),
                reason.getBytes(StandardCharsets.UTF_8));
    }

    public static HttpResponse internalServerError() {
        return new HttpResponse(500, "Internal Server Error",
                Map.of("Content-Type", "text/plain"),
                "500 Internal Server Error".getBytes(StandardCharsets.UTF_8));
    }

    public int getStatusCode()          { return statusCode; }
    public String getStatusText()       { return statusText; }
    public Map<String, String> getHeaders() { return headers; }
    public byte[] getBody()             { return body; }
}