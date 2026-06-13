# Linux Log Inspection

## Tags
#linux #observability #backend-engineering #operations #foundations

---

## Overview

- Modern Linux has two parallel logging systems: journald (systemd-managed) and traditional `/var/log` files
- Most systemd services log to journald by default — not to `.log` files
- journald stores logs in binary format — requires `journalctl` to read
- Traditional services write to `/var/log` — read with `grep`, `tail`, `awk`
- Log rotation is mandatory in production — unmanaged logs fill `/var` and blind the system

---

## Two Logging Systems

### journald

- Daemon: `systemd-journald`
- Storage: binary format in `/run/log/journal/` (volatile) or `/var/log/journal/` (persistent)
- Sources: all systemd services, kernel, boot process, anything writing to stdout/stderr
- Configured in unit file: `StandardOutput=journal` / `StandardError=journal`
- Tool: `journalctl`

### Traditional /var/log

- Plain text files written directly by services
- Common locations:

```
/var/log/syslog          → general system messages
/var/log/auth.log        → authentication, sudo, SSH logins
/var/log/kern.log        → kernel messages
/var/log/nginx/
│   ├── access.log       → every HTTP request
│   └── error.log        → nginx errors
/var/log/postgresql/     → database logs
```

---

## journalctl — Essential Commands

```bash
# Follow live (like tail -f)
journalctl -u myapp -f

# Last N lines
journalctl -u myapp -n 100

# Since last boot
journalctl -u myapp -b

# Time-based filtering — most powerful feature
journalctl -u myapp --since "2024-01-04 14:00:00" --until "2024-01-04 16:00:00"
journalctl -u myapp --since "1 hour ago"
journalctl -u myapp --since "yesterday"

# Filter by priority
journalctl -u myapp -p err        # errors and above
journalctl -u myapp -p warning    # warnings and above

# Kernel messages only
journalctl -k

# All services, errors only, last hour
journalctl -p err --since "1 hour ago"

# Check journal disk usage
journalctl --disk-usage

# Pattern search (grep-style)
journalctl -u myapp -g "connection refused"
```

**Priority levels:** 0=emergency, 1=alert, 2=critical, 3=error, 4=warning, 5=notice, 6=info, 7=debug

---

## grep / tail — Traditional Log Tools

```bash
# Follow log in real time
tail -f /var/log/nginx/error.log

# Last N lines
tail -n 100 /var/log/nginx/access.log

# Search for pattern
grep "ERROR" /var/log/myapp/app.log

# Search with context (3 lines before and after match)
grep -C 3 "OutOfMemoryError" /var/log/myapp/app.log

# Case-insensitive
grep -i "error" /var/log/myapp/app.log

# Count occurrences
grep -c "ERROR" /var/log/myapp/app.log

# Show line numbers
grep -n "ERROR" /var/log/myapp/app.log

# Search across multiple files
grep -r "connection refused" /var/log/

# Combine: errors in last 500 lines
tail -n 500 /var/log/myapp/app.log | grep "ERROR"
```

---

## journalctl vs grep — When to Use Which

| Need | Tool |
|------|------|
| Time-based filtering | `journalctl --since/--until` |
| Service-specific logs (systemd) | `journalctl -u servicename` |
| Pattern search in traditional logs | `grep` |
| Live log following | Both: `journalctl -f` or `tail -f` |
| Cross-service correlation | `journalctl` (all in one place) |
| Parsing structured log formats | `grep` + `awk` or `jq` for JSON logs |

**Rule:** If the service logs to journald → use `journalctl`. Time-based filtering is far easier than constructing timestamp regex for `grep`.

---

## Log Rotation

Without rotation, logs grow indefinitely → `/var` fills → service can't write logs → system is blind.

### logrotate

Config location: `/etc/logrotate.d/myapp`

```
/var/log/myapp/*.log {
    daily
    rotate 14
    compress
    delaycompress
    missingok
    notifempty
    postrotate
        kill -HUP $(cat /var/run/myapp.pid)
    endscript
}
```

| Directive | Meaning |
|-----------|---------|
| `daily` | Rotate every day |
| `rotate 14` | Keep 14 rotated files |
| `compress` | Gzip old logs |
| `delaycompress` | Don't compress most recent rotated file (may still be written to) |
| `missingok` | Don't error if log file missing |
| `notifempty` | Don't rotate empty files |
| `postrotate` | Run after rotation |

**Critical: `postrotate` signal:**
After rotation, the old file is renamed. The service still has an FD pointing to the old file (now at a different path). It keeps writing there. New log file gets no new entries.

Fix: signal the service to close and reopen its log file:
```bash
kill -HUP <PID>    # most daemons reopen log files on SIGHUP
```

Without this: service writes to renamed/rotated file, new log file stays empty.

---

## Production Incident Flow — Logs

```bash
# Step 1: Check if service is running and see recent errors
systemctl status myapp

# Step 2: Check last N lines for crash reason
journalctl -u myapp -n 100

# Step 3: Check error-level logs in specific timeframe
journalctl -u myapp -p err --since "2 hours ago"

# Step 4: Follow live during investigation
journalctl -u myapp -f

# Step 5: If traditional logs — search with context
grep -C 5 "FATAL" /var/log/myapp/app.log | tail -50
```

---

## Failure Scenarios

- `/var` partition full → service cannot write logs → all log calls fail silently → blind during incident
- `postrotate` signal missing → service writes to old rotated file after rotation → new log file always empty → monitoring misses errors
- journald configured as volatile (`/run/log/journal`) → logs lost on reboot → can't diagnose post-crash
- Log level too verbose in production → fills disk fast → rotate too slow to keep up
- No log rotation configured → 6 months of logs → `/var` fills → incident

---

## Common Mistakes

- Using `grep` for time-based filtering on journald services — just use `journalctl --since`
- Not configuring `StandardOutput=journal` in unit file → service logs go nowhere
- Forgetting `postrotate` SIGHUP → service writes to old file after rotation
- Setting log level to DEBUG in production → disk fills in hours under load
- No log rotation → discovery of problem only when disk is 100%

---

## Interview Perspective

- "Where do systemd service logs go?" → journald, read with `journalctl -u servicename`
- "How do you find all errors from the last hour for a specific service?" → `journalctl -u myapp -p err --since "1 hour ago"`
- "What happens if /var fills up?" → logs can't be written, service may fail, blind to further issues
- "After log rotation, the new log file is empty. Why?" → service still writing to old FD — needs SIGHUP to reopen log file

---

## Revision Summary

- journald = binary log store for systemd services, read with `journalctl`
- `journalctl -u <service> -p err --since "X" --until "Y"` = time + priority filtering
- Traditional logs in `/var/log` — use `grep`, `tail -f`
- Log rotation mandatory — unmanaged logs fill `/var` and blind the system
- `postrotate` in logrotate must signal service to reopen log file (SIGHUP)
- `/var` full = no more logs = invisible failures

---

## Active Recall Questions

1. Your service is managed by systemd but you can't find its logs in `/var/log`. Where are they and how do you read them?
2. How do you find all error-level logs for `myapp` from yesterday between 2pm and 4pm?
3. After log rotation, the new log file is empty but the rotated file keeps growing. What is the cause and fix?
4. What happens to your service's ability to log when `/var` fills up?
5. Why can't you use `journalctl --since` with traditional `/var/log` files?
6. What does `delaycompress` do in logrotate and why is it needed?

---

## Related Concepts

- [[Linux Filesystem Structure]]
- [[Linux Systemd and Services]]
- [[Linux Process Management]]
- [[Linux Shell Scripting and Cron]]
- [[Observability — Structured Logging]]
