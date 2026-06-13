package com.httpserver;

import com.httpserver.handler.Handlers;
import com.httpserver.router.Router;
import com.httpserver.server.HttpServer;
import com.httpserver.server.ServerConfig;

import java.io.IOException;
import java.util.logging.Logger;

public class Main {
    private static final Logger log = Logger.getLogger(Main.class.getName());

    public static void main(String[] args) throws IOException {
        Router router = new Router();
        router.GET("/health", Handlers::health);
        router.GET("/hello", Handlers::hello);
        router.GET("/echo", Handlers::echo);
        router.POST("/echo", Handlers::echo);

        ServerConfig config = ServerConfig.defaultConfig();
        HttpServer server = new HttpServer(config, router);

        // Shutdown hook — equivalent to Go's signal.Notify(sigCh, SIGTERM, SIGINT)
        // JVM registers this with the OS; called on Ctrl+C or SIGTERM
        Runtime.getRuntime().addShutdownHook(new Thread(() -> {
            log.info("Shutdown signal received");
            server.shutdown(15_000); // 15 seconds — same as Go implementation
        }, "shutdown-hook"));

        log.info("Starting server on port " + config.port());
        server.start(); // blocks until shutdown
    }
}