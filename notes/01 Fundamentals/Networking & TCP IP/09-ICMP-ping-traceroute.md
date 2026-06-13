# ICMP, ping & traceroute

## Tags
#networking #diagnostics #icmp #backend-engineering

---

## Overview

- **ICMP** (Internet Control Message Protocol) operates at Layer 3 alongside IP
- Not for application data — for **network diagnostics and error reporting**
- **ping** measures reachability and round-trip latency using ICMP Echo
- **traceroute** maps the path to a destination by exploiting the IP TTL field

---

## ICMP Message Types

```
Type 0   — Echo Reply           (ping response)
Type 8   — Echo Request         (ping)
Type 11  — Time Exceeded        (TTL expired — used by traceroute)
Type 3   — Destination Unreachable  (port closed, host unreachable)
Type 4   — Source Quench        (deprecated congestion signal)
```

ICMP is transported directly in IP packets — it sits at Layer 3, not Layer 4. No ports, no connections.

---

## ping

Sends ICMP Echo Request, expects ICMP Echo Reply:

```bash
ping 8.8.8.8

PING 8.8.8.8: 56 bytes of data
64 bytes from 8.8.8.8: icmp_seq=1 ttl=117 time=12.4 ms
64 bytes from 8.8.8.8: icmp_seq=2 ttl=117 time=11.8 ms
```

**What it tells you:**
- Host is reachable (at ICMP level)
- Round-trip latency (RTT)
- Packet loss percentage

**What it does NOT tell you:**
- Whether a specific TCP port is open
- Application-level health
- A host can be pingable but have port 443 closed
- A host can have ICMP blocked (firewalled) but TCP 443 working fine

---

## traceroute — How It Works

Exploits the **IP TTL field**:

```
Every IP packet has TTL (Time To Live) — decremented by 1 at each router hop
When TTL hits 0: router drops packet, sends ICMP Type 11 "Time Exceeded" back to sender
This ICMP reply includes the router's IP address and arrives with measurable RTT
```

traceroute sends packets with **incrementally increasing TTL**:

```
TTL=1 → first router decrements to 0 → drops + sends ICMP "Time Exceeded"
        → you learn: router 1 IP, RTT to hop 1

TTL=2 → second router decrements to 0 → drops + ICMP
        → you learn: router 2 IP, RTT to hop 2

TTL=n → destination reached → sends ICMP Echo Reply (or TCP RST)
        → full path mapped
```

### Example Output

```bash
traceroute google.com

 1  192.168.1.1      1.2 ms    ← home router
 2  10.10.1.1        4.8 ms    ← ISP gateway
 3  * * *                      ← router not responding to ICMP (firewall)
 4  72.14.198.1      11.3 ms   ← ISP backbone / peering point
 5  216.239.51.9     13.1 ms   ← Google network
 6  8.8.8.8          14.2 ms   ← destination
```

---

## `* * *` — What It Means

