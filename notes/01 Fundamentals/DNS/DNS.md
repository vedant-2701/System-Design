# DNS — Domain Name System

## Tags
#networking #dns #hld #distributed-systems #foundations

---

## Overview

- DNS is a **hierarchical, distributed key-value database** mapping domain names to records (not just IPs)
- Query key = `(domain name, record type)` — same domain can have multiple record types simultaneously
- Resolution is **delegated** — no single entity knows everything; authority is split across a hierarchy
- DNS is the foundation of all internet communication — every HTTP request depends on it
- Designed for **read-heavy, eventually consistent** access patterns; caching is fundamental

---

## Why It Exists

- Humans can't memorize IPs; names need to map to addresses
- Centralized host files (like `/etc/hosts`) don't scale — billions of domains, constantly changing IPs
- DNS solves this via **hierarchical delegation**: each layer only knows its own zone, delegates the rest
- Caching at every layer makes the system fast despite the delegation chain

---

## Resolution Chain — Internal Working

### Actors

| Actor | Role | Caches? |
|---|---|---|
| Stub Resolver (OS) | Forwards queries, checks local cache + `/etc/hosts` | Yes (short-lived) |
| Recursive Resolver | Does all iterative work on client's behalf, returns final answer | Yes (heavily) |
| Root Nameserver | Returns referral to TLD NS — knows nothing about individual domains | No |
| TLD Nameserver | Returns referral to authoritative NS for the domain | No |
| Authoritative Nameserver | Source of truth — serves actual DNS records for its zone | No |

### Full Resolution Flow (cache miss)

```
Browser cache
  ↓
OS Stub Resolver (checks /etc/hosts → OS cache)
  ↓
Recursive Resolver (ISP / 8.8.8.8 / 1.1.1.1)
  ↓ cache miss → iterative resolution begins
Root Nameserver
  → referral: "for .com, ask 192.5.6.30"
TLD Nameserver (.com — Verisign)
  → referral: "for google.com, ask ns1.google.com at 216.239.32.10"
Authoritative Nameserver (ns1.google.com)
  → answer: "www.google.com → 142.250.x.x, TTL=300"
Recursive Resolver caches answer → returns to OS → OS caches → browser gets IP
```

### Key Insight
- Root and TLD servers return **referrals**, not answers
- Your OS **never** talks to root servers — only to the recursive resolver
- The recursive resolver performs all iteration; this is called **iterative resolution**
- Root servers only store ~1500 TLD delegations — stable, rarely changes

---

## DNS Record Types

| Record | Purpose | Points To |
|---|---|---|
| **A** | Maps hostname → IPv4 address | IP address |
| **AAAA** | Maps hostname → IPv6 address | IPv6 address |
| **CNAME** | Alias — maps hostname → another hostname | Another hostname (follows chain to A) |
| **MX** | Mail exchange — which servers handle email | Hostname (+ priority number) |
| **TXT** | Arbitrary text — SPF, DKIM, DMARC, domain verification | Text string |
| **NS** | Declares authoritative nameservers for a zone | Nameserver hostnames |
| **SOA** | Zone metadata — serial, timing, primary NS | Zone-level metadata |

### Critical CNAME Constraint
- CNAME **cannot** exist on an apex/bare domain (`example.com`) — only on subdomains
- Apex domain needs SOA + NS records; CNAME cannot coexist with other records on the same name
- CDN workaround: **ALIAS / ANAME / CNAME flattening** (Cloudflare, Route53) — proprietary extension

### MX Priority
- Lower number = higher priority (tried first)
- Multiple MX records = built-in mail delivery redundancy
- MX points to hostname, not IP — separate A lookup performed by sending mail server

### TXT Real-World Uses
- **SPF** — authorizes which mail servers can send for your domain
- **DKIM** — stores public key for email signing
- **DMARC** — email authentication policy
- **Domain verification** — Google/GitHub/AWS prove domain ownership

---

## TTL and Caching

### What TTL Controls
- How long recursive resolvers (and OS) cache a DNS record before re-querying
- TTL is set by the **authoritative nameserver** — domain owner controls it
- Each caching layer honors TTL independently — different resolvers expire at different times

### TTL Tradeoff Table

