package com.httpserver.parser;

import java.util.Collections;
import java.util.HashMap;
import java.util.Map;

/**
 * Parsed HTTP/1.1 request.
 *
 * Immutable after construction — safe to pass across threads without synchronization.
 * Java conceptual equivalent of Go's parser.Request struct.
 */
public final class HttpRequest {
    private final String method;
    private final String path;
    private final String version;
    private final Map<String, String> headers; // lowercase keys
    private final byte[] body;
    private final String remoteAddr;

    public HttpRequest(String method, String path, String version,
                       Map<String, String> headers, byte[] body, String remoteAddr) {
        this.method = method;
        this.path = path;
        this.version = version;
        // Defensive copy — caller must not mutate the map after construction
        this.headers = Collections.unmodifiableMap(new HashMap<>(headers));
        this.body = body == null ? new byte[0] : body.clone();
        this.remoteAddr = remoteAddr;
    }

    public String getMethod()     { return method; }
    public String getPath()       { return path; }
    public String getVersion()    { return version; }
    public Map<String, String> getHeaders() { return headers; }
    public byte[] getBody()       { return body.clone(); } // defensive copy on read
    public String getRemoteAddr() { return remoteAddr; }

    public String getHeader(String name) {
        return headers.getOrDefault(name.toLowerCase(), null);
    }
}