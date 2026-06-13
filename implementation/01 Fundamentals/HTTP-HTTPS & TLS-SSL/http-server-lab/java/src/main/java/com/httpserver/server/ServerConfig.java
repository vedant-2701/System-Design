package com.httpserver.server;

import java.time.Duration;

/**
 * Server configuration. Immutable record — all fields set at construction.
 *
 * Java 16+ records eliminate boilerplate for pure data containers.
 * Equivalent to Go's Config struct but with compiler-generated accessors.
 */
public record ServerConfig(
        String host,
        int port,
        Duration readTimeout,
        Duration writeTimeout,
        Duration idleTimeout,
        int maxConnections
) {
    public static ServerConfig defaultConfig() {
        return new ServerConfig(
                "0.0.0.0",
                8080,
                Duration.ofSeconds(10),
                Duration.ofSeconds(10),
                Duration.ofSeconds(60),
                1000
        );
    }

    public static ServerConfig withPort(int port) {
        return new ServerConfig(
                "0.0.0.0",
                port,
                Duration.ofSeconds(10),
                Duration.ofSeconds(10),
                Duration.ofSeconds(60),
                1000
        );
    }
}