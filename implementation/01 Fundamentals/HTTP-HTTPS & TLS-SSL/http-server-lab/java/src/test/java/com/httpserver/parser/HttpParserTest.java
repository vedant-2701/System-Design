package com.httpserver.parser;

import org.junit.jupiter.api.Test;
import org.junit.jupiter.params.ParameterizedTest;
import org.junit.jupiter.params.provider.ValueSource;

import java.io.*;
import java.nio.charset.StandardCharsets;

import static org.junit.jupiter.api.Assertions.*;

class HttpParserTest {

    // Helper: wraps a raw HTTP request string into the two stream types
    // the parser requires. Mirrors Go's requestReader() helper.
    private record Streams(BufferedReader reader, BufferedInputStream raw) {}

    private Streams streamsFor(String raw) {
        byte[] bytes = raw.getBytes(StandardCharsets.ISO_8859_1);
        ByteArrayInputStream bais = new ByteArrayInputStream(bytes);
        BufferedInputStream rawStream = new BufferedInputStream(bais);
        BufferedReader reader = new BufferedReader(
                new InputStreamReader(new ByteArrayInputStream(bytes), StandardCharsets.ISO_8859_1));
        // Note: both reader and rawStream wrap the SAME byte array independently here
        // (safe for testing; in production they share the socket stream).
        return new Streams(reader, rawStream);
    }

    // More accurate helper that shares the underlying stream
    private record TestStreams(BufferedReader reader, BufferedInputStream raw) {}

    private TestStreams sharedStreams(String rawRequest) {
        byte[] bytes = rawRequest.getBytes(StandardCharsets.ISO_8859_1);
        BufferedInputStream bis = new BufferedInputStream(new ByteArrayInputStream(bytes));
        BufferedReader reader = new BufferedReader(new InputStreamReader(bis, StandardCharsets.ISO_8859_1));
        return new TestStreams(reader, bis);
    }

    // --- Happy path tests ---

    @Test
    void parseRequest_simpleGET() throws Exception {
        String raw = "GET /hello HTTP/1.1\r\nHost: localhost\r\n\r\n";
        var s = sharedStreams(raw);

        HttpRequest req = HttpParser.parseRequest(s.reader(), s.raw(), "127.0.0.1");

        assertEquals("GET",      req.getMethod());
        assertEquals("/hello",   req.getPath());
        assertEquals("HTTP/1.1", req.getVersion());
        assertEquals("localhost", req.getHeader("host"));
        assertEquals(0, req.getBody().length);
    }

    @Test
    void parseRequest_headerCaseNormalization() throws Exception {
        // RFC 7230: header names are case-insensitive — must normalize to lowercase
        String raw = "GET / HTTP/1.1\r\nCONTENT-TYPE: text/plain\r\nX-Custom: Value\r\n\r\n";
        var s = sharedStreams(raw);

        HttpRequest req = HttpParser.parseRequest(s.reader(), s.raw(), "127.0.0.1");

        assertEquals("text/plain", req.getHeader("content-type"),
                "Header lookup should be case-insensitive");
        assertEquals("Value", req.getHeader("x-custom"));
    }

    @Test
    void parseRequest_POSTWithBody() throws Exception {
        String body = "{\"name\":\"vedant\"}";
        String raw = "POST /users HTTP/1.1\r\n" +
                "Host: localhost\r\n" +
                "Content-Type: application/json\r\n" +
                "Content-Length: " + body.length() + "\r\n" +
                "\r\n" +
                body;
        var s = sharedStreams(raw);

        HttpRequest req = HttpParser.parseRequest(s.reader(), s.raw(), "127.0.0.1");

        assertEquals("POST", req.getMethod());
        assertEquals(body, new String(req.getBody(), StandardCharsets.UTF_8));
    }

    // --- Malformed request tests ---

    @Test
    void parseRequest_malformedRequestLine_throwsParseException() {
        String raw = "NOT VALID\r\n\r\n";
        var s = sharedStreams(raw);

        assertThrows(HttpParseException.class,
                () -> HttpParser.parseRequest(s.reader(), s.raw(), "127.0.0.1"));
    }

