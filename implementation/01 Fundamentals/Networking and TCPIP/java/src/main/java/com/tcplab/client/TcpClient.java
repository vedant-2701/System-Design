package com.tcplab.client;

import com.tcplab.protocol.MessageFramer;

import java.io.BufferedReader;
import java.io.DataInputStream;
import java.io.DataOutputStream;
import java.io.EOFException;
import java.io.IOException;
import java.io.InputStreamReader;
import java.net.InetSocketAddress;
import java.net.Socket;
import java.nio.charset.StandardCharsets;
import java.time.Duration;
import java.util.logging.Level;
import java.util.logging.Logger;

/**
 * TCP echo client with exponential backoff reconnection.
 */
public class TcpClient {

    private static final Logger LOG = Logger.getLogger(TcpClient.class.getName());

    private static final String SERVER_HOST = "127.0.0.1";
    private static final int SERVER_PORT = 9000;
    private static final int CONNECT_TIMEOUT_MS = 5_000;
    private static final int READ_TIMEOUT_MS = 15_000;
    private static final int MAX_RETRIES = 10;
    private static final Duration MIN_RETRY_DELAY = Duration.ofMillis(500);
    private static final Duration MAX_RETRY_DELAY = Duration.ofSeconds(30);

    private final String host;
    private final int port;

    public TcpClient(String host, int port) {
        this.host = host;
        this.port = port;
    }

    /**
     * Connects to the server with exponential backoff.
     * Each retry doubles the delay, capped at MAX_RETRY_DELAY.
     */
    private Socket connect() throws IOException, InterruptedException {
        int attempt = 0;
        while (attempt < MAX_RETRIES) {
            attempt++;
            LOG.info(String.format("Connecting to %s:%d (attempt %d)", host, port, attempt));

            try {
                Socket socket = new Socket();
                socket.connect(new InetSocketAddress(host, port), CONNECT_TIMEOUT_MS);
                socket.setSoTimeout(READ_TIMEOUT_MS);
                socket.setTcpNoDelay(true);
                socket.setKeepAlive(true);

                LOG.info(String.format("Connected to %s:%d", host, port));
                return socket;

            } catch (IOException e) {
                LOG.warning(String.format("Connection attempt %d failed: %s", attempt, e.getMessage()));

                if (attempt >= MAX_RETRIES) {
                    throw new IOException("Max retries exceeded", e);
                }

                // Exponential backoff: delay = min(minDelay * 2^(attempt-1), maxDelay)
                // Jitter: add up to 25% random variance to avoid retry storms
                long baseMs = Math.min(
                    MIN_RETRY_DELAY.toMillis() * (1L << (attempt - 1)),
                    MAX_RETRY_DELAY.toMillis()
                );
                long jitterMs = (long) (Math.random() * baseMs * 0.25);
                long delayMs = baseMs + jitterMs;

                LOG.info(String.format("Retrying in %dms", delayMs));
                Thread.sleep(delayMs);
            }
        }
        throw new IOException("Failed to connect after " + MAX_RETRIES + " attempts");
    }

    public void run() throws IOException, InterruptedException {
        Socket socket = connect();

        try (socket;
             var reader = new BufferedReader(new InputStreamReader(System.in))) {

            var framer = new MessageFramer(
                new DataInputStream(socket.getInputStream()),
                new DataOutputStream(socket.getOutputStream())
            );

            System.out.println("Connected. Type messages and press Enter. Ctrl+C to quit.");

            String line;
            while ((line = reader.readLine()) != null) {
                if (line.isBlank()) continue;

                long startNs = System.nanoTime();

                framer.writeMessage(line);

                byte[] response = framer.readMessage();
                long rttMs = Duration.ofNanos(System.nanoTime() - startNs).toMillis();

                System.out.printf("Echo [%s] rtt=%dms%n",
                    new String(response, StandardCharsets.UTF_8), rttMs);

                LOG.info(String.format("Round trip complete: sent=%d bytes, rtt=%dms",
                    line.length(), rttMs));
            }

        } catch (EOFException e) {
            LOG.info("Server closed the connection");
        } catch (IOException e) {
            LOG.log(Level.SEVERE, "Connection error", e);
            throw e;
        }
    }

    public static void main(String[] args) throws IOException, InterruptedException {
        new TcpClient(SERVER_HOST, SERVER_PORT).run();
    }
}