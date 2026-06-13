# Question

> Here's a scenario: You open a browser and type https://api.example.com/users/123.
> Walk me through everything that happens — from that URL being entered to the response being rendered.

---

# Answer

Good skeleton. You have the right sequence instinct. Let me now build the complete, precise picture — filling every gap.

---

## The Complete HTTPS Request Lifecycle

---

### Step 1 — URL Parsing

Browser parses:
- **Scheme**: `https` → use TLS, default port 443
- **Host**: `api.example.com`
- **Path**: `/users/123`
- **No query string, no fragment here**

This tells the browser: I need a secure connection to `api.example.com` on port 443, and I want resource `/users/123`.

---

### Step 2 — DNS Resolution

Browser needs an IP address. It checks in order:

1. **Browser DNS cache** — recent lookups cached here
2. **OS DNS cache** — system-level cache (`/etc/hosts` checked here too)
3. **Router cache** — your local router may have it
4. **Recursive resolver** — your ISP or configured DNS (8.8.8.8 etc.) does the heavy lifting
5. **Authoritative resolution** — root → TLD (`.com`) → `example.com` nameserver → returns IP

Result: `api.example.com` → `93.184.216.34` (example). Cached with TTL.

**Gap in your answer**: DNS happens *before* TCP. You had it right but didn't mention that DNS itself uses UDP port 53 — it's a separate network transaction that must complete before anything else.

---

### Step 3 — TCP Connection Establishment

Now the browser knows the IP. It opens a TCP connection to `93.184.216.34:443`.

```
Client          Server
  |---SYN-------->|
  |<--SYN-ACK-----|
  |---ACK-------->|
```

**One full round trip.** Connection is now established. But no data sent yet.

---

### Step 4 — TLS Handshake

This is where your answer had a gap. TLS negotiation happens *after* TCP, *before* any HTTP data. This is a separate process.

**TLS 1.3 handshake** (modern, what you'll see in production):

```
Client                          Server
  |---ClientHello---------------->|   (supported cipher suites, TLS version, client random, key share)
  |<--ServerHello-----------------|   (chosen cipher suite, server random, key share)
  |<--Certificate-----------------|   (server's TLS certificate)
  |<--CertificateVerify-----------|   (proof server owns the private key)
  |<--Finished--------------------|
  |---Finished------------------->|
  |===Encrypted HTTP begins=======|
```

**One round trip for TLS 1.3.** So total before first HTTP byte: 1 RTT TCP + 1 RTT TLS = **2 RTTs**.

TLS 1.2 was 2 RTTs for TLS alone — 3 RTTs total. This is why TLS 1.3 was a significant improvement.

**What's happening during the TLS handshake:**

1. Client and server **agree on cipher suite** — which encryption algorithm to use
2. Server sends its **certificate** — contains its public key and identity
3. Client **verifies the certificate** — checks it was signed by a trusted CA, not expired, matches the domain
4. They perform a **key exchange** (ECDHE typically) — both sides derive the same symmetric session key without ever transmitting it
5. From this point, all communication is encrypted with that symmetric key

**Why symmetric key after asymmetric negotiation?** Asymmetric encryption (RSA, ECDH) is computationally expensive. You use it once to establish a shared secret, then switch to fast symmetric encryption (AES) for the actual data.

We'll go deep on certificates and CA chain separately. For now, continue.

---

### Step 5 — HTTP Request Construction and Transmission

Now the browser constructs the HTTP request:

```
GET /users/123 HTTP/1.1
Host: api.example.com
Accept: application/json
Accept-Encoding: gzip, br
Connection: keep-alive
Cookie: session=abc123
User-Agent: Mozilla/5.0...
```

This plaintext is handed to TLS, which encrypts it. The encrypted bytes are handed to TCP, which breaks it into segments. TCP segments are wrapped in IP packets with source/destination IPs. IP packets are wrapped in Ethernet frames with MAC addresses.

**Hop-by-hop routing** — you had this right. Each router unwraps to IP layer, reads destination IP, decrements TTL, rewraps in a new Ethernet frame with the next hop's MAC address, forwards. If TTL hits zero, packet is dropped and ICMP "Time Exceeded" is sent back — this is how `traceroute` works.

---

### Step 6 — Server Receives and Processes

At the server, the stack goes bottom-up:

- Physical → bits
- Ethernet frame → unwrap → IP packet
- IP packet → unwrap → TCP segment
- TCP → reassemble segments in order → byte stream
- TLS → decrypt → plaintext HTTP request
- HTTP server → parse method, path, headers → route to handler
- Handler executes business logic, queries database, builds response

---

### Step 7 — HTTP Response

Server constructs:

```
HTTP/1.1 200 OK
Content-Type: application/json
Content-Encoding: gzip
Cache-Control: max-age=3600
Set-Cookie: session=abc123; HttpOnly; Secure
Content-Length: 284

{"id": 123, "name": "Vedant"...}
```

This goes back down the stack, TLS encrypted, TCP segmented, IP routed, back to client.

---

### Step 8 — Client Processes Response

Client TCP reassembles segments. TLS decrypts. Browser parses HTTP response — status code, headers, body. Renders or processes accordingly.

---

### The RTT Summary

```
DNS lookup:        1 RTT  (UDP)
TCP handshake:     1 RTT
TLS 1.3:           1 RTT
HTTP request:      1 RTT
─────────────────────────
Total:             4 RTTs minimum on first request to new server
```

QUIC/HTTP3 collapses TCP+TLS into 1 RTT. On repeat connections, 0-RTT is possible — data sent with the very first packet.

---