    @ParameterizedTest
    @ValueSource(strings = {"BREW", "CONNECT", "TRACE", "FOOBAR"})
    void parseRequest_unsupportedMethod_throwsParseException(String method) {
        String raw = method + " /path HTTP/1.1\r\nHost: localhost\r\n\r\n";
        var s = sharedStreams(raw);

        assertThrows(HttpParseException.class,
                () -> HttpParser.parseRequest(s.reader(), s.raw(), "127.0.0.1"),
                "Expected parse exception for method: " + method);
    }

    @Test
    void parseRequest_emptyLine_throwsEOF() {
        var s = sharedStreams("");
        assertThrows(EOFException.class,
                () -> HttpParser.parseRequest(s.reader(), s.raw(), "127.0.0.1"));
    }

    @Test
    void parseRequest_invalidContentLength_nonNumeric() {
        String raw = "POST /data HTTP/1.1\r\nContent-Length: abc\r\n\r\n";
        var s = sharedStreams(raw);

        assertThrows(HttpParseException.class,
                () -> HttpParser.parseRequest(s.reader(), s.raw(), "127.0.0.1"));
    }

    @Test
    void parseRequest_negativeContentLength_throws() {
        String raw = "POST /data HTTP/1.1\r\nContent-Length: -1\r\n\r\n";
        var s = sharedStreams(raw);

        assertThrows(HttpParseException.class,
                () -> HttpParser.parseRequest(s.reader(), s.raw(), "127.0.0.1"));
    }

    @Test
    void parseRequest_bodyExceedsMaxSize_throws() {
        String raw = "POST /upload HTTP/1.1\r\nContent-Length: 999999999\r\n\r\n";
        var s = sharedStreams(raw);

        assertThrows(HttpParseException.class,
                () -> HttpParser.parseRequest(s.reader(), s.raw(), "127.0.0.1"));
    }

    @Test
    void parseRequest_malformedHeader_noColon() {
        String raw = "GET / HTTP/1.1\r\nInvalidHeaderWithoutColon\r\n\r\n";
        var s = sharedStreams(raw);

        assertThrows(HttpParseException.class,
                () -> HttpParser.parseRequest(s.reader(), s.raw(), "127.0.0.1"));
    }

    @Test
    void parseRequest_tooManyHeaders_throws() throws Exception {
        StringBuilder sb = new StringBuilder("GET / HTTP/1.1\r\n");
        for (int i = 0; i <= HttpParser.MAX_HEADER_COUNT; i++) {
            // Unique header names — map deduplication must not hide the count
            sb.append("X-Header-").append(i).append(": value\r\n");
        }
        sb.append("\r\n");
        var s = sharedStreams(sb.toString());

        assertThrows(HttpParseException.class,
                () -> HttpParser.parseRequest(s.reader(), s.raw(), "127.0.0.1"),
                "Expected exception for too many headers");
    }

    // --- Serialization tests ---

    @Test
    void serializeResponse_statusLineAndContentLength() throws Exception {
        HttpResponse resp = HttpResponse.ok("hello", "text/plain");
        ByteArrayOutputStream out = new ByteArrayOutputStream();

        HttpParser.serializeResponse(out, resp);

        String result = out.toString(StandardCharsets.US_ASCII);
        assertTrue(result.startsWith("HTTP/1.1 200 OK\r\n"),
                "Expected status line, got: " + result.substring(0, Math.min(50, result.length())));
        assertTrue(result.contains("Content-Length: 5"),
                "Expected Content-Length header");
        assertTrue(result.contains("\r\n\r\n"),
                "Expected blank line separating headers from body");
        assertTrue(result.endsWith("hello"),
                "Expected body at end");
    }

    @Test
    void serializeResponse_emptyBody_contentLengthZero() throws Exception {
        HttpResponse resp = new HttpResponse(204, "No Content",
                new java.util.HashMap<>(), new byte[0]);
        ByteArrayOutputStream out = new ByteArrayOutputStream();

        HttpParser.serializeResponse(out, resp);

        String result = out.toString(StandardCharsets.US_ASCII);
        assertTrue(result.contains("Content-Length: 0"),
                "Expected Content-Length: 0 for empty body");
    }
}