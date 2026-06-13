package com.httpserver.handler;

import com.httpserver.parser.HttpRequest;
import com.httpserver.parser.HttpResponse;

import java.nio.charset.StandardCharsets;
import java.util.Map;

/**
 * Sample request handlers.
 * Static methods — stateless, no shared mutable state, trivially thread-safe.
 *
 * Java vs Go: Go uses package-level functions. Java groups them in a class.
 * The design intent is identical — pure functions from Request → Response.
 */
public final class Handlers {

    private Handlers() {}

    public static HttpResponse health(HttpRequest req) {
        String body = "{\"status\":\"ok\"}";
        return HttpResponse.ok(body, "application/json");
    }

    public static HttpResponse hello(HttpRequest req) {
        String body = "Hello from scratch-built Java HTTP server! You called "
                + req.getMethod() + " " + req.getPath() + "\n";
        return HttpResponse.ok(body, "text/plain");
    }

    public static HttpResponse echo(HttpRequest req) {
        StringBuilder sb = new StringBuilder();
        sb.append("{");
        sb.append("\"method\":\"").append(req.getMethod()).append("\",");
        sb.append("\"path\":\"").append(req.getPath()).append("\",");
        sb.append("\"remote\":\"").append(req.getRemoteAddr()).append("\",");
        sb.append("\"body\":\"").append(new String(req.getBody(), StandardCharsets.UTF_8)).append("\"");
        sb.append("}");
        return HttpResponse.ok(sb.toString(), "application/json");
    }
}