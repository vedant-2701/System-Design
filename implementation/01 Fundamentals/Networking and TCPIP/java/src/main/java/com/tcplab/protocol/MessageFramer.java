package com.tcplab.protocol;

import java.io.DataInputStream;
import java.io.DataOutputStream;
import java.io.EOFException;
import java.io.IOException;
import java.nio.charset.StandardCharsets;

/**
 * Length-prefix framing over TCP byte streams.
 *
 * <p>Wire format: [4 bytes big-endian int32: payload length] [N bytes: payload]
 *
 * <p>This is intentionally wire-compatible with the Go implementation.
 * Both use 4-byte big-endian length headers, so Go servers can talk to Java
 * clients and vice versa — critical for polyglot microservice architectures.
 *
 * <p>WHY DataInputStream/DataOutputStream:
 * They provide readFully() which blocks until exactly N bytes are read,
 * handling partial reads internally. This is equivalent to io.ReadFull in Go.
 * A naive InputStream.read() may return fewer bytes than requested.
 *
 * <p>Thread-safety: NOT safe for concurrent use on the same stream.
 * Each connection should have its own MessageFramer instance.
 */
public final class MessageFramer {

    /** Maximum allowed message size — defence against memory exhaustion attacks. */
    public static final int MAX_MESSAGE_SIZE = 1024 * 1024; // 1 MB

    private final DataInputStream in;
    private final DataOutputStream out;

    public MessageFramer(DataInputStream in, DataOutputStream out) {
        this.in = in;
        this.out = out;
    }

    /**
     * Reads one complete framed message from the stream.
     * Blocks until a full message is available.
     *
     * @return the message payload bytes
     * @throws EOFException if the connection was closed cleanly (no more messages)
     * @throws IOException  on network error or oversized message
     */
    public byte[] readMessage() throws IOException {
        // readInt() reads exactly 4 bytes and interprets them as big-endian int32.
        // Throws EOFException if stream is closed before the 4 bytes arrive —
        // this is the clean disconnect signal (equivalent to io.EOF in Go).
        int size;
        try {
            size = in.readInt();
        } catch (EOFException e) {
            throw e; // re-throw: clean disconnect, caller should handle separately
        }

        // Validate before allocating. Without this, a crafted header of 0x7FFFFFFF
        // causes a 2GB allocation attempt before any payload is read.
        if (size < 0 || size > MAX_MESSAGE_SIZE) {
            throw new IOException(
                String.format("Invalid message size %d: must be 0..%d (possible malicious client)",
                    size, MAX_MESSAGE_SIZE)
            );
        }

        byte[] payload = new byte[size];

        // readFully() blocks until exactly 'size' bytes are read.
        // If connection drops mid-payload, throws EOFException (mapped to IOException).
        // This is the critical difference from read() which may return partial data.
        in.readFully(payload);

        return payload;
    }

    /**
     * Writes one complete framed message to the stream.
     * Flushes immediately so the message is not held in the output buffer.
     *
     * @param payload the message bytes to send
     * @throws IllegalArgumentException if payload exceeds MAX_MESSAGE_SIZE
     * @throws IOException on network error
     */
    public void writeMessage(byte[] payload) throws IOException {
        if (payload.length > MAX_MESSAGE_SIZE) {
            throw new IllegalArgumentException(
                String.format("Payload size %d exceeds maximum %d", payload.length, MAX_MESSAGE_SIZE)
            );
        }

        // writeInt() writes exactly 4 bytes, big-endian — matches Go's binary.BigEndian.PutUint32
        out.writeInt(payload.length);
        out.write(payload);

        // Flush is critical: without it, DataOutputStream buffers data internally
        // and the bytes may never reach the network.
        // In production with high throughput, consider buffering multiple messages
        // before flushing to reduce syscall overhead.
        out.flush();
    }

    /** Convenience overload for string messages using UTF-8 encoding. */
    public void writeMessage(String message) throws IOException {
        writeMessage(message.getBytes(StandardCharsets.UTF_8));
    }
}