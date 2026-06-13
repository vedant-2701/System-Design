# Linux Shell Scripting and Cron

## Tags
#linux #shell #automation #backend-engineering #foundations

---

## Overview

- Shell scripts automate repetitive operational tasks — backups, cleanup, health checks, deployments
- Cron schedules scripts on a time-based trigger
- Production shell scripts require defensive flags to prevent silent failures
- Cron has environment differences from interactive shells — a common source of bugs

---

## Production Script Header

```bash
#!/bin/bash
set -euo pipefail
```

Every production shell script should start with this.

| Flag | Behavior without flag | Behavior with flag |
|------|----------------------|-------------------|
| `-e` | Script continues after failed command | Script exits immediately on any non-zero exit code |
| `-u` | Unset variables treated as empty string | Script exits on reference to unset variable |
| `-o pipefail` | Only last command in pipe determines exit code | Any command in pipe failing causes pipe to fail |

**Why this matters in production:**

Without `-e`:
```bash
cd /app/data        # fails silently (dir doesn't exist)
rm -rf *            # runs in current directory — disaster
```

With `set -e`: script stops at `cd` failure. Disaster avoided.

### Handling intentional failures with -e

`-e` causes problems when a command legitimately returns non-zero:

```bash
# grep returns exit code 1 when no match — stops script with -e
grep "ERROR" /var/log/app.log

# Fix: suppress exit code
grep "ERROR" /var/log/app.log || true

# Fix: use conditional
if grep -q "ERROR" /var/log/app.log; then
    echo "errors found"
fi
```

### Debug flag

```bash
set -x              # trace: print each command to stderr before executing
bash -x script.sh   # debug without modifying script
```

Use during debugging, remove before committing. Never leave `-x` on in production — leaks sensitive values to logs.

---

## Cron Syntax

```
* * * * * /path/to/command
│ │ │ │ │
│ │ │ │ └── day of week (0-7, Sunday=0 or 7)
│ │ │ └──── month (1-12)
│ │ └────── day of month (1-31)
│ └──────── hour (0-23)
└────────── minute (0-59)
```

### Common patterns

```bash
* * * * *       # every minute
*/5 * * * *     # every 5 minutes
0 2 * * *       # daily at 2am
0 9 * * 1       # every Monday at 9am
*/15 * * * 1-5  # every 15 minutes on weekdays
0 0 1 * *       # first day of every month at midnight
0 * * * 1-5     # every hour on weekdays
```

### Crontab management

```bash
crontab -e              # edit current user's crontab
crontab -l              # list current crontab
crontab -u appuser -e   # edit another user's crontab
```

---

## Production Cron Mistakes

### Mistake 1: No output redirection

```bash
# Bad — output lost or sent to local mail spool nobody reads
0 2 * * * /opt/scripts/backup.sh

# Good — capture both stdout and stderr
0 2 * * * /opt/scripts/backup.sh >> /var/log/backup.log 2>&1
```

`2>&1` redirects stderr to stdout so both go to the same log file. Without this, errors are silently swallowed.

### Mistake 2: Missing PATH

Cron runs with a minimal environment — `/usr/local/bin`, custom tool paths often missing.

```bash
# Bad — tool may not be found in cron's PATH
0 2 * * * python3 /opt/scripts/backup.py

# Good — absolute path
0 2 * * * /usr/bin/python3 /opt/scripts/backup.py

# Or set PATH at top of crontab
PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin
```

### Mistake 3: Overlapping jobs

Job takes 10 minutes but runs every 5 minutes. Two instances run simultaneously, fight over same files/DB rows.

**Fix — lock file pattern:**

```bash
#!/bin/bash
set -euo pipefail

LOCKFILE=/tmp/myjob.lock

if [ -f "$LOCKFILE" ]; then
    echo "Job already running at $(date), exiting"
    exit 0
fi

trap "rm -f $LOCKFILE" EXIT    # remove lock even on crash
touch $LOCKFILE

# actual job
/opt/scripts/process_data.sh
```

`trap ... EXIT` is critical — without it, a crash leaves the lockfile forever, permanently blocking future runs.

