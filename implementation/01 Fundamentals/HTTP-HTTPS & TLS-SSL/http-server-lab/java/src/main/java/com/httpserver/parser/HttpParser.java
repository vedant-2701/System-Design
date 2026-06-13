package com.httpserver.parser;

import java.io.*;
import java.nio.charset.StandardCharsets;
import java.util.HashMap;
import java.util.Map;
import java.util.Set;

/**
 * HTTP/1.1 request parser and response serializer.
 *
 * Conceptual equivalent of Go's parser.go.
 * Takes a BufferedReader (not Socket directly) — same testability rationale:
 * parser is pure byte logic, decoupled from network I/O.
 *
 * Key Java vs Go difference:
 * Go uses multiple return values (value, error).
 * Java uses checked exceptions for parse errors — callers must handle them.
 * Both force the caller to deal with the error case explicitly.
 */
public final class HttpParser {

    // Mirrors Go constants — same attack surface reasoning
    public static final int MAX_HEADER_SIZE  = 8 * 1024;       // 8KB
    public static final int MAX_BODY_SIZE    = 1024 * 1024;    // 1MB
    public static final int MAX_HEADER_COUNT = 100;

    private static final Set<String> VALID_METHODS = Set.of(
            "GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS"
    );

    // Static utility class — no instances needed
    private HttpParser() {}

    /**
     * Parses one HTTP/1.1 request from the given reader.
     *
     * @throws HttpParseException on malformed input, size limit violations
     * @throws IOException        on underlying I/O failure
     * @throws EOFException       when client closes connection cleanly
     */
    public static HttpRequest parseRequest(BufferedReader reader,
                                           BufferedInputStream rawStream,
                                           String remoteAddr)
            throws IOException, HttpParseException {

        // --- Step 1: Request line ---
        String requestLine = reader.readLine();
        if (requestLine == null) {
            throw new EOFException("connection closed before request line");
        }

        String[] parts = requestLine.split(" ", 3);
        if (parts.length != 3) {
            throw new HttpParseException("malformed request line: " + requestLine);
        }

        String method  = parts[0];
        String path    = parts[1];
        String version = parts[2];

        if (!VALID_METHODS.contains(method)) {
            throw new HttpParseException("unsupported method: " + method);
        }
        if (!version.equals("HTTP/1.1") && !version.equals("HTTP/1.0")) {
            throw new HttpParseException("unsupported HTTP version: " + version);
        }
        if (!path.startsWith("/")) {
            throw new HttpParseException("invalid path: " + path);
        }

        // --- Step 2: Headers ---
        Map<String, String> headers = new HashMap<>();
        int totalHeaderBytes = 0;

        while (true) {
            String line = reader.readLine();
            if (line == null) {
                throw new HttpParseException("connection closed mid-headers");
            }

            totalHeaderBytes += line.length() + 2; // +2 for CRLF
            if (totalHeaderBytes > MAX_HEADER_SIZE) {
                throw new HttpParseException(
                        "headers exceed maximum size of " + MAX_HEADER_SIZE + " bytes");
            }

            if (line.isEmpty()) {
                break; // blank line = end of headers
            }

            if (headers.size() >= MAX_HEADER_COUNT) {
                throw new HttpParseException(
                        "too many headers (max " + MAX_HEADER_COUNT + ")");
            }

            int colonIdx = line.indexOf(':');
            if (colonIdx < 0) {
                throw new HttpParseException("malformed header line: " + line);
            }

            // RFC 7230: header names are case-insensitive — normalize to lowercase
            String name  = line.substring(0, colonIdx).trim().toLowerCase();
            String value = line.substring(colonIdx + 1).trim();

            if (name.isEmpty()) {
                throw new HttpParseException("empty header name in: " + line);
            }

            headers.put(name, value);
        }

        // --- Step 3: Body ---
        // Java design note: BufferedReader converts bytes to chars using charset.
        // For body reading we need raw bytes — so we use rawStream directly.
        // This is a Java-specific concern; Go's bufio.Reader works at byte level throughout.
        byte[] body = new byte[0];

        if (headers.containsKey("content-length")) {
            int contentLength;
            try {
                contentLength = Integer.parseInt(headers.get("content-length").trim());
            } catch (NumberFormatException e) {
                throw new HttpParseException(
                        "invalid Content-Length: " + headers.get("content-length"));
            }

            if (contentLength < 0) {
                throw new HttpParseException("negative Content-Length: " + contentLength);
            }
            if (contentLength > MAX_BODY_SIZE) {
                throw new HttpParseException(
                        "body size " + contentLength + " exceeds maximum " + MAX_BODY_SIZE);
            }

            char[] charBody = new char[contentLength];
            int bytesRead = 0;
            while (bytesRead < contentLength) {
                int read = reader.read(charBody, bytesRead, contentLength - bytesRead);
                if (read == -1) {
                    break;
                }
                bytesRead += read;
            }
            if (bytesRead < contentLength) {
                throw new HttpParseException(
                        "body truncated: expected " + contentLength + " got " + bytesRead);
            }
            body = new byte[contentLength];
            for (int i = 0; i < contentLength; i++) {
                body[i] = (byte) charBody[i];
            }
        }

        return new HttpRequest(method, path, version, headers, body, remoteAddr);
    }

    /**
     * Serializes an HttpResponse to the given OutputStream.
     *
     * Design: writes directly to OutputStream (not Writer) to handle binary bodies correctly.
     * Mixing Writer and OutputStream for the same connection causes encoding bugs.
     */
    public static void serializeResponse(OutputStream out, HttpResponse resp)
            throws IOException {

        // Content-Length must always be set for keep-alive to work correctly
        resp.getHeaders().put("Content-Length", String.valueOf(resp.getBody().length));
        resp.getHeaders().put("Server", "java-http-scratch/1.0");

        StringBuilder sb = new StringBuilder();

        // Status line
        sb.append("HTTP/1.1 ")
          .append(resp.getStatusCode())
          .append(" ")
          .append(resp.getStatusText())
          .append("\r\n");

        // Headers
        for (Map.Entry<String, String> entry : resp.getHeaders().entrySet()) {
            sb.append(entry.getKey()).append(": ").append(entry.getValue()).append("\r\n");
        }

        // Header terminator
        sb.append("\r\n");

        out.write(sb.toString().getBytes(StandardCharsets.US_ASCII));

        // Body — written as raw bytes, not through a Writer
        if (resp.getBody().length > 0) {
            out.write(resp.getBody());
        }

        out.flush();
    }
}