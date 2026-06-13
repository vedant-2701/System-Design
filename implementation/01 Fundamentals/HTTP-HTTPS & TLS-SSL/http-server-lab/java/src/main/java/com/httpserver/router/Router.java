package com.httpserver.router;

import com.httpserver.parser.HttpRequest;
import com.httpserver.parser.HttpResponse;

import java.util.Map;
import java.util.concurrent.ConcurrentHashMap;
import java.util.function.Function;

/**
 * Routes (method, path) pairs to handler functions.
 *
 * Java conceptual equivalent of Go's router.Router.
 *
 * Key differences from Go:
 * - HandlerFunc is java.util.function.Function<HttpRequest, HttpResponse>
 *   (Go uses a named function type; Java uses a functional interface)
 * - ConcurrentHashMap: registration happens at startup (single-threaded)
 *   but reads happen under concurrent request load — ConcurrentHashMap is safe for this.
 *   Go's map is safe in the same way because it's written before goroutines start.
 */
public class Router {

    @FunctionalInterface
    public interface HandlerFunc extends Function<HttpRequest, HttpResponse> {}

    // key: "METHOD /path"
    private final ConcurrentHashMap<String, HandlerFunc> routes = new ConcurrentHashMap<>();
    // key: "/path" — for 405 detection
    private final ConcurrentHashMap<String, Boolean> paths = new ConcurrentHashMap<>();

    /**
     * Registers a handler for a specific method+path pair.
     * Throws IllegalStateException on duplicate — fail fast at startup.
     */
    public void register(String method, String path, HandlerFunc handler) {
        String key = method + " " + path;
        if (routes.putIfAbsent(key, handler) != null) {
            throw new IllegalStateException("Duplicate route registration: " + key);
        }
        paths.put(path, true);
    }

    public void GET(String path, HandlerFunc handler)    { register("GET", path, handler); }
    public void POST(String path, HandlerFunc handler)   { register("POST", path, handler); }
    public void PUT(String path, HandlerFunc handler)    { register("PUT", path, handler); }
    public void DELETE(String path, HandlerFunc handler) { register("DELETE", path, handler); }

    /**
     * Result of a route lookup.
     * Using a sealed interface models the three outcomes cleanly:
     * Found, NotFound, MethodNotAllowed — exhaustive pattern matching in callers.
     *
     * Java 17+ sealed interfaces are the idiomatic way to express what Go expresses
     * with multiple return values + nil checks.
     */
    public sealed interface LookupResult permits
            LookupResult.Found,
            LookupResult.NotFound,
            LookupResult.MethodNotAllowed {

        record Found(HandlerFunc handler) implements LookupResult {}
        record NotFound() implements LookupResult {}
        record MethodNotAllowed() implements LookupResult {}
    }

    public LookupResult lookup(String method, String path) {
        String key = method + " " + path;
        HandlerFunc handler = routes.get(key);

        if (handler != null) {
            return new LookupResult.Found(handler);
        }
        if (paths.containsKey(path)) {
            return new LookupResult.MethodNotAllowed();
        }
        return new LookupResult.NotFound();
    }
}