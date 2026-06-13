package com.tcplab.protocol;

import org.junit.jupiter.api.Test;
import org.junit.jupiter.params.ParameterizedTest;
import org.junit.jupiter.params.provider.ValueSource;

import java.io.*;
import java.nio.ByteBuffer;
import java.nio.charset.StandardCharsets;

import static org.junit.jupiter.api.Assertions.*;

class MessageFramerTest {

    // Helper: create a MessageFramer backed by an in-memory byte array.
    private static class MemoryPipe {
        final ByteArrayOutputStream buffer = new ByteArrayOutputStream();
        final DataOutputStream out;
        MessageFramer writerFramer;

        MemoryPipe() throws IOException {
            out = new DataOutputStream(buffer);
            // Writer framer writes to buffer
            writerFramer = new MessageFramer(
                new DataInputStream(new ByteArrayInputStream(new byte[0])),
                out
            );
        }

        MessageFramer readerFramer() {
            byte[] data = buffer.toByteArray();
            return new MessageFramer(
                new DataInputStream(new ByteArrayInputStream(data)),
                new DataOutputStream(OutputStream.nullOutputStream())
            );
        }
    }

    @Test
    void roundtrip_simple_message() throws IOException {
        var pipe = new MemoryPipe();
        pipe.writerFramer.writeMessage("Hello, World!");
        byte[] got = pipe.readerFramer().readMessage();
        assertEquals("Hello, World!", new String(got, StandardCharsets.UTF_8));
    }

    @Test
    void roundtrip_empty_message() throws IOException {
        var pipe = new MemoryPipe();
        pipe.writerFramer.writeMessage(new byte[0]);
        byte[] got = pipe.readerFramer().readMessage();
        assertEquals(0, got.length);
    }

    @Test
    void roundtrip_unicode() throws IOException {
        String msg = "こんにちは世界";
        var pipe = new MemoryPipe();
        pipe.writerFramer.writeMessage(msg);
        byte[] got = pipe.readerFramer().readMessage();
        assertEquals(msg, new String(got, StandardCharsets.UTF_8));
    }

    @Test
    void multiple_messages_in_stream_read_independently() throws IOException {
        String[] messages = {"first", "second", "third"};

        var buffer = new ByteArrayOutputStream();
        var out = new DataOutputStream(buffer);
        var writer = new MessageFramer(new DataInputStream(InputStream.nullInputStream()), out);

        for (String m : messages) {
            writer.writeMessage(m);
        }

        var reader = new MessageFramer(
            new DataInputStream(new ByteArrayInputStream(buffer.toByteArray())),
            new DataOutputStream(OutputStream.nullOutputStream())
        );

        for (String expected : messages) {
            byte[] got = reader.readMessage();
            assertEquals(expected, new String(got, StandardCharsets.UTF_8));
        }
    }

    @Test
    void read_from_empty_stream_throws_EOFException() {
        var reader = new MessageFramer(
            new DataInputStream(new ByteArrayInputStream(new byte[0])),
            new DataOutputStream(OutputStream.nullOutputStream())
        );
        assertThrows(EOFException.class, reader::readMessage);
    }

    @Test
    void read_partial_header_throws_EOFException() {
        // Only 2 bytes of the 4-byte header
        var reader = new MessageFramer(
            new DataInputStream(new ByteArrayInputStream(new byte[]{0x00, 0x01})),
            new DataOutputStream(OutputStream.nullOutputStream())
        );
        assertThrows(EOFException.class, reader::readMessage);
    }

    @Test
    void oversized_message_header_rejected() {
        // Header claims 2MB payload (> 1MB limit)
        int oversizeBytes = MessageFramer.MAX_MESSAGE_SIZE + 1;
        byte[] header = ByteBuffer.allocate(4).putInt(oversizeBytes).array();

        var reader = new MessageFramer(
            new DataInputStream(new ByteArrayInputStream(header)),
            new DataOutputStream(OutputStream.nullOutputStream())
        );

        IOException ex = assertThrows(IOException.class, reader::readMessage);
        assertTrue(ex.getMessage().contains("Invalid message size"),
            "Error should mention invalid size: " + ex.getMessage());
    }

    @Test
    void negative_size_in_header_rejected() {
        // Negative int in header (e.g. 0xFFFFFFFF = -1 when interpreted as signed int)
        byte[] header = ByteBuffer.allocate(4).putInt(-1).array();
        var reader = new MessageFramer(
            new DataInputStream(new ByteArrayInputStream(header)),
            new DataOutputStream(OutputStream.nullOutputStream())
        );
        IOException ex = assertThrows(IOException.class, reader::readMessage);
        assertTrue(ex.getMessage().contains("Invalid message size"));
    }

    @Test
    void write_oversized_payload_throws_IllegalArgumentException() {
        byte[] oversized = new byte[MessageFramer.MAX_MESSAGE_SIZE + 1];
        var writer = new MessageFramer(
            new DataInputStream(InputStream.nullInputStream()),
            new DataOutputStream(OutputStream.nullOutputStream())
        );
        assertThrows(IllegalArgumentException.class, () -> writer.writeMessage(oversized));
    }

    @Test
    void truncated_payload_throws_EOFException() throws IOException {
        // Header says 100 bytes, but only 10 bytes follow
        var buffer = new ByteArrayOutputStream();
        var dos = new DataOutputStream(buffer);
        dos.writeInt(100);            // claim 100 bytes
        dos.write(new byte[10]);     // only provide 10
        dos.flush();

        var reader = new MessageFramer(
            new DataInputStream(new ByteArrayInputStream(buffer.toByteArray())),
            new DataOutputStream(OutputStream.nullOutputStream())
        );
        assertThrows(EOFException.class, reader::readMessage);
    }

    @ParameterizedTest
    @ValueSource(ints = {1, 100, 1024, 65536, 1024 * 1024})
    void roundtrip_various_payload_sizes(int size) throws IOException {
        byte[] payload = new byte[size];
        for (int i = 0; i < size; i++) payload[i] = (byte) (i % 256);

        var buffer = new ByteArrayOutputStream();
        var writer = new MessageFramer(
            new DataInputStream(InputStream.nullInputStream()),
            new DataOutputStream(buffer)
        );
        writer.writeMessage(payload);

        var reader = new MessageFramer(
            new DataInputStream(new ByteArrayInputStream(buffer.toByteArray())),
            new DataOutputStream(OutputStream.nullOutputStream())
        );
        byte[] got = reader.readMessage();

        assertArrayEquals(payload, got, "Payload mismatch for size " + size);
    }
}