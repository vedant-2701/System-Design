# Linux Production Incident Diagnostic Playbook

## Tags
#linux #operations #backend-engineering #debugging #incident-response

---

## Overview

- A structured diagnostic flow prevents wasted time and missed root causes
- Always establish what state the system is in before acting
- Branch based on evidence, not assumptions
- Fix root cause, not symptoms — restarting without understanding causes repeat incidents

---

## Universal First Step

```bash
systemctl status myapp
```

This single command gives you: running state, PID, restart count, last few log lines, exit code on crash. It determines which diagnostic branch you take.

---

## Diagnostic Flow

```
systemctl status myapp
        │
        ├── FAILED / INACTIVE ───────────────────────────────────────────┐
        │                                                                │
        │   journalctl -u myapp -n 100          # why did it crash?      │
        │                                                                │
        │   Common crash causes:                                         │
        │   - OOM killed (check: journalctl -k | grep "Killed process")  │
        │   - Port already in use                                        │
        │   - Missing config file / env variable                         │
        │   - Dependency not ready (DB not up yet)                       │
        │   - Permission denied on file/socket                           │
        │                                                                │
        │   Is it crash-looping?                                         │
        │   systemctl status → check restart count                       │
        │   Fix root cause BEFORE restarting                             │
        │                                                                │
        └── ACTIVE but not responding ───────────────────────────────────┘
                │
                ├── Step 1: Is it listening?
                │   ss -tlnp sport = :<port>
                │
                │   Not there → service started but failed to bind
                │     → check logs: journalctl -u myapp -n 50
                │
                │   On 127.0.0.1 → wrong interface
                │     → fix bind address in config
                │
                │   On 0.0.0.0 → binding correctly → continue
                │
                ├── Step 2: Resource exhaustion?
                │   top
                │   → high us%  = CPU-bound, check thread pool, hotspots
                │   → high wa%  = I/O bottleneck, check DB/disk latency
                │   → high load, low CPU = processes waiting (lock contention?)
                │
                │   uptime                     # quick load average
                │   nproc                      # compare load to core count
                │
                ├── Step 3: FD leak?
                │   ls /proc/<PID>/fd | wc -l
                │   cat /proc/<PID>/limits | grep "open files"
                │   ls -la /proc/<PID>/fd/ | grep deleted
                │
                ├── Step 4: Connection state problems?
                │   ss -tan | awk '{print $1}' | sort | uniq -c | sort -rn
                │
                │   CLOSE_WAIT accumulating → app not closing sockets (bug)
                │   TIME_WAIT large → too many short-lived connections
                │   Recv-Q high on listen port → accept queue full (tune somaxconn)
                │
                ├── Step 5: Dependencies reachable?
                │   ss -tnp dst :5432         # active connections to postgres
                │   ss -tnp dst :6379         # active connections to redis
                │   tcpdump -i eth0 -n host <dep-host> port <dep-port>
                │
                │   No connections → app not connecting (config? crash before connect?)
                │   Connections exist, SYN no SYN-ACK → network/firewall issue
                │   Connections exist, packets flowing → dependency-side issue
                │
                └── Step 6: Error logs
                    journalctl -u myapp -p err --since "1 hour ago"
                    journalctl -u myapp -f      # follow live
```

---

## Resource Exhaustion Cheatsheet

| Symptom | Likely Cause | Check |
|---------|-------------|-------|
| High CPU `us%` | App CPU-bound | `top P`, check thread pool size |
| High CPU `wa%` | I/O bottleneck | DB query times, disk latency |
| Load > core count | Processes queuing | `top`, check for lock contention |
| "Too many open files" | FD exhaustion | `/proc/<PID>/fd/`, `/proc/<PID>/limits` |
| Connections refused | Port not listening OR FD exhausted | `ss -tlnp`, FD count |
| Disk full but ls shows nothing | Deleted file held open | `ls -la /proc/<PID>/fd/ \| grep deleted` |
| Logs stopped | `/var` full | `df -h /var` |

---

## Signal Discipline in Incidents

```bash
# Always try graceful first
kill -15 <PID>
sleep 10

# Check if still running
ps -p <PID>

# Escalate only if necessary
kill -9 <PID>
```