| TTL | Cache Hit Rate | Failover Speed | Query Volume to Auth NS |
|---|---|---|---|
| High (86400s) | Excellent | Very slow (up to 24h) | Low |
| Medium (3600s) | Good | Slow (up to 1h) | Medium |
| Low (300s) | Moderate | Fast (5 min) | High |
| Very Low (30s) | Poor | Near-instant | Very high — stampede risk |
| 0 | None | Instant | Extreme — auth NS hammered; some resolvers ignore it |

### TTL Lifecycle Strategy (Production)
1. New service → start at 60–300s (IPs not yet stable)
2. Service stabilized → raise to 3600s (optimize cache hit rate)
3. Planned migration → lower to 60s **48 hours before change** (let low TTL propagate)
4. Execute change → failover window = 1 TTL cycle
5. Post-migration stable → raise TTL back

---

## DNS Propagation

### The Problem
- Updating an authoritative NS record takes effect **immediately** on the authoritative server
- But thousands of recursive resolvers worldwide have the **old answer cached** for the remaining TTL
- Change propagates gradually as individual resolver caches expire — this is DNS propagation
- **There is no mechanism to force-invalidate cached records on external resolvers**

### Propagation Failure Scenario
```
11:00am — Mumbai server down, GeoDNS updated to Singapore IP
11:00–11:05am — Resolvers that cached Mumbai IP in last 5 min still serve it
               → Users get connection refused
11:05am+ — TTLs expire, resolvers re-query auth NS, get Singapore IP
           → Users reach Singapore server
Worst-case outage window = 1 full TTL cycle from time of last cache
```

### Mitigation
- Pre-emptively lower TTL before planned changes
- Health-check-aware DNS (Route53 failover, Cloudflare) — auto-remove unhealthy IPs from responses
- Combine low TTL + health checks for fast, automatic failover

---

## GeoDNS

### How It Works
- Authoritative nameserver returns **different A records** based on the geographic origin of the query
- Geo is inferred from the **recursive resolver's IP**, not the client's IP
- Indian user → ISP resolver in Mumbai → Auth NS sees Mumbai IP → returns Mumbai server IP

### The Resolver IP Problem
- GeoDNS sees the resolver's IP, not the user's IP
- User in Mumbai using Google's 8.8.8.8 → nearest Google resolver may be Singapore → wrong region served
- Fix: **EDNS Client Subnet (ECS)** — recursive resolver includes truncated client IP (/24) in query
  - Google 8.8.8.8: sends ECS (better geo accuracy)
  - Cloudflare 1.1.1.1: does NOT send ECS (privacy-first)

### GeoDNS Failure Scenarios
- **Stale cache after failover** — old regional IP cached; users hit dead server until TTL expires
- **Resolver IP mismatch** — user in one region, resolver in another → wrong region served
- **Auth NS regional degradation** — if DNS provider's PoP in a region fails, queries time out

### GeoDNS vs Anycast

| | GeoDNS | Anycast |
|---|---|---|
| **Mechanism** | DNS returns different IPs per region | Same IP advertised from multiple locations via BGP |
| **Failover speed** | Bounded by TTL | BGP reconverges in seconds — no TTL problem |
| **Compliance/data residency** | Deterministic — can enforce regional routing | Not deterministic — routing decided by BGP |
| **Operational complexity** | Low — DNS provider feature | High — requires BGP infrastructure |
| **Accuracy** | Depends on resolver IP / ECS | Network-level routing, highly accurate |
| **Use when** | Compliance requirements, regional features, small team | Pure latency optimization, large scale |

---

## DNS-Based Load Balancing

### Round-Robin DNS
- Multiple A records for same hostname → resolver returns all IPs → client picks one
- Provides rough traffic distribution, not true load balancing

```
api.myservice.com.    60    IN    A    1.2.3.4
api.myservice.com.    60    IN    A    5.6.7.8
api.myservice.com.    60    IN    A    9.10.11.12
```

### Limitations
- **No health awareness** — dead server IPs still returned until manually removed
- **Not request-level** — once client has IP, all requests go to that server
- **Uneven distribution** — resolver caching means different resolvers serve the same ordered list to all their users
- DNS load balancing = **traffic distribution**, not true load balancing

### Health-Check-Aware DNS
- DNS provider monitors server health, removes unhealthy IPs from responses automatically
- Route53 weighted routing + health checks is canonical example
- Still bounded by TTL for propagation — must combine with low TTL

---

## DNSSEC

### Problem It Solves
- Standard DNS responses are **unauthenticated** — no way to verify the answer came from the real authoritative NS
- **DNS cache poisoning** — attacker forges DNS response, redirects users to malicious server
- Kaminsky attack (2008) demonstrated large-scale exploitation

### How It Works
- Authoritative NS signs records with a private key
- Resolvers verify signatures using the public key
- Public keys form a **chain of trust** anchored at the root zone (signed by ICANN)

```
Root Zone (ICANN signs)
  ↓
.com TLD (Verisign signs)
  ↓
google.com (Google signs)
  ↓
A record — cryptographically verified as authentic
```

### What DNSSEC Does NOT Do
- Does NOT encrypt DNS queries — queries are still visible on the network (DNS-over-HTTPS / DNS-over-TLS solves this)
- Does NOT prevent DDoS on DNS infrastructure
- Does NOT solve TTL/propagation problems

### Operational Risks
- Larger DNS responses → more UDP fragmentation → potential resolution failures
- **Key rotation complexity** — incorrect rotation = domain goes completely dark
- Real outages caused by DNSSEC misconfiguration (`.gov` domains have gone dark)
- Managed providers (Cloudflare, Route53) automate key rotation — dramatically reduces risk

### When to Enable
- Threat model: users submitting credentials/PII/payments → enable regardless of industry
- Using Cloudflare/Route53 → enable (single toggle, automated key management)
- Self-hosted DNS → evaluate operational maturity before enabling; misconfiguration risk is high

---

## Failure Scenarios

### Cache Stampede (Thundering Herd)
- TTL expires simultaneously for millions of users
- All recursive resolvers re-query authoritative NS at the same moment
- Auth NS overwhelmed → delayed responses → resolver timeouts → resolver retries → load doubles
- Mitigation: staggered TTLs, rate limiting on auth NS, anycast distribution of auth NS load

### DNS Propagation Lag
- IP changed on auth NS but old IP cached across resolvers worldwide
- Users directed to dead/wrong server for up to 1 full TTL cycle
- Mitigation: pre-emptive TTL reduction 48h before changes

### Resolver IP Geolocation Mismatch (GeoDNS)
- User in region A uses resolver in region B → wrong server returned
- Mitigation: EDNS Client Subnet (ECS), or switch to Anycast

### Authoritative NS Unavailability
- If auth NS goes down, recursive resolvers with cached records continue serving — no immediate impact
- After TTL expires, new queries fail — resolution breaks for the domain
- Mitigation: always run multiple authoritative nameservers (NS records should list 2+ servers)

### DNSSEC Misconfiguration
- Expired or incorrectly rotated signing keys → resolvers reject all responses for the domain → domain unreachable
- Mitigation: use managed DNS providers with automated key rotation

---

## Debugging DNS — Tools and Process

### Systematic Debugging Order
```
1. Query your authoritative NS directly
   → confirms what the source of truth is serving
2. Query public/regional resolvers
   → reveals what cached answer end users are getting
3. Compare (1) vs (2)
   → mismatch = propagation/caching issue
4. Check GeoDNS rules if geographic scope
   → query auth NS from different regional IPs
5. Check DNS provider status page
   → auth NS infrastructure degradation
6. Check recent change history + TTL values
   → correlate incident timing with changes
```

### Key Commands
```bash
# Query authoritative NS directly (bypasses caching)
dig @ns1.myservice.com api.myservice.com A

# Query a specific public resolver
dig @8.8.8.8 api.myservice.com A

# Trace full resolution chain (what recursive resolver actually does)
dig +trace api.myservice.com A

# Check TTL remaining on cached record
dig api.myservice.com A   # TTL field in answer section counts down
```

### Online Tools
- `dnschecker.org` — queries from ~100 global locations simultaneously (best for propagation debugging)
- `dnsviz.net` — visualizes DNSSEC chain of trust
- `mxtoolbox.com` — MX record and email delivery debugging

---

## Real-World Usage

- **Cloudflare 1.1.1.1** — public recursive resolver, privacy-first (no ECS), anycast distributed
- **Google 8.8.8.8** — public recursive resolver, sends ECS for geo accuracy
- **Route53** — AWS managed authoritative DNS + GeoDNS + health-check failover
- **Cloudflare DNS** — authoritative + CNAME flattening (solves apex CNAME limitation) + DNSSEC automation
- **Anycast** — Cloudflare, Fastly, Akamai serve entire networks from a single IP across 200+ PoPs

---

## Common Mistakes

- Confusing recursive resolver with authoritative nameserver — they are fundamentally different actors
- Assuming DNS changes propagate instantly — they do not; TTL controls the window
- Setting TTL=0 expecting instant propagation — many resolvers ignore it; auth NS gets hammered
- Putting CNAME on apex domain — spec violation; use ALIAS/ANAME flattening instead
- Enabling DNSSEC on self-hosted DNS without understanding key rotation — domain can go dark
- Using GeoDNS for compliance without accounting for resolver IP mismatch — users may hit wrong region
- Treating DNS load balancing as equivalent to a load balancer — it is not; no health awareness, not request-level

---

## Interview Perspective

### Commonly Tested Areas
- Full DNS resolution chain with all actors named and roles precise
- TTL tradeoff: low vs high, when to use each
- GeoDNS vs Anycast: mechanism, tradeoffs, when to choose
- DNS propagation: why it exists, how long it takes, how to minimize the window
- DNSSEC: what it solves, what it doesn't, operational risks
- DNS-based load balancing limitations vs real load balancers

### Questions Candidates Commonly Miss
- "Who does the actual iterative resolution work?" → Recursive resolver, not the client
- "What do root and TLD servers return?" → Referrals, not answers
- "Why can't CNAME be on an apex domain?" → Cannot coexist with SOA/NS records
- "How do you minimize DNS failover time?" → Pre-emptively lower TTL + health-check-aware DNS
- "What's the difference between GeoDNS and Anycast?" → DNS-layer vs network-layer routing

### Expected Tradeoff Discussions
- TTL selection for new vs stable services
- GeoDNS accuracy limitations (resolver IP, ECS tradeoffs)
- DNSSEC security vs operational complexity
- DNS load balancing limitations (always needs a real LB for request-level routing)

---

## Revision Summary

- DNS is a hierarchical distributed database; query = `(name, record type)` not just `(name)`
- OS talks only to recursive resolver; recursive resolver does iterative resolution (root → TLD → auth)
- Root and TLD return **referrals**; only authoritative NS returns the actual record
- TTL controls cache lifetime; DNS changes propagate over 1 full TTL cycle — not instantly
- Pre-emptively lower TTL 48h before planned IP changes to shrink the propagation window
- GeoDNS infers geography from **resolver IP**, not client IP; ECS partially fixes this
- Anycast: same IP, multiple locations, BGP routing — no TTL failover problem; requires BGP infra
- DNSSEC adds cryptographic signatures to records; does NOT encrypt queries; misconfiguration = domain dark
- DNS load balancing is coarse-grained traffic distribution — not request-level load balancing
- Cache stampede: simultaneous TTL expiry → thundering herd on auth NS → cascading timeouts

---

## Active Recall Questions

1. Walk through the full DNS resolution chain for `api.myservice.com` from browser to final IP. Name every actor and what each returns.
2. Why can't root servers store the IP of every domain on the internet?
3. Your authoritative NS serves the correct IP. Users in Europe still hit the old IP. Why? What controls how long this lasts?
4. Why can't a CNAME record exist on an apex domain? How do CDNs solve this?
5. What is the difference between a recursive resolver and an authoritative nameserver?
6. You need to migrate your service to a new IP with zero downtime. Walk through your DNS strategy step by step.
7. GeoDNS returns the wrong region for some users. What are the two causes and how do you fix each?
8. What does DNSSEC protect against? What does it explicitly NOT protect against?
9. Why is DNS-based load balancing not a replacement for a real load balancer?
10. You have TTL=86400. Your server goes down at 11am. At what point do users start hitting your failover server? What would you do differently next time?
11. What is EDNS Client Subnet? What is the tradeoff between sending ECS and not sending it?
12. Anycast vs GeoDNS — when would you choose each, and why?

---

## Related Concepts

- [[HTTP & HTTPS]]
- [[TCP IP Model]]
- [[Load Balancing]]
- [[CDN & Caching Layers]]
- [[GeoDNS & Anycast]]
- [[TLS Handshake]]
- [[Distributed Caching]]
- [[Cache Invalidation]]
- [[BGP Routing]]
- [[High Availability Patterns]]