A hop showing `* * *` means that router **did not send an ICMP Time Exceeded response**. Reasons:
- Firewall blocking ICMP outbound (security policy — don't reveal internal topology)
- Router deprioritizes ICMP response generation under load
- ICMP responses rate-limited

**Critical: `* * *` does NOT mean the router is broken or traffic isn't flowing through it.** Real TCP/UDP traffic passes through normally. Only ICMP responses are suppressed. You can confirm: if subsequent hops respond and destination is reachable, the `* * *` hop is functional.

---

## Production Use Cases

**Diagnosing latency spikes:**
```
Your service → hop 1 (1ms) → hop 2 (2ms) → hop 3 (45ms jump) → destination
→ Latency introduced between hop 2 and 3
→ Peering point congestion, ISP issue, or cross-region transit
```

**Diagnosing packet loss:**
```
traceroute -q 10  (send 10 probes per hop)
→ hop 4: 30% loss on probes
→ congestion or misconfigured router at that hop
```

**Mapping cloud network paths:**
```
Client → CDN edge → origin region
traceroute reveals if traffic is hitting the correct edge node
or routing suboptimally (e.g. bypassing CDN)
```

**Confirming traffic is NOT going through unexpected hops (security):**
- BGP hijacking can reroute traffic through unexpected countries/ASes
- traceroute reveals the actual path

---

## ICMP in Production Firewalls

Many production firewalls block ICMP:

| ICMP Action | Impact |
|-------------|--------|
| Block all ICMP inbound | ping fails; traceroute shows `* * *` at this host |
| Block ICMP outbound | Your host can't send ICMP error messages back |
| Allow ICMP from specific IPs | Monitoring/alerting systems can ping; public cannot |

**Danger of blocking all ICMP:** ICMP Type 3 "Destination Unreachable" and ICMP Type 4 "Source Quench" carry important network signals. Blocking these can break path MTU discovery → TCP connections hang with large packets. Recommendation: block ICMP Echo selectively, not all ICMP.

---

## Tradeoffs

| Tool | What it reveals | Limitation |
|------|----------------|-----------|
| ping | Reachability, RTT, loss | ICMP may be blocked; doesn't test TCP ports |
| traceroute | Path, per-hop latency | `* * *` hops; ICMP-blocking routers invisible; path may differ per flow |
| `curl -v` | TCP + TLS + HTTP health | Application layer; doesn't show network path |
| `telnet host port` | TCP port reachability | Binary: open/closed, no path info |

---

## Failure Scenarios

- **ping fails but service works**: ICMP blocked on host; TCP/HTTP fine. Don't use ping as health check.
- **traceroute shows `* * *` at destination**: host up but ICMP blocked. Use TCP-based health check instead.
- **Sudden latency increase at specific hop**: ISP congestion, peering issue, or BGP route change.
- **Asymmetric path**: outbound and return paths differ (common); traceroute only shows outbound path.
- **MTU blackhole**: ICMP Type 3 Code 4 (fragmentation needed) blocked by firewall → TCP connections hang for large payloads → `tcp_mtu_probing` as mitigation.

---

## Interview Perspective

- "traceroute shows `* * *` at hop 3 but destination is reachable — what does this mean?" → Router exists but doesn't emit ICMP; traffic flows normally
- "ping succeeds but your HTTP endpoint times out — what do you check?" → TCP port open? TLS working? App healthy? ICMP success ≠ app health
- "How does traceroute work?" → Incremental TTL; each router sends ICMP Time Exceeded when TTL=0; RTT + IP collected per hop
- "Why shouldn't you block all ICMP on a firewall?" → Path MTU discovery uses ICMP Type 3; blocking it breaks large TCP transfers
- Common mistake: treating ping failure as proof the host is down

---

## Revision Summary

- ICMP = Layer 3 control protocol; for diagnostics and error reporting, not application data
- ping = ICMP Echo Request/Reply; measures reachability and RTT; ICMP may be firewalled
- traceroute = incremental TTL trick; collects ICMP Time Exceeded from each hop; builds path map
- `* * *` = ICMP suppressed at that hop; does NOT mean traffic is blocked
- Production use: diagnose latency source, confirm routing, detect BGP issues
- Don't block all ICMP — path MTU discovery requires ICMP Type 3

---

## Active Recall Questions

1. How does traceroute actually discover each hop's IP and RTT?
2. traceroute shows `* * *` at hop 4 but you can reach the destination. Is hop 4 broken?
3. ping to your server succeeds, but your HTTP endpoint returns no response. What does this tell you?
4. Why is blocking all ICMP on a production firewall dangerous?
5. You see a 40ms latency jump between hop 5 and hop 6 in traceroute. What could cause this?

---

## Related Concepts

- [[OSI Model & TCP/IP Stack]]
- [[IP Addressing, Subnets, CIDR]]
- [[NAT and Routing]]
- [[TCP - Three-Way Handshake and Connection Lifecycle]]
