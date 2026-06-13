# IP Addressing, Subnets & CIDR

## Tags
#networking #ip #infrastructure #backend-engineering

---

## Overview

- IPv4 addresses are 32-bit integers written as 4 octets (e.g. `192.168.1.105`)
- Every address encodes two things: **network portion** + **host portion**
- The split is defined by the **subnet mask**
- CIDR (Classless Inter-Domain Routing) compresses subnet masks into prefix notation (`/24`)
- Routing is possible because IP addresses are hierarchical — unlike flat MAC addresses

---

## Why It Exists

- MAC addresses are flat — no location information, unroutable across networks
- IP introduces hierarchical, location-aware addressing enabling routing across the internet
- CIDR replaced classful addressing (Class A/B/C) to eliminate wasteful fixed-size allocations

---

## Subnet Mask & Address Breakdown

```
IP:   192.168.1.105
Mask: 255.255.255.0  →  binary: 11111111.11111111.11111111.00000000

Network portion: 192.168.1   (first 24 bits — the 1s in the mask)
Host portion:    105          (last 8 bits — the 0s in the mask)
```

Reserved addresses in every subnet:
- `.0` → **network address** (identifies the subnet)
- `.255` → **broadcast address** (sends to all hosts)
- Usable hosts = 2^(host bits) − 2

---

## CIDR Notation

```
192.168.1.0/24   →  256 addresses, 254 usable
192.168.1.0/25   →  128 addresses, 126 usable
10.0.0.0/16      →  65536 addresses, 65534 usable
x.x.x.x/32      →  1 address (single host)
0.0.0.0/0        →  entire internet (default route)
```

**Longest prefix match**: routers pick the most specific matching route. `/24` beats `/16` beats `/0`.

---

## Private IP Ranges (RFC 1918)

Not routable on the public internet:
```
10.0.0.0/8
172.16.0.0/12
192.168.0.0/16
```

Used inside VPCs, home networks, container networks. Require NAT to reach the internet.

---

## /32 Use Cases

Not used as a subnet (a network of one host is meaningless for routing). Used in:
- **Firewall/security group rules**: `allow SSH from 203.0.113.5/32` — exactly one IP
- **Route table entries**: send `10.2.5.7/32` to a specific target
- **Allowlisting**: restrict a webhook endpoint to one known third-party IP

---

## Production Engineering Relevance

**VPC design (AWS/GCP):**
```
VPC:             10.0.0.0/16      (65k IPs total)
Public subnet:   10.0.1.0/24      (254 usable)
Private subnet:  10.0.2.0/24      (254 usable)
```
- Subnets within the same VPC **cannot overlap**
- Poor upfront CIDR planning → run out of IPs → cannot add instances without destroying infrastructure
- Kubernetes pod CIDR: each node gets a slice; pods get IPs from that slice

**Security group rules use CIDR:**
- `10.0.0.0/8` = allow entire internal network
- `203.0.113.0/24` = allow a specific external subnet
- `0.0.0.0/0` = allow all (dangerous for inbound)

---

## Failure Scenarios

- **CIDR overlap**: two subnets overlap → routing ambiguity → packets go to wrong destination
- **IP exhaustion**: undersized CIDR block for a VPC → can't launch new instances
- **Broadcast storms**: large /8 subnet with excessive broadcast traffic — segment into smaller subnets
- **Subnet miscalculation**: forgetting to subtract 2 (network + broadcast) → off-by-one in capacity planning

---

## Tradeoffs

| Classful (old) | CIDR (modern) |
|----------------|--------------|
| Fixed /8, /16, /24 sizes | Arbitrary prefix length |
| Wasteful allocation | Right-size allocation |
| Simple routing tables | Larger, more complex routing tables |
| No longer used | Universal standard |

---

## Interview Perspective

- "How many usable hosts in a /26?" → 2^6 − 2 = 62
- "Can two subnets overlap in a VPC?" → No, causes routing ambiguity
- "What does /32 mean in a security group rule?" → Exactly one IP
- Common mistake: forgetting to subtract 2 for network and broadcast addresses
- Common mistake: claiming subnets can be carved out of an already-allocated block

---

## Revision Summary

- IP = 32 bits; split into network + host by subnet mask
- CIDR `/N` = first N bits are network; remaining bits are hosts
- Usable hosts = 2^(32-N) − 2
- Private ranges (10/8, 172.16/12, 192.168/16) not routable publicly → need NAT
- /32 = single host; used in firewall rules and route entries
- Longest prefix match determines routing — more specific always wins
- VPC CIDR planning must be done upfront; overlapping subnets are illegal

---

## Active Recall Questions

1. A VPC has CIDR `10.0.0.0/16`. You create subnet `10.0.5.0/24`. Can you also create `10.0.5.128/25`? Why or why not?
2. How many usable hosts are in a `/28`?
3. Why are private IP ranges not routable on the internet?
4. What does longest prefix match mean in routing?
5. When would you use a `/32` in a security group rule?

---

## Related Concepts

- [[NAT and Routing]]
- [[OSI Model & TCP/IP Stack]]
- [[TCP - Three-Way Handshake and Connection Lifecycle]]
- [[Port Exhaustion and TIME_WAIT]]
