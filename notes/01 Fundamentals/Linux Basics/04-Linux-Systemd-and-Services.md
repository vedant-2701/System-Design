# Linux Systemd and Services

## Tags
#linux #systemd #backend-engineering #operations #foundations

---

## Overview

- systemd is PID 1 — the init system on modern Linux, parent of all processes
- Manages service lifecycle: start, stop, restart, enable on boot, dependency ordering
- Reads unit files (`.service`) that declare how a service should run
- Integrates with journald for centralized logging
- Replaces old SysV init scripts with declarative configuration

---

## Unit File Structure

Location: `/etc/systemd/system/myapp.service`

```ini
[Unit]
Description=My Backend Service
After=network.target postgresql.service
Requires=postgresql.service

[Service]
Type=simple
User=appuser
WorkingDirectory=/opt/myapp
ExecStart=/opt/myapp/bin/server --port 8080
ExecReload=/bin/kill -HUP $MAINPID
Restart=on-failure
RestartSec=5s
StartLimitIntervalSec=60s
StartLimitBurst=3
StandardOutput=journal
StandardError=journal
Environment=APP_ENV=production
EnvironmentFile=/etc/myapp/env

[Install]
WantedBy=multi-user.target
```

---

## Key Unit File Directives

### [Unit] — Dependencies

| Directive | Meaning |
|-----------|---------|
| `After=X` | Start after X is running (ordering only) |
| `Requires=X` | Hard dependency — if X dies, this dies too |
| `Wants=X` | Soft dependency — try to start X but don't fail if unavailable |

**`After` vs `Requires`:** `After` controls order. `Requires` controls lifecycle coupling. Use both together for true hard dependencies.

### [Service] — Process behavior

**Type options:**

| Type | Meaning |
|------|---------|
| `simple` | Process stays in foreground — PID is main process |
| `forking` | Process forks then parent exits (old daemon style) |
| `notify` | Process sends systemd a ready signal when fully initialized |

**Getting `Type` wrong causes subtle bugs:** With wrong type, systemd may mark service as "active" before it's actually ready to serve traffic. Downstream services that `After=` this one start too early.

**Restart policy:**

| Value | Behavior |
|-------|----------|
| `no` | Never restart |
| `always` | Restart regardless of exit code |
| `on-failure` | Restart only on non-zero exit |
| `on-abnormal` | Restart on signal/timeout |

**`RestartSec=5s`** — mandatory in production. Without it, a crashing service restarts instantly, loops thousands of times per minute, hammering the system.

**Crash loop prevention:**
```ini
StartLimitIntervalSec=60s
StartLimitBurst=3
```
If service crashes and restarts more than 3 times in 60 seconds, systemd marks it failed and stops retrying. Forces human intervention rather than infinite crash-loop.

### [Install] — Boot behavior

```ini
WantedBy=multi-user.target   # start when system reaches normal multi-user mode
```

Targets are system states:
- `multi-user.target` → normal CLI mode
- `graphical.target` → GUI mode
- `network.target` → network is available

---

## Essential systemctl Commands

```bash
# Service control
systemctl start myapp
systemctl stop myapp
systemctl restart myapp
systemctl reload myapp          # sends SIGHUP — reloads config, no downtime

# Boot behavior
systemctl enable myapp          # start on boot (creates symlink)
systemctl disable myapp         # remove from boot

# Inspection
systemctl status myapp          # state, PID, last log lines, restart count
systemctl is-active myapp       # returns: active / inactive
systemctl is-enabled myapp      # returns: enabled / disabled

# After editing unit file — MANDATORY
systemctl daemon-reload
systemctl restart myapp

# List services
systemctl list-units --type=service
systemctl list-units --type=service --state=failed
```

---

## Common Production Mistakes

**Forgetting `daemon-reload` after editing unit file:**
Edit `/etc/systemd/system/myapp.service` → run `systemctl restart myapp` → change doesn't apply. systemd cached the old file. Always run `systemctl daemon-reload` first.

**No `RestartSec`:**
New deployment has bug → crashes instantly → restarts → crashes → thousands of loops per minute → system under unnecessary load.

**Wrong `Type`:**
Using `Type=simple` for a service that forks. systemd watches the wrong PID. When the parent exits (intentionally), systemd thinks the service died and restarts it.

**`Requires` without `After`:**
Service starts in parallel with its dependency. The dependency isn't ready yet. Service fails on first connection attempt.

---

## Systemd Timers vs Cron

| Feature | Cron | Systemd Timer |
|---------|------|---------------|
| Logging | Manual redirection | Automatic via journald |
| Monitoring | Manual | `systemctl status` |
| Dependencies | None | Full systemd dependency system |
| Missed job handling | Skipped | Can catch up with `Persistent=true` |
| Accuracy | Minute-level | Microsecond |
| Complexity | Simple | More setup |

**Rule:** Use cron for simple tasks. Use systemd timers for critical production jobs where observability and dependency management matter.

---

## Failure Scenarios

- Crash loop without `StartLimitBurst` → service hammers system, alerts never fire clearly
- `Requires=postgresql.service` but postgres takes 30s to start → app fails first connection → need both `After=` and `Requires=`
- `StandardOutput=journal` not set → service logs go nowhere, blind during incidents
- Unit file edited but `daemon-reload` not run → old config in effect, confusing behavior
- Service marked active but not yet ready (`Type=simple` on forking service) → downstream starts too early

---

## Interview Perspective

- "How does a Linux service restart automatically after crashing?" → systemd `Restart=on-failure` + `RestartSec`
- "How do you prevent a crash loop?" → `StartLimitBurst` + `StartLimitIntervalSec`
- "What is the difference between `Requires` and `Wants`?" → hard vs soft dependency
- "You edited a systemd unit file. What must you run before restarting?" → `systemctl daemon-reload`
- "How do you reload nginx config without dropping connections?" → `systemctl reload nginx` or `kill -HUP <PID>`

---

## Revision Summary

- systemd = PID 1, manages all service lifecycle on modern Linux
- Unit files declare: how to start, dependencies, restart behavior, user, logging
- `After` = ordering, `Requires` = lifecycle coupling — need both for hard dependencies
- `Restart=on-failure` + `RestartSec=5s` = auto-restart with delay
- `StartLimitBurst` = crash loop prevention — systemd stops retrying after N crashes in window
- Always run `systemctl daemon-reload` after editing unit files
- `Type=simple/forking/notify` — wrong type causes subtle lifecycle bugs
- systemd timers > cron for production jobs that need observability

---

## Active Recall Questions

1. What is the difference between `After=` and `Requires=` in a unit file?
2. Your service is configured with `Restart=on-failure` but no `RestartSec`. A bad deployment causes it to crash on startup. What happens to the system?
3. You edit a unit file and restart the service. The change doesn't take effect. Why?
4. What does `StartLimitBurst=3` with `StartLimitIntervalSec=60s` do?
5. When would you use `Type=notify` instead of `Type=simple`?
6. What is `WantedBy=multi-user.target` doing in the `[Install]` section?

---

## Related Concepts

- [[Linux Process Management]]
- [[Linux Log Inspection]]
- [[Linux Shell Scripting and Cron]]
- [[Linux Signals]]
- [[Operating Systems — Processes vs Threads]]
