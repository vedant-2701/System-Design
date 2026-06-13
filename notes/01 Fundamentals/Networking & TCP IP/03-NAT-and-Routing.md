# NAT & Routing

## Tags
#networking #nat #routing #infrastructure #backend-engineering

---

## Overview

- **Routing**: forwarding packets hop-by-hop toward destination using routing tables
- **NAT (Network Address Translation)**: translates private IPs to a public IP at network boundary
- NAT exists because IPv4 (~4.3B addresses) is insufficient for the number of internet-connected devices
- Every device in a home/VPC has a private IP; NAT presents a single public IP to the internet

---

## Routing

### How It Works

Every router maintains a **routing table**:

```
Destination        Gateway         Interface
10.0.1.0/24        —               eth0      (local — deliver directly)
172.16.0.0/12      10.0.0.254      eth0      (send to internal gateway)
0.0.0.0/0          10.0.0.1        eth0      (default route — everything else)
```

- Router applies **longest prefix match** — most specific rule wins
- `/24` match beats `/16` beats `/0`
- Packet forwarded to next-hop gateway; process repeats until destination network
- Default route `0.0.0.0/0` = catch-all → gateway to internet

### TTL (Time To Live)

- Every IP packet carries a TTL counter
- Each router decrements TTL by 1
- TTL hits 0 → router drops packet and sends ICMP "Time Exceeded" back to sender
- Prevents packets from looping forever in routing loops
- **traceroute exploits this** (see [[ICMP, ping, traceroute]])

---

## NAT (NAPT — Network Address Port Translation)

### The Problem

Private IPs (`10.x.x.x`, `192.168.x.x`) cannot be routed on the public internet — millions of networks use the same private ranges. Packets from `192.168.1.5` are unroutable externally; return packets have nowhere to go.

### How NAT Works

```
Internal device:  192.168.1.5:54321
                        ↓
               NAT Gateway (router)
               Public IP: 203.0.113.10
                        ↓
Outgoing packet rewritten:
  Source: 203.0.113.10:41234   (public IP + new port)
                        ↓
               Destination server
                        ↓
Response returns to 203.0.113.10:41234
                        ↓
NAT table lookup: 41234 → 192.168.1.5:54321
                        ↓
Packet rewritten, forwarded internally
```

NAT maintains a **translation table**: `(public IP, public port) ↔ (private IP, private port)`

---

## Outbound vs Inbound Through NAT

| Direction | Works? | Why |
|-----------|--------|-----|
| Outbound (internal → internet) | Yes | NAT creates table entry on first packet |
| Inbound (internet → internal) | No (by default) | No existing NAT table entry; packet dropped |

**Implications:**
- Servers need public IPs or a load balancer with public IP
- AWS instances in private subnets use a **NAT Gateway** for outbound; cannot receive inbound directly
- WebSocket servers: client initiates (creating NAT entry), then bidirectional communication works
- Protocols that embed IPs in payload (FTP active mode, some VoIP) break through NAT — NAT only rewrites headers

---

## Port Exhaustion Through NAT

NAT maps outbound connections to `(public IP, public port)` pairs. Ports are 16-bit = 65535 total, ~64k ephemeral.

**The limit is per `(source IP, destination IP, destination port)` tuple:**
- 64k concurrent connections to `api.example.com:443`
- Another 64k to `api2.example.com:443`

**Real production problem — TIME_WAIT accumulation:**

With high-frequency short-lived connections:
```
800 new connections/sec to same destination
Each enters TIME_WAIT for 60 sec after close
800 × 60 = 48,000 ports consumed after 60 sec
Ephemeral range ≈ 64,000 → nearly exhausted
New connections fail with EADDRNOTAVAIL
```

**Fixes:**
- Connection pooling (primary fix — reuse connections)
- `SO_REUSEADDR` / `tcp_tw_reuse` kernel tunable
- Multiple NAT IPs (AWS NAT Gateway scales this way)
- Tune ephemeral port range: `/proc/sys/net/ipv4/ip_local_port_range`

---

## Failure Scenarios

- **Port exhaustion**: `EADDRNOTAVAIL` on new connections under sustained load; service appears healthy otherwise
- **NAT table overflow**: large number of concurrent connections exceeds NAT table capacity → new connections silently dropped
- **Keep-alive mismatch**: upstream closes connection; client reuses stale pool connection → RST / broken pipe
- **Asymmetric routing**: packet goes out one path, return comes back different path → NAT table miss → dropped

---

## Scaling Considerations

- NAT is a **single point of failure** — NAT Gateway must be HA (AWS NAT Gateway is managed + HA per AZ)
- NAT Gateway scales by adding public IPs → multiplies available port budget
- High-throughput services should minimize reliance on NAT via connection pooling
- For massive outbound volume: consider multiple NAT Gateways across AZs + route tables per AZ

---

## Interview Perspective

- "Why can't servers in a private subnet receive inbound traffic?" → No public IP; NAT has no entry for unsolicited inbound
- "What causes EADDRNOTAVAIL under load?" → Port exhaustion from TIME_WAIT accumulation
- "How does AWS NAT Gateway scale?" → Adds public IPs, multiplying port budget
- Common mistake: saying port exhaustion limit is global (it's per destination tuple)

---

## Revision Summary

- Routing uses longest prefix match on routing tables; TTL prevents infinite loops
- NAT translates `(private IP, private port) ↔ (public IP, public port)` at boundary
- Outbound NAT works automatically; inbound requires explicit mapping (LB or public IP)
- Port exhaustion: TIME_WAIT × connection rate → `EADDRNOTAVAIL`; fixed by connection pooling
- Protocols embedding IPs in payload break through NAT
- NAT Gateway must be AZ-aware for HA

---

## Active Recall Questions

1. Why can't a server in a private subnet receive inbound connections through a NAT gateway?
2. What is the theoretical port exhaustion limit when making connections to a single upstream service?
3. Your service makes 500 new TCP connections/sec to `api.example.com:443`. After 60 seconds, how many ports are in TIME_WAIT?
4. What breaks when a protocol embeds IP addresses in its payload and passes through NAT?
5. What kernel file controls the ephemeral port range on Linux?

---

## Related Concepts

- [[IP Addressing, Subnets, CIDR]]
- [[TCP - Three-Way Handshake and Connection Lifecycle]]
- [[Port Exhaustion and TIME_WAIT]]
- [[ICMP, ping, traceroute]]
- [[HTTP Connection Pooling]]