**Never `kill -9` as first action.** In-flight requests drop, DB connections leak, file writes may be incomplete.

---

## Post-Incident Checklist

After service is restored:

- [ ] Root cause identified (not just "restarted and it worked")
- [ ] Log evidence preserved (`journalctl -u myapp --since "incident time" > incident.log`)
- [ ] Is the issue likely to recur? (fix vs temporary mitigation)
- [ ] Were any FDs or connections leaked? (check after restart)
- [ ] Did log rotation miss the incident window? (check logrotate config)
- [ ] Does the unit file need `StartLimitBurst` tuning?

---

## Common Incident Patterns

### Pattern 1: OOM Kill
```bash
journalctl -k | grep -i "killed process"
# or
dmesg | grep -i "oom"
```
Fix: tune JVM heap / application memory limits, add memory monitoring alert.

### Pattern 2: FD Exhaustion
```bash
ls /proc/<PID>/fd | wc -l          # current FD count
cat /proc/<PID>/limits | grep files # limit
ss -tan | grep CLOSE_WAIT | wc -l  # socket leak?
ls -la /proc/<PID>/fd | grep deleted # file leak?
```
Fix: fix socket/file close logic in application.

### Pattern 3: Crash Loop
```bash
systemctl status myapp             # high restart count
journalctl -u myapp -n 20          # look for repeated same error
```
Fix: fix root cause, don't `systemctl restart` blindly.

### Pattern 4: Dependency Failure
```bash
ss -tnp dst :5432                  # are connections to DB established?
tcpdump -i eth0 host db-host port 5432   # packets flowing?
```
Fix: restore dependency, check circuit breaker / retry logic in app.

### Pattern 5: Wrong Bind Address
```bash
ss -tlnp | grep <port>             # shows 127.0.0.1 instead of 0.0.0.0
```
Fix: change `bind` / `host` config in application config file.

---

## Failure Scenarios

- Restarting without finding root cause → same incident 30 minutes later
- Using `kill -9` first → leaked connections, corrupted write operations
- Checking logs before confirming service state → looking in wrong place
- Assuming network issue without running `tcpdump` → missing that bind address is wrong
- Not preserving logs after restart → lose crash evidence, can't do post-mortem

---

## Interview Perspective

- "Walk me through diagnosing a service that's not responding" → expect: systemctl → logs → ss → top → tcpdump
- "Service is active in systemd but not accepting connections. What do you check?" → `ss -tlnp`, then bind address, then FD count
- "High load but low CPU on a backend service — what's happening?" → high `wa%`, I/O bottleneck
- "How do you restart a service without losing the crash evidence?" → preserve logs first, then restart

---

## Revision Summary

- `systemctl status` is always step 1 — determines which branch to take
- Crashed → read logs before restarting — root cause first
- Running but broken → check: listening port → resource usage → FD count → connection states → dependencies → logs
- High `wa%` = I/O bottleneck — not CPU
- CLOSE_WAIT accumulating = FD leak = imminent failure
- `tcpdump` when logs say connection error but you can't tell where the failure is
- Never `kill -9` first — try SIGTERM, wait, then escalate

---

## Active Recall Questions

1. Service is active in systemd but HTTP requests time out. Walk through your diagnostic steps in order.
2. `top` shows low CPU but high `wa%` and slow response times. What is the bottleneck?
3. `ss -tlnp` shows your service listening on `127.0.0.1:8080`. External requests time out. What is the fix?
4. CLOSE_WAIT count is increasing every minute. What will eventually break and what is the root cause?
5. `df -h` shows `/var` at 100%. `du -sh /var/log/*` shows small log files. What else might be consuming space?
6. Service crashes on startup. `systemctl restart myapp` brings it back but it crashes again in 2 seconds. What is wrong with just restarting it?

---

## Related Concepts

- [[Linux Process Management]]
- [[Linux Systemd and Services]]
- [[Linux Log Inspection]]
- [[Linux Networking Tools]]
- [[Linux Filesystem Structure]]
- [[Linux Permissions and Ownership]]
- [[File Descriptors and epoll]]
- [[Observability — Structured Logging]]
