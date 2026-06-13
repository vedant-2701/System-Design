# Question

You're designing the DNS strategy for a new global API service. You need to decide:

1. What TTL to set on your A records
2. Whether to use GeoDNS or Anycast for geographic routing
3. Whether to enable DNSSEC

For each of the three decisions — what factors drive your choice, and what are you trading off? 

---

# Answer

## Your Answers — Precise Evaluation

**TTL — Correct but incomplete.**

Starting at 60-300s for a new service is right. Gradual increase as you stabilize is right. But you identified cache stampede as the risk without connecting it to *what you'd do about it*. A senior engineer would say: low TTL + health-check-aware DNS (Route53 failover) gives you fast failover without stampede because your authoritative NS handles the query volume, not your application servers. Also missing: the operational trigger for *when* you raise TTL — you raise it only after your IP addresses are stable and you have confidence in your infrastructure.

**GeoDNS vs Anycast — Strong answer.**

The compliance/data residency angle is exactly right and often missed by candidates. Anycast for pure latency optimization, GeoDNS when you need deterministic regional routing for legal reasons. One thing to add: Anycast requires you to operate BGP infrastructure or use a provider that does (Cloudflare, Fastly). GeoDNS is simpler operationally — just a DNS provider feature. For a small team, GeoDNS is often the pragmatic choice even without compliance requirements.

**DNSSEC — Directionally correct, but the reasoning has a gap.**

You framed it as "banking/government/healthcare vs others" — that's an industry framing, not an engineering framing. The real question is: *what's your threat model?* DNSSEC protects against cache poisoning attacks. If your users are submitting credentials, payment info, or sensitive data through your API, a cache poisoning attack that redirects them to a malicious server is catastrophic — regardless of industry. A SaaS B2B API with enterprise customers has the same risk profile as a fintech app. The packet size concern is real but overstated — modern DNS handles it. The key rotation complexity is the more legitimate operational concern.

---

## How a Senior Engineer Actually Thinks Through This

The mental model isn't "what are the rules for each decision." It's: **what are you optimizing for, what are you risking, and what's your operational capacity to manage complexity.**

---

### The thinking process

**Step 1 — Establish your constraints first**

Before any technical decision, ask:
- Do we have compliance requirements? (GDPR, data residency, HIPAA)
- What's our team size and operational maturity?
- What's the cost of downtime? (SLA, revenue impact)
- What's our current traffic scale?

These answers drive everything else. A three-person startup and a 500-person company make different DNS decisions for rational reasons.

---

**Step 2 — TTL decision tree**

```
New service, IPs not stable yet?
  → TTL 60-300s. Accept query volume cost.
  → Add health checks to DNS records immediately.
  → Plan: raise TTL to 3600s once stable for 30 days.

Stable service, planned migration coming?
  → Lower TTL to 60s 48 hours before migration.
  → Execute migration.
  → Raise TTL back after confirming stability.

Stable service, no changes planned?
  → TTL 3600-86400s. Optimize for cache hit rate.
  → Resolver query volume drops dramatically.
```

The senior insight: **TTL management is a lifecycle, not a one-time setting.** You actively manage it around operational events.

---

**Step 3 — Geographic routing decision tree**

```
Do you need deterministic regional routing?
  (compliance, data residency, regional feature differences)
  → GeoDNS. Accept the ECS/resolver-IP accuracy limitation.
  → Combine with health checks for failover.

Do you just want low latency globally?
  → Anycast if you have the infrastructure or use a provider.
  → Anycast is operationally simpler once set up — no DNS rule maintenance.
  → No TTL propagation problem for failover (BGP reconverges in seconds).

Small team, limited infrastructure?
  → GeoDNS via Route53 or Cloudflare. Managed service.
  → Anycast requires BGP expertise or significant provider dependency.
```

The senior insight: **Anycast is superior for pure latency but has a higher infrastructure bar.** GeoDNS is the pragmatic default for most teams.

---

**Step 4 — DNSSEC decision tree**

```
What data flows through this API?
  → Credentials, PII, payment data → enable DNSSEC. Attack surface is real.
  → Internal service mesh, non-sensitive data → lower priority.

What's your DNS provider support?
  → Route53, Cloudflare — DNSSEC is a checkbox. Low operational overhead.
  → Self-hosted BIND — key rotation is manual and risky. Higher bar.

What's your team's operational maturity?
  → If you can't monitor DNSSEC key expiry and rotation → don't enable it yet.
  → A misconfigured DNSSEC is worse than no DNSSEC — your domain goes dark.

Are you using a CDN or third-party DNS proxy?
  → Cloudflare proxied mode — DNSSEC handled for you automatically.
  → Non-proxied — you manage it.
```

The senior insight: **DNSSEC is not about industry — it's about threat model and operational readiness.** The question isn't "are we important enough to need security" but "can we operate this safely without misconfiguring it."

---

### The combined answer for your global API service

Here's how I'd actually decide:

**TTL:** Start at 300s. Set up health-check-aware DNS records from day one. Once the service has been stable for 30 days with no IP changes, raise to 3600s. Document this as a runbook step for any future migrations.

**Routing:** If it's a pure API with no compliance requirements — start with GeoDNS via Route53 or Cloudflare. It's a managed feature, low operational overhead, good enough for most scale. Revisit Anycast if you grow to the point where you're operating your own PoPs or if GeoDNS accuracy becomes a real problem.

**DNSSEC:** If using Cloudflare as DNS provider — enable it. It's a single toggle, they handle key management. If on Route53 — enable it, AWS automates key rotation. If self-hosting DNS — skip it until you have the operational maturity. The risk of misconfiguration outweighs the security benefit for a small team.

---