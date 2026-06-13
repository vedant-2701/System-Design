# linux-service-lab

A production-style reference implementation demonstrating:
- Systemd service lifecycle management
- Process health monitoring beyond "is it running"
- Log rotation with correct FD handoff
- Idempotent installation

---

## Problem Statement

Running a backend service reliably on Linux requires more than writing a binary and starting it. Three distinct problems must be solved:

1. **Lifecycle management** — Who starts the service on boot? Who restarts it on crash? What prevents infinite crash loops?
2. **Health monitoring** — systemd knows if a process is alive. It does not know if the application layer is deadlocked, leaking FDs, or silently failing.
3. **Log management** — Logs grow indefinitely. `/var` fills. The service can no longer write logs. You become blind exactly when you need visibility most.

This implementation solves all three with standard Linux tooling.

---

## Architecture

```
┌─────────────────────────────────────────┐
│              systemd (PID 1)            │
│                                         │
│  myapp.service ←── manages lifecycle    │
│       │                                 │
│       ▼                                 │
│  /opt/myapp/bin/server                  │
│       │  writes to                      │
│       ▼                                 │
│  /var/log/myapp/myapp.log               │
│       │  rotated by                     │
│       ▼                                 │
│  logrotate (daily, postrotate SIGHUP)   │
│                                         │
│  myapp-monitor.timer ─fires every 60s─► │
│  myapp-monitor.service                  │
│       │  runs                           │
│       ▼                                 │
│  monitor-myapp.sh                       │
│       │  checks                         │
│       ├── systemd active state          │
│       ├── HTTP /health → 200            │
│       ├── FD count vs limit             │
│       ├── RSS vs threshold              │
│       └── restart count                 │
└─────────────────────────────────────────┘
```

---

## Component Breakdown

### cmd/server/main.go

A minimal HTTP service exposing:

| Endpoint | Purpose |
|----------|---------|
| `/health` | Returns `{"status":"ok","uptime":"..."}` — used by monitor and load balancers |
| `/crash` | Exits with code 1 — used to test systemd restart behavior |
| `/work` | Runs a CPU loop — makes `top` and memory checks meaningful |
| `/log` | Writes a log line at a given level — used to test log rotation |

**Design decisions:**

- Binds to `0.0.0.0` explicitly — avoids the silent `127.0.0.1` misconfiguration that blocks external traffic
- Graceful shutdown on SIGTERM with 15-second drain — matches `TimeoutStopSec` in unit file; in-flight requests complete before process exits
- Logs structured `key=value` lines — parseable by `grep` and log aggregators without regex gymnastics
- Logs to file AND stderr — file for logrotate, stderr for journald capture of startup failures before log file opens

### systemd/myapp.service

Key directives and their reasoning:

| Directive | Value | Why |
|-----------|-------|-----|
| `Restart=on-failure` | on-failure | Restart on crash, not on clean `systemctl stop` |
| `RestartSec=5s` | 5s | Prevents instant crash loops hammering the system |
| `StartLimitBurst=3` | 3 | Forces human intervention after 3 crashes in 60s |
| `TimeoutStopSec=15` | 15s | Matches Go shutdown drain window |
| `LimitNOFILE=65536` | 65536 | Raise FD limit beyond OS default of 1024 for connection-heavy services |
| `MemoryMax=512M` | 512M | OOM before the service takes down the whole machine |
| `PrivateTmp=true` | true | Service gets isolated `/tmp` — prevents temp file conflicts with other services |
| `ProtectSystem=strict` | strict | Filesystem is read-only except explicitly whitelisted paths |

### scripts/monitor-myapp.sh

Checks in dependency order:

