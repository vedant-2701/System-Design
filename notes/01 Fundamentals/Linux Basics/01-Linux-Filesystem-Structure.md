# Linux Filesystem Structure

## Tags
#linux #operating-systems #backend-engineering #foundations

---

## Overview

- Linux uses a single unified directory tree rooted at `/`
- Everything is a file — devices, processes, kernel state, hardware
- Some directories are backed by disk; others are virtual (kernel-generated in memory)
- Understanding the layout is essential for production diagnostics and service configuration

---

## Directory Map

```
/
├── bin/      → essential binaries (ls, cp, bash)
├── sbin/     → system/root binaries (iptables, fdisk)
├── etc/      → configuration files (never runtime data)
├── var/      → variable data — grows over time (logs, caches, spools)
├── proc/     → VIRTUAL — live kernel and process state
├── sys/      → VIRTUAL — hardware and driver state
├── tmp/      → temporary files, cleared on reboot
├── var/tmp/  → temporary files, NOT cleared on reboot
├── home/     → user home directories
├── root/     → root user home
├── usr/      → user-installed programs and libraries
├── lib/      → shared libraries for /bin and /sbin
├── dev/      → device files
├── mnt/      → mount points
├── opt/      → optional/third-party software
└── run/      → runtime data (PIDs, sockets) — cleared on reboot
```

---

## /proc — Virtual Filesystem

- Not stored on disk — kernel generates contents in memory on-demand
- Reading `/proc` files is zero-cost — no disk I/O, no process attachment
- Every running process gets `/proc/<PID>/`

```
/proc/<PID>/fd/        → all open file descriptors
/proc/<PID>/status     → memory, state, threads
/proc/<PID>/cmdline    → exact startup command
/proc/<PID>/maps       → memory mappings
/proc/<PID>/limits     → per-process resource limits
/proc/<PID>/smaps_rollup → PSS (true private memory usage)

/proc/meminfo          → total/free/available RAM
/proc/cpuinfo          → CPU cores, model
/proc/loadavg          → system load averages
/proc/sys/             → tunable kernel parameters
```

### Checking FD leaks via /proc (zero overhead)
```bash
ls /proc/<PID>/fd | wc -l               # count open FDs
cat /proc/<PID>/limits | grep "open files"  # check FD limit
ls -la /proc/<PID>/fd/ | grep deleted   # files held open after deletion
```

---

## /proc/sys — Kernel Tuning

Critical parameter for backend services:

```bash
# Max accept queue size per listening socket
cat /proc/sys/net/core/somaxconn        # default: 128 (dangerously low)

# Set at runtime
sysctl -w net.core.somaxconn=1024

# Persist across reboots
echo "net.core.somaxconn=1024" >> /etc/sysctl.conf
sysctl -p
```

**Why somaxconn matters:** Incoming TCP connections queue before your app calls `accept()`. If app is slow or overloaded, queue fills. New connections are silently dropped by kernel. Default of 128 causes production incidents under traffic spikes.

---

## /tmp vs /var/tmp

| Property | /tmp | /var/tmp |
|----------|------|----------|
| Cleared on reboot | Yes | No |
| Use for | Short-lived scratch data | Data that must survive reboot |
| Risk if misused | Data loss on reboot | Fills /var partition if uncleaned |

**Production failure — wrong choice:**
- Video transcoding job writes 3-hour intermediate chunks to `/tmp` → server reboots midway → chunks gone → job in inconsistent state
- Large uncleaned files in `/var/tmp` fill `/var` → logs can no longer be written → service becomes blind

---

## Deleted File Held Open — Classic Incident

When a process opens a file and you `rm` it:
- Directory entry disappears immediately
- Inode + data blocks stay on disk until process closes the FD
- `df` shows disk full, `ls` shows nothing suspicious
- Space freed only after process releases FD or restarts

```bash
ls -la /proc/<PID>/fd/ | grep deleted   # find held-open deleted files
```

---

## Failure Scenarios

- `/var` full → service can't write logs → blind to further failures
- `somaxconn` too low → connections silently dropped under load
- Writing large temp files to `/var/tmp` without cleanup → fills `/var`
- Deleted file held open → disk space not freed → misleading `df` output

---

## Interview Perspective

- "What is `/proc`?" → Virtual filesystem, kernel-generated in memory, zero-cost reads
- "How do you check FD leaks without attaching to a process?" → `/proc/<PID>/fd/`
- "Why would `df` show disk full but `du` show low usage?" → Deleted file held open by a process
- "What is `somaxconn` and why does its default cause incidents?" → Accept queue size, default 128 drops connections silently

---

## Revision Summary

- `/proc` is virtual — kernel generates it in memory, nothing on disk
- `/proc/<PID>/fd/` = open FDs, zero overhead, use for leak detection
- `somaxconn` default 128 = accept queue cap, tune to 1024+ for production
- `/tmp` cleared on reboot, `/var/tmp` is not — wrong choice causes data loss or disk fill
- Deleted files stay on disk until holding process closes its FD
- `/var` full = logs stop = blind production system

---

## Active Recall Questions

1. What makes `/proc` different from a regular filesystem?
2. How do you check if a process is leaking file descriptors without using strace?
3. `df` shows disk is 100% full. `ls /var/log` shows only small files. What else could be consuming space and how do you find it?
4. What happens to TCP connections when `somaxconn` is too low under a traffic spike?
5. A 3-hour batch job uses `/tmp` for intermediate files. A server reboot happens at hour 2. What's the failure and how do you fix it?
6. How do you make a kernel parameter change survive a reboot?

---

## Related Concepts

- [[Linux Process Management]]
- [[Linux Permissions and Ownership]]
- [[Linux Log Inspection]]
- [[Linux Networking Tools]]
- [[Linux Systemd and Services]]
- [[File Descriptors and epoll]]
- [[Virtual Memory and Paging]]