### Mistake 4: Silent failures unnoticed for weeks

```bash
# Add explicit failure logging
0 2 * * * /opt/scripts/backup.sh >> /var/log/backup.log 2>&1 \
    || echo "BACKUP FAILED $(date)" >> /var/log/backup-errors.log
```

Or use systemd timers for critical jobs — failures visible via `systemctl status`.

---

## Systemd Timers vs Cron

For critical production jobs, prefer systemd timers:

```ini
# /etc/systemd/system/backup.timer
[Unit]
Description=Daily Backup Timer

[Timer]
OnCalendar=daily
Persistent=true       # run missed jobs on next boot

[Install]
WantedBy=timers.target
```

```ini
# /etc/systemd/system/backup.service
[Unit]
Description=Backup Job

[Service]
Type=oneshot
ExecStart=/opt/scripts/backup.sh
```

```bash
systemctl enable backup.timer
systemctl start backup.timer
systemctl status backup.timer    # see last run, next run, exit status
journalctl -u backup.service     # full logs
```

`Persistent=true` — if system was down when job should have run, run it on next boot. Cron skips missed jobs entirely.

---

## Tradeoffs

| Concern | Cron | Systemd Timer |
|---------|------|---------------|
| Setup complexity | Simple | More boilerplate |
| Logging | Manual redirection | Automatic via journald |
| Missed job recovery | No | Yes (`Persistent=true`) |
| Dependency management | None | Full systemd |
| Monitoring | Manual log watching | `systemctl status` |
| Production visibility | Low | High |

**Rule:** Cron for simple, low-stakes automation. Systemd timers for anything where missed runs or failures have production impact.

---

## Failure Scenarios

- No `set -e` → script continues after critical failure → corrupted state
- No `set -u` → typo in variable name treated as empty string → wrong path, wrong behavior
- No `pipefail` → `grep ... | process_data.sh` — grep fails silently, `process_data.sh` runs on empty input
- No lock file → overlapping job instances → data corruption or race conditions
- No output redirection in cron → errors silently discarded → failures invisible for weeks
- PATH mismatch in cron → script works in terminal, silently fails in cron
- No `trap` on lockfile → crash leaves lock forever → all future runs skip

---

## Interview Perspective

- "What does `set -euo pipefail` do and why is it important?" → defensive flags preventing silent failures
- "Your cron job works in terminal but fails silently in cron. Why?" → PATH difference, missing absolute paths
- "Two instances of your cron job are running simultaneously and corrupting data. How do you fix it?" → lock file with `trap` cleanup
- "How do you handle a command that legitimately fails in a script with `set -e`?" → `|| true` or conditional

---

## Revision Summary

- `set -euo pipefail` = mandatory production script header — prevents silent failures
- `-e` stops on failure, `-u` stops on unset variable, `-o pipefail` catches pipe failures
- `grep || true` or `if grep -q ...` to handle intentional non-zero exits with `-e`
- Cron syntax: minute hour dom month dow
- Cron has minimal PATH — always use absolute paths or set PATH explicitly in crontab
- Redirect cron output: `>> /var/log/job.log 2>&1` — otherwise errors are invisible
- Lock file + `trap` prevents overlapping job instances
- Systemd timers > cron for production-critical jobs (observability, missed run recovery)

---

## Active Recall Questions

1. What happens in a script without `set -e` if `cd /nonexistent` is followed by `rm -rf *`?
2. A cron job runs fine in your terminal but does nothing when scheduled. What is the most likely cause?
3. Your backup job runs every 5 minutes but takes 7 minutes to complete. What's the risk and how do you fix it?
4. What does `trap "rm -f $LOCKFILE" EXIT` do and why is it necessary?
5. A script uses `grep "ERROR" log.txt | wc -l`. grep finds no matches. Without `pipefail`, does the script continue or exit? What about with `pipefail`?
6. When would you choose a systemd timer over cron?

---

## Related Concepts

- [[Linux Systemd and Services]]
- [[Linux Log Inspection]]
- [[Linux Process Management]]
- [[Linux Permissions and Ownership]]
- [[Background Jobs and Queues]]