1. **Service active** — if down, skip remaining checks (they'll all fail misleadingly)
2. **Health endpoint** — verifies application layer, not just process existence
3. **FD usage** — reads `/proc/<pid>/fd` and `/proc/<pid>/limits`; alerts above 80%
4. **RSS memory** — reads `/proc/<pid>/status`; alerts above configured threshold
5. **Restart count** — reads `NRestarts` from systemd; catches crash loops between monitor runs

**Critical design choice — the monitor does NOT restart the service.**

Two systems managing restarts creates race conditions:
- Monitor restarts service at t=0
- systemd also tries to restart at t=0 (its RestartSec elapsed)
- Both succeed — now two instances of the service are running
- Both bind to the same port — one fails with "address already in use"
- systemd marks service as failed
- Monitor doesn't know systemd is involved
- Incident is now harder to diagnose than if only one system was responsible

The monitor's job is detection and alerting. systemd's job is healing.

### logrotate.d/myapp

Two configs in one file — application log and monitor alert log rotated separately:

**Application log:** daily, 30 days, `delaycompress`

`delaycompress` is not just an optimization — it's correctness. After rotation the old file is renamed but not yet compressed. The service may still have an FD open to it. Compressing a file that's being written to corrupts the gzip stream. `delaycompress` delays one cycle, by which time the postrotate SIGHUP has caused the service to reopen its log FD on the new file.

**postrotate SIGHUP flow:**
```
1. logrotate renames myapp.log → myapp.log.1
2. logrotate creates new empty myapp.log
3. postrotate fires: systemctl reload myapp.service
4. systemd sends SIGHUP to main process
5. Service closes old FD, opens new myapp.log
6. Writes now go to new file
7. Next rotation cycle: myapp.log.1 is now safe to compress
```

Without step 3-6: service keeps writing to `.log.1`, new `myapp.log` stays empty, monitoring sees no log activity, space is not freed because the FD is still open.

---

## Why This Approach

### Systemd timer over cron for monitoring

| Concern | Cron | Systemd timer |
|---------|------|---------------|
| Logs | Manual redirection | Automatic via journald |
| Missed runs | Silently skipped | Recovered with `Persistent=true` |
| Failure visibility | grep in log file | `systemctl status myapp-monitor.service` |
| Dependency ordering | None | `After=myapp.service` |

### Structured log format (key=value)

```
timestamp=2024-01-04T14:00:01Z level=INFO service=myapp check=fd_usage status=OK pid=1234 open_fds=42 fd_limit=65536 usage_percent=0
```

Advantages over free-text logging:
- `grep "status=FAIL"` finds all failures across all checks
- `grep "check=fd_usage"` isolates FD check history
- Parseable by logstash, fluentd, cloudwatch without custom regex
- No ambiguity when values contain spaces (use quotes) or special characters

---

## Alternatives Considered

### Custom log rotation script instead of logrotate

**Rejected.** logrotate handles edge cases that take years to discover: atomic renames, permission preservation, compressed file naming, signal delivery. Writing a correct log rotation script from scratch introduces more risk than it eliminates.

### Health check inside the systemd unit (sd_notify)

Could use `Type=notify` and have the Go binary call `sd_notify(READY=1)` after the HTTP server is listening. This tells systemd the service is truly ready, not just "the exec started."

**Not implemented here** to keep the example focused, but in production this is valuable — it prevents systemd from marking dependent services as startable before the HTTP listener is actually up.

### Prometheus metrics instead of custom monitor script

A Prometheus exporter would expose FD count, memory, and restart count as time-series metrics with alerting via Alertmanager — far more powerful than a shell script.

**Not implemented** because it introduces a metrics stack dependency. The shell script approach works with zero external dependencies and is appropriate for simpler deployments. At scale, replace with Prometheus.

---

## Edge Cases

| Scenario | Handled? | How |
|----------|----------|-----|
| Service not started yet | Yes | Monitor checks service state first, skips dependent checks |
| PID changes after restart | Yes | Monitor re-reads PID from systemd on every run |
| `/proc/<pid>/fd` not readable | Yes | `|| true` skip with WARN log — doesn't fail monitor run |
| Log file missing at rotation | Yes | `missingok` in logrotate config |
| Install run twice | Yes | Idempotent — `id user || useradd`, config skip if exists |
| Binary missing at install time | Yes | Explicit check with helpful error message |
| Service crashes during shutdown drain | Yes | `context.WithTimeout(15s)` forces exit after drain window |

---

## Production Considerations

### What's missing for a real production service

1. **Alerting** — The monitor writes to a log file. In production, failures should page on-call via PagerDuty, OpsGenie, or a dead man's switch pattern.
2. **sd_notify** — `Type=notify` ensures systemd knows when the HTTP listener is actually ready.
3. **Structured logging library** — Replace `log.Printf` with `zerolog` or `zap` for JSON-structured logs with consistent fields.
4. **Metrics endpoint** — `/metrics` in Prometheus format for Grafana dashboards.
5. **Config validation on startup** — Fail fast with a clear error if required env vars are missing.

### Testing this lab

```bash
# Build the binary
go build -o server ./cmd/server/ && echo "Build OK" && ls -lh server

# Run locally (without systemd)
APP_PORT=8080 APP_LOG_FILE=/tmp/myapp.log ./server

# Test endpoints
curl http://localhost:8080/health
curl http://localhost:8080/work?n=5000000
curl "http://localhost:8080/log?level=error&msg=test-error"
curl http://localhost:8080/crash   # triggers exit(1) — test restart

# Test monitor script manually (service must be running)
SERVICE_NAME=myapp \
HEALTH_URL=http://127.0.0.1:8080/health \
MAX_RSS_MB=400 \
ALERT_LOG=/tmp/monitor-alerts.log \
bash scripts/monitor-myapp.sh

# Test log rotation config syntax
logrotate --debug logrotate.d/myapp

# Install (requires root, systemd)
sudo bash scripts/install.sh --start
systemctl status myapp.service
journalctl -u myapp.service -f
journalctl -u myapp-monitor.service -n 20
```

---

## File Structure

```
linux-service-lab/
├── cmd/server/
│   └── main.go                   # Go backend service
├── scripts/
│   ├── install.sh                # Idempotent install script
│   └── monitor-myapp.sh          # Process health monitor
├── systemd/
│   ├── myapp.service             # Service lifecycle unit
│   ├── myapp-monitor.service     # Monitor job unit (run by timer)
│   └── myapp-monitor.timer       # Timer schedule
├── logrotate.d/
│   └── myapp                     # Log rotation config
├── go.mod
└── README.md
```