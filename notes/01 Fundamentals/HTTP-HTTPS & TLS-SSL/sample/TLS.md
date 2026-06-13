# The Core Problem

You connect to `https://api.example.com`. The server sends you a certificate saying "I am api.example.com." 

**How do you know this certificate isn't forged?**

An attacker performing a man-in-the-middle attack could intercept your connection and present their own certificate saying "I am api.example.com." If you just trust whatever certificate you receive, encryption is useless — you'd be encrypting traffic to the attacker.

You need a way to verify the certificate's authenticity without prior knowledge of the server.

---

## The Solution — Certificate Authorities

The internet solves this with a **web of trust** built on Certificate Authorities.

**Core idea**: There exist a small number of organizations — Certificate Authorities (CAs) — that are universally trusted. Your operating system and browser ship with a list of ~150 trusted root CAs pre-installed. Mozilla, Microsoft, Apple, Google all maintain these lists.

When a CA vouches for a certificate, you trust it — because you already trust the CA.

---

## Certificate Chain — How Trust Works

Trust doesn't flow directly from root CA to every website. It flows through a chain.

```
Root CA (DigiCert Global Root)
    ↓ signs
Intermediate CA (DigiCert TLS RSA SHA256 2020)
    ↓ signs
End-Entity Certificate (api.example.com)
```

**Why intermediates exist**: Root CA private keys are extraordinarily valuable. If compromised, millions of certificates become untrusted. Root CAs keep their private keys offline — literally in hardware security modules in physically secured facilities. They use intermediate CAs for day-to-day signing. If an intermediate is compromised, it can be revoked without touching the root.

---

## What's Inside a Certificate

A TLS certificate is a structured document containing:

```
Subject:          api.example.com
Issuer:           DigiCert TLS RSA SHA256 2020 CA
Valid From:       2026-01-01
Valid Until:      2027-01-01
Public Key:       <server's public key>
SANs:             api.example.com, *.example.com
Signature:        <CA's digital signature over all the above>
```

**Subject Alternative Names (SANs)** — list of domains this certificate is valid for. Wildcard certs (`*.example.com`) cover all subdomains.

**The signature is the critical part.** The CA signs the certificate using its private key. Anyone with the CA's public key can verify this signature. Since you already trust the CA, and the CA's public key is in your trust store, you can verify any certificate the CA signed.

---

## Certificate Verification — Step by Step

When your browser receives `api.example.com`'s certificate during TLS handshake:

**Step 1 — Build the chain**

Server sends its certificate plus intermediate CA certificate. Browser finds the root CA in its local trust store.

```
api.example.com cert → signed by → Intermediate CA cert → signed by → Root CA (trusted locally)
```

**Step 2 — Verify each signature**

Browser verifies intermediate CA's signature on the end-entity cert. Then verifies root CA's signature on the intermediate cert. Cryptographic verification — not a network call.

**Step 3 — Check validity period**

Is today between Valid From and Valid Until? Expired certificates are rejected.

**Step 4 — Check domain matches**

Does `api.example.com` appear in the Subject or SANs? Prevents a certificate issued for `evil.com` from being used for `api.example.com`.

**Step 5 — Check revocation**

Has this certificate been revoked? Two mechanisms:

- **CRL (Certificate Revocation List)** — CA publishes a list of revoked serial numbers. Browser downloads and checks. Problem: CRLs can be large and become stale.
- **OCSP (Online Certificate Status Protocol)** — browser queries CA's OCSP server in real time: "is certificate #abc123 still valid?" Problem: adds latency, privacy concern (CA learns which sites you visit), OCSP server downtime breaks TLS.
- **OCSP Stapling** — server queries OCSP itself, attaches the signed response to the TLS handshake. Client doesn't need to contact CA. Eliminates latency and privacy concerns. **This is the production solution.**

**Step 6 — All checks pass**

Browser extracts server's public key from certificate. Uses it for key exchange. Connection proceeds.

---

## The TLS Handshake in Full Detail

Now you have enough context to understand every step precisely.

**TLS 1.3 Handshake:**

```
Client                                    Server
  |                                          |
  |----ClientHello-------------------------->|
  |    - TLS version: 1.3                   |
  |    - Client random (32 bytes)           |
  |    - Supported cipher suites            |
  |    - Key share (ECDHE public key)       |
  |                                          |
  |<---ServerHello---------------------------|
  |    - Chosen cipher suite                |
  |    - Server random (32 bytes)           |
  |    - Key share (ECDHE public key)       |
  |                                          |
  |<---Certificate---------------------------|
  |    - Server cert + intermediate cert    |
  |                                          |
  |<---CertificateVerify---------------------|
  |    - Signature proving server owns      |
  |      the private key                    |
  |                                          |
  |<---Finished------------------------------|
  |    - MAC of entire handshake            |
  |                                          |
  |----Finished----------------------------->|
  |                                          |
  |====Encrypted HTTP begins================|
```

**What's happening cryptographically:**

Both sides have each other's ECDHE public keys. Using Elliptic Curve Diffie-Hellman, both independently compute the **same shared secret** without ever transmitting it. An eavesdropper who sees both public keys cannot derive the shared secret — that's the mathematical guarantee of DH.

From the shared secret, both sides derive symmetric session keys for encryption (AES-GCM typically) and MAC keys for integrity verification.

**CertificateVerify** — server signs the entire handshake transcript with its private key. Client verifies this signature using the public key from the certificate. This proves the server actually possesses the private key corresponding to the certificate — not just that it has a copy of the certificate.

---

## Why This Is Secure

**Confidentiality** — session keys derived from ECDHE. Never transmitted. Eavesdropper sees only public keys, cannot derive session key.

**Forward Secrecy** — ECDHE generates ephemeral key pairs per session. Even if server's long-term private key is compromised later, past session keys cannot be derived. Contrast with RSA key exchange where the session key is encrypted with the server's long-term key — compromise the key, decrypt all past traffic.

**Authentication** — certificate chain proves server identity. CertificateVerify proves key possession.

**Integrity** — AES-GCM is an authenticated encryption mode. Any tampering with ciphertext is detected.

---

## Certificate Issuance — How You Get a Certificate

**Domain Validation (DV)** — CA verifies you control the domain. Automated via ACME protocol (Let's Encrypt). Takes seconds. Proves domain control only — not organizational identity.

**Organization Validation (OV)** — CA verifies your organization exists. Manual process, days.

**Extended Validation (EV)** — strict organizational verification. Used to show green bar in browsers. Mostly deprecated now — browsers removed visual distinction.

**Let's Encrypt** changed the industry — free, automated, 90-day certificates with auto-renewal. No reason for paid DV certs anymore for most use cases. 90-day expiry is intentional — forces automation, limits compromise window.

---