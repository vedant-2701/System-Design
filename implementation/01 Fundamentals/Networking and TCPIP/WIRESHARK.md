# Wireshark Capture Guide — TCP Lab

This guide explains what to capture and **what to look for** at each stage.
The goal is not just to capture packets — it is to see the TCP concepts from
the notes become visible as real bytes on the wire.

---

## Setup

### Install Wireshark
```bash
# Ubuntu/Debian
sudo apt-get install wireshark

# macOS
brew install --cask wireshark
```

### Capture Interface
Use the **loopback interface** (`lo` on Linux, `lo0` on macOS) since both
server and client run locally.

In Wireshark: Capture → Interfaces → select `Loopback: lo`

### Capture Filter (set before capturing)
```
tcp port 9000
```
This filters to only our server's port, eliminating noise.

---

## Stage 1 — TCP Three-Way Handshake

**What to do:** Start capture, then start the server, then start the client.

**What you should see (first 3 packets):**

```
No.  Source         Destination    Protocol  Info
1    127.0.0.1      127.0.0.1      TCP       54321 → 9000 [SYN] Seq=0
2    127.0.0.1      127.0.0.1      TCP       9000 → 54321 [SYN, ACK] Seq=0 Ack=1
3    127.0.0.1      127.0.0.1      TCP       54321 → 9000 [ACK] Seq=1 Ack=1
```

**Click packet 1 (SYN). Expand "Transmission Control Protocol":**
- `Source Port`: ephemeral port assigned by OS (~32768–60999)
- `Destination Port`: 9000
- `Sequence Number`: random ISN (Initial Sequence Number)
- `Flags`: SYN bit set, all others 0
- `Window Size`: receiver buffer advertisement

**Click packet 2 (SYN-ACK). Notice:**
- `Sequence Number`: server's own random ISN (different from client's)
- `Acknowledgment Number`: client's ISN + 1 (confirms receipt of SYN)
- Both SYN and ACK flags set

**Click packet 3 (ACK). Notice:**
- `Acknowledgment Number`: server's ISN + 1
- This is the packet that completes the handshake
- No data yet — purely control

**Key insight to verify:** sequence numbers are random, not starting from 0.
Wireshark displays them as relative (0, 1, 2...) for readability — toggle off
with: Edit → Preferences → Protocols → TCP → uncheck "Relative sequence numbers".

---

## Stage 2 — Length-Prefix Framing on the Wire

**What to do:** Send a message from the client (e.g. "Hello")

**What you should see:**

```
No.  Source    Dest      Protocol  Info
4    client    server    TCP       PSH, ACK Seq=1 Len=9
5    server    client    TCP       ACK Seq=1 Ack=10
6    server    client    TCP       PSH, ACK Seq=1 Len=9
7    client    server    TCP       ACK Seq=10 Ack=10
```

Packet 4 carries `Len=9` because:
- 4 bytes: length header (big-endian uint32 = 5 for "Hello")
- 5 bytes: "Hello" payload
- Total: 9 bytes

**Click packet 4. Expand "Data" in the packet bytes pane:**
```
0000   00 00 00 05 48 65 6c 6c 6f
       ─────────── ───────────────
       length=5    "Hello" in ASCII
```

This is your framing protocol visible at the byte level:
- Bytes 0–3: `00 00 00 05` = big-endian uint32 = 5
- Bytes 4–8: `48 65 6c 6c 6f` = "Hello"

**Apply display filter to isolate data packets:**
```
tcp.port == 9000 && tcp.len > 0
```

---

## Stage 3 — TCP Stream Following

Right-click any packet → Follow → TCP Stream

This shows the raw byte stream reassembled by Wireshark — you see exactly
what your application's `read()` call receives, regardless of how TCP fragmented it.

Red = client → server
Blue = server → client

The length-prefix headers will be visible as non-printable characters followed
by the ASCII payload. This makes the framing protocol tangible.

---

## Stage 4 — Simulate Packet Fragmentation

**What to do:** Send a long message (e.g. 5000 characters) and observe.

With a large enough message, TCP may fragment it across multiple segments.
You will see the `PSH` flag only on the final segment of a message — PSH
tells the receiver "pass this to the application now, don't wait for more."

Look for:
```
No.  Info
10   TCP  [ACK] Seq=1    Len=1460   ← first segment (MSS = 1460 bytes)
11   TCP  [PSH, ACK] Seq=1461 Len=540  ← final segment with PSH
```

This directly demonstrates why a single `read()` call is insufficient —
TCP gives you data as segments arrive, not as application messages.

---

## Stage 5 — TCP Keepalive Probes

**What to do:** Connect a client, send one message, then leave the connection
idle for 30 seconds (the keepAliveInterval we configured).

**Display filter:**
```
tcp.port == 9000 && tcp.analysis.keep_alive
```

You should see periodic probe packets:
```
No.  Info
100  TCP  [ACK] (TCP Keep-Alive)   ← server probing client
101  TCP  [ACK] (TCP Keep-Alive ACK) ← client responding
```

These are 0-byte or 1-byte probes sent by the OS, invisible to your application.
If the client stops responding, after `tcp_keepalive_probes` (default 9) failed
probes the OS closes the socket — your `read()` returns an error.

---

## Stage 6 — Graceful Connection Teardown (FIN Four-Way)

**What to do:** Type Ctrl+C in the client. Watch the last packets.

```
No.  Source    Dest      Info
200  client    server    TCP  [FIN, ACK]   ← client done sending
201  server    client    TCP  [ACK]        ← server acknowledges
202  server    client    TCP  [FIN, ACK]   ← server done sending
203  client    server    TCP  [ACK]        ← client acknowledges
```

After packet 203, the client socket enters TIME_WAIT.
You cannot see TIME_WAIT in Wireshark directly, but you can verify it:

```bash
ss -tan | grep TIME_WAIT
```

---

## Stage 7 — Abrupt Disconnect (RST vs FIN)

**What to do:** Kill the client with `kill -9 <pid>` instead of Ctrl+C.

```
No.  Source    Dest      Info
300  client    server    TCP  [RST, ACK]   ← abrupt close, no FIN
```

Compare with graceful close (FIN) from Stage 6:
- `FIN`: "I'm done sending, but I'll wait for you to finish"
- `RST`: "I'm closing immediately. Discard everything."

Your server's `read()` will receive `ECONNRESET` (Go) / `SocketException "Connection reset"` (Java).

---

## Useful Wireshark Display Filters

```
# All traffic on our port
tcp.port == 9000

# Only data-carrying segments (exclude pure ACKs)
tcp.port == 9000 && tcp.len > 0

# Only control packets (SYN, FIN, RST)
tcp.port == 9000 && (tcp.flags.syn || tcp.flags.fin || tcp.flags.rst)

# Retransmissions (simulated packet loss)
tcp.analysis.retransmission

# Keepalive probes
tcp.analysis.keep_alive

# Window full / zero window (flow control)
tcp.analysis.window_full || tcp.analysis.zero_window

# Duplicate ACKs (precursor to fast retransmit)
tcp.analysis.duplicate_ack
```

---

## What to Look For — Summary

| Concept from Notes | Where to see it in Wireshark |
|-------------------|------------------------------|
| 3-way handshake | First 3 packets: SYN, SYN-ACK, ACK |
| Random ISNs | Sequence numbers in SYN packets (disable relative seq) |
| Length-prefix framing | Data bytes: first 4 bytes = big-endian length header |
| TCP byte stream | Follow TCP Stream — shows reassembled data regardless of segments |
| TCP segmentation | Large message split across multiple packets |
| PSH flag | Set on final segment of a message group |
| Keepalive probes | Idle connection, filter by `tcp.analysis.keep_alive` |
| Graceful close | FIN, ACK, FIN, ACK sequence |
| Abrupt close | RST packet instead of FIN |
| TIME_WAIT | Verify with `ss -tan` after graceful close |
| Flow control | Zero window advertisement in ACK headers |
