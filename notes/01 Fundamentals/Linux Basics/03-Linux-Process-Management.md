# Linux Process Management

## Tags
#linux #operating-systems #backend-engineering #foundations

---

## Overview

- Every running program is a process with a unique PID
- Processes have state, resource usage, parent-child relationships, and signal handlers
- Linux provides `/proc`, `ps`, `top`, `kill` to inspect and control processes
- Understanding process lifecycle and signals is critical for production service management

---

## Process States

| State | Symbol | Meaning |
|-------|--------|---------|
| Running | `R` | Actively using CPU |
| Sleeping | `S` | Waiting for event (interruptible) |
| Uninterruptible sleep | `D` | Waiting for I/O — cannot be killed |
| Zombie | `Z` | Finished but not yet reaped by parent |
| Stopped | `T` | Paused via SIGSTOP |

**D state is critical:** Process is deep in a kernel I/O operation. `kill -9` has no effect. Only fix is resolving the underlying I/O (unmount hung NFS, fix disk). If unresolvable, reboot required.

---

## Zombie Processes

**Why they exist:** When a child process exits, the kernel preserves its exit code in the process table until the parent calls `wait()` to collect it. This waiting state = zombie.

**Why they accumulate:** Parent never calls `wait()` — either buggy code or parent itself crashed.

**Impact:** Zombies consume no CPU or memory but hold a PID slot. PID exhaustion → kernel cannot spawn new processes → system failure.

**Diagnosis and cleanup:**
```bash
ps aux | grep 'Z'                  # find zombies (Z in STAT column)
kill -SIGCHLD <parent_PID>         # signal parent to call wait()
```

If parent is dead: zombies get reparented to PID 1 (systemd), which calls `wait()` and cleans them automatically.

---

## Memory: VSZ vs RSS vs PSS

| Metric | Meaning | Where |
|--------|---------|-------|
| VSZ (VIRT) | Virtual memory reserved — may never be used | `ps`, `top` |
| RSS (RES) | Physical RAM currently used by this process | `ps`, `top` |
| PSS | Proportional share of shared memory — true private footprint | `/proc/<PID>/smaps_rollup` |

**Why RSS overstates:** Shared libraries (e.g. JVM) counted in RSS of every process using them, but physically exist once in RAM.

**Why VSZ overstates:** Demand paging — `malloc(1GB)` reserves address space but kernel only allocates physical pages when written to.

**Production rule:** Use PSS for true per-process memory accounting. RSS for quick checks.

```bash
cat /proc/<PID>/smaps_rollup | grep Pss
```

---

## ps — Process Snapshot

```bash
ps aux                          # all processes, all users
ps aux | grep nginx             # find specific process
ps auxf                         # process tree with parent-child relationships
ps -T -p <PID>                  # threads of a specific process
ps aux --sort=-%mem | head -10  # top memory consumers
ps aux --sort=-%cpu | head -10  # top CPU consumers
```

**Key columns:**
- `STAT` → process state (S, R, Z, D)
- `VSZ` → virtual memory
- `RSS` → physical RAM
- `%CPU`, `%MEM` → current utilization

---

## top — Live Process Monitor

```bash
top
# Keyboard shortcuts:
# M → sort by memory
# P → sort by CPU
# 1 → show individual CPU cores
# k → kill a process
# q → quit
```

### Header interpretation

```
load average: 0.52, 0.58, 0.71
              └1min  └5min  └15min
```

**Load average** = number of processes running or waiting for CPU.

- Saturation point = number of CPU cores (check with `nproc`)
- 1min > 15min → load increasing → investigate now
- 1min < 15min → load decreasing → was worse, calming down

```
%Cpu: 2.1 us  0.5 sy  96.9 id  0.3 wa
```

| Field | Meaning | High value implies |
|-------|---------|-------------------|
| `us` | User space (app code) | CPU-bound application |
| `sy` | Kernel/syscalls | Excessive system calls |
| `id` | Idle | System is not overloaded |
| `wa` | I/O wait | I/O bottleneck — adding CPU won't help |

**High `wa` is critical:** CPU is idle waiting for disk/network. Adding more CPUs won't help. Fix the I/O bottleneck.

---

## Signals

```bash
kill <PID>           # sends SIGTERM (default)
kill -15 <PID>       # explicit SIGTERM
kill -9 <PID>        # SIGKILL — last resort only
kill -1 <PID>        # SIGHUP — reload config
kill -SIGCHLD <PID>  # signal parent to reap zombies

pkill nginx          # kill by name
pgrep nginx          # get PID by name
```

### Key signals

| Signal | Number | Catchable | Behavior |
|--------|--------|-----------|----------|
| SIGTERM | 15 | Yes | Graceful shutdown request |
| SIGKILL | 9 | No | Immediate kernel kill — no cleanup |
| SIGHUP | 1 | Yes | Convention: reload config |
| SIGINT | 2 | Yes | Ctrl+C interrupt |
| SIGCHLD | 17 | Yes | Child state changed |
| SIGSTOP | 19 | No | Pause process |
| SIGCONT | 18 | Yes | Resume paused process |

### SIGTERM vs SIGKILL — production discipline

**Always try SIGTERM first:**
```bash
kill -15 <PID>
sleep 5
# if still running:
kill -9 <PID>
```

**Why `kill -9` is last resort:**
- No cleanup hooks run
- In-flight requests dropped
- DB connections not returned to pool
- Open files may be corrupted
- Downstream services get abrupt disconnects

### SIGHUP convention

Originally: terminal hangup. Now universally repurposed as "reload config without restarting":
```bash
kill -HUP <nginx_PID>    # nginx re-reads config, zero downtime
```

Process catches SIGHUP, re-reads config file, applies changes, continues serving.

---

## Production Diagnostic Flow

```
systemctl status myapp          # Step 1: is it running?
    │
    ├── Not running → journalctl -u myapp -n 100   # why did it crash?
    │
    └── Running but broken →
            ss -tlnp sport = :8080                  # is it listening?
            top                                     # CPU? wa%? load trend?
            ls /proc/<PID>/fd | wc -l               # FD leak?
            ss -tan | awk '{print $1}' | sort | uniq -c   # connection states
            journalctl -u myapp --since "1 hour ago" -p err
```

---

## Failure Scenarios

- High `wa%` → I/O bottleneck, not CPU — adding cores won't help
- D-state process → kernel I/O hang — unkillable, resolve I/O or reboot
- Zombie accumulation → PID exhaustion → no new processes can spawn
- SIGKILL on misbehaving service → incomplete writes, leaked connections, corrupted state
- Load 1min >> 15min on production server → sudden traffic spike or runaway process

---

## Interview Perspective

- "What is a zombie process and why does it exist?" → exit code preservation until parent calls `wait()`
- "What is the difference between SIGTERM and SIGKILL?" → catchable/graceful vs uncatchable/immediate
- "High load average but low CPU usage — what's happening?" → high `wa%`, I/O bottleneck
- "VSZ is 2GB but RSS is 200MB — is this a memory leak?" → No, VSZ includes reserved but untouched memory (demand paging)
- "A process is in D state. You run kill -9. What happens?" → Nothing, D state is unkillable

---

## Revision Summary

- Process states: R (running), S (sleeping), D (uninterruptible I/O — unkillable), Z (zombie)
- Zombie = dead process waiting for parent to call `wait()` — holds PID, no resources
- VSZ = reserved (may never be used), RSS = physical RAM, PSS = true private footprint
- Load average saturation = number of cores; 1min > 15min = increasing load
- High `wa%` = I/O bottleneck — CPU is idle waiting, adding cores doesn't help
- SIGTERM first (catchable, graceful), SIGKILL only as last resort
- SIGHUP = reload config convention for production daemons

---

## Active Recall Questions

1. Load average is `4.2, 1.1, 0.9` on a 2-core machine. What is happening and what do you check next?
2. A process is in D state. Why can't you kill it with `kill -9`?
3. What is the difference between VSZ and RSS? Which is more useful and why?
4. Your service crashed and left 500 zombie processes. How do you clean them up without rebooting?
5. Why is `kill -9` a last resort in production?
6. What does high `wa%` in `top` indicate and what should you investigate?

---

## Related Concepts

- [[Linux Filesystem Structure]]
- [[Linux Systemd and Services]]
- [[Linux Log Inspection]]
- [[Linux Networking Tools]]
- [[File Descriptors and epoll]]
- [[Concurrency — Thread Pools]]
- [[Operating Systems — Context Switching]]
