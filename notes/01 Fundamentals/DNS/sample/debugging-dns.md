# Question

> You're the engineer on-call. At 2am, you get an alert: 30% of users in Europe cannot reach api.myservice.com. Users in the US and Asia are fine. Your servers in Frankfurt are healthy — you can curl them directly by IP and they respond correctly. DNS records on your authoritative nameserver look correct.
> What do you suspect? Walk me through your debugging process step by step. What would you check, in what order, and what tools would you use?

---

# Answer

## How to Think Through a DNS Incident

The first question in any incident: **what has changed, and what hasn't?**

You told me:
- Europe: broken
- US + Asia: fine
- Frankfurt servers: healthy (curl by IP works)
- Authoritative NS records: correct

The fact that direct IP access works eliminates the application layer entirely. This is a **DNS or routing problem**, not an application problem. And since US + Asia are fine, it's **geographically scoped** — something specific to European resolution.

Now you reason through the layers.

---

### Hypothesis 1 — European recursive resolvers have a stale/wrong cached entry

The most common cause. Some European ISP resolver cached a bad IP before you fixed your records, and the TTL hasn't expired yet.

**How you'd confirm:** Query multiple European recursive resolvers directly and see what IP they're returning. Compare against what your authoritative NS serves.

**Tools:** `dig` and `nslookup`

```bash
# Query your authoritative nameserver directly
dig @ns1.myservice.com api.myservice.com A

# Query a European public resolver
dig @1.1.1.1 api.myservice.com A          # Cloudflare (global, but try it)
dig @8.8.8.8 api.myservice.com A          # Google

# Query from a European vantage point
# You'd use a tool like DNS Checker (web), or SSH into a European server and run dig there
```

If your authoritative NS returns the correct IP but European resolvers return a different IP — stale cache confirmed. You wait for TTL expiry or contact the resolver operator (impractical for ISPs).

---

### Hypothesis 2 — GeoDNS misconfiguration

You're running GeoDNS. European users should get Frankfurt IP. What if the GeoDNS rule is misconfigured and European resolver IPs are being matched to the wrong region — returning a US or Asia IP that's either unreachable from Europe or routes poorly?

**How you'd confirm:** Query your authoritative NS from a European IP and check what record it returns. If it returns a non-Frankfurt IP for European queries, your GeoDNS rules are wrong.

---

### Hypothesis 3 — Authoritative NS itself is having issues in a specific region

Your authoritative nameserver might be using anycast. The Frankfurt PoP of your DNS provider might be degraded, causing European queries to fail or time out before they reach a healthy PoP. This is rare but happens — Cloudflare, Route53, and others have had regional DNS outages.

**How you'd confirm:** Check your DNS provider's status page. Try querying your authoritative NS directly from Europe and measure response time. If queries time out rather than return wrong answers, this is the suspect.

---

### Hypothesis 4 — TTL propagation in progress after a recent change

Someone on your team recently changed a DNS record. The old record had a high TTL. European resolvers cached it before the change. US and Asia resolvers happened to re-query after the change and got the new record.

**How you'd confirm:** Check your DNS change history. Look at the TTL on the old record. Calculate whether the timing matches the incident start.

---

### Hypothesis 5 — DNS cache poisoning (rare but real)

A malicious or misconfigured resolver in Europe is returning a forged record. If DNSSEC is not enabled on your domain, this is theoretically possible.

**How you'd confirm:** If multiple independent European resolvers all return the same wrong IP that's not yours, and it's not a TTL issue, something more serious is happening. Check if DNSSEC is enabled. Report to your DNS provider.

---

## The Debugging Order

In practice you'd go in this order — fastest to confirm first:

```
1. Check what your authoritative NS is serving right now
   → eliminates "your records are wrong" immediately

2. Check what a European resolver is returning
   → if different from authoritative, it's a caching/propagation issue

3. Check GeoDNS rules
   → if authoritative returns different IPs for European vs US queries, verify correctness

4. Check DNS provider status page
   → if their infrastructure is degraded in Europe, you'll see it here

5. Check recent DNS change history + TTL values
   → correlate timing of incident with any recent changes
```

---

## The Tools — Since You Asked

These are the standard DNS debugging tools. Simple to use, worth knowing.

**`dig`** — the primary DNS debugging tool on Linux/Mac

```bash
# Basic lookup
dig api.myservice.com A

# Query a specific resolver
dig @8.8.8.8 api.myservice.com A

# Query a specific authoritative NS directly
dig @ns1.myservice.com api.myservice.com A

# Check TTL remaining on cached record
dig api.myservice.com A
# Look at the TTL field in the answer section — it counts down as cache ages

# Trace the full resolution chain step by step
dig +trace api.myservice.com A
# This shows root → TLD → authoritative, each referral explicitly
```

**`nslookup`** — simpler, available on Windows too

```bash
nslookup api.myservice.com
nslookup api.myservice.com 8.8.8.8
```

**`host`** — simplest

```bash
host api.myservice.com
```

**Online tools:**
- `dnschecker.org` — queries your domain from ~100 locations worldwide simultaneously, shows what each resolver returns. Perfect for "is this a regional issue" debugging.
- `mxtoolbox.com` — MX record debugging, email delivery issues
- `dnsviz.net` — visualizes DNSSEC chain of trust

---

## `dig +trace` — Worth Understanding Deeply

This single command shows you the entire resolution chain:

```bash
$ dig +trace www.google.com A

# Output shows:
# 1. Query to root servers → referral to .com TLD servers
# 2. Query to .com TLD → referral to google.com authoritative NS
# 3. Query to google.com NS → final A record answer
```

This is exactly what the recursive resolver does on a cache miss, made visible. If any step in the chain fails, you see exactly where.