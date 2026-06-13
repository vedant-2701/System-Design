# Operating Systems

## Tags
#os #foundations #processes #threads #memory #scheduling #system-design

---

## Overview

- An OS manages hardware resources and provides abstractions (processes, files, memory) to user programs
- Core abstractions: **Process** (isolated execution environment), **Virtual Memory** (address space illusion), **File Descriptor** (uniform I/O interface), **Scheduler** (CPU time allocation)
- Every abstraction exists to solve a concrete problem — isolation, safety, fairness, performance

---

## Processes

### What A Process Is

A process is not just code — it is a complete isolated execution environment created by the OS.

```text
Process
├── Text Segment       → compiled instructions (read-only)
├── Data Segment       → initialized global/static variables
├── BSS Segment        → uninitialized global/static variables (zero-filled)
├── Heap               → dynamic allocation (grows upward ↑)
├── Stack              → function frames, local vars (grows downward ↓)
├── File Descriptors   → open files, sockets, pipes
├── Page Table         → virtual → physical address mapping
└── PCB                → OS metadata for this process
```

### Process Control Block (PCB)

Stored in **kernel memory (RAM)** — never disk.

```text
PCB contains:
├── PID
├── Process State        → Running / Ready / Waiting / Terminated
├── Program Counter      → next instruction address
├── CPU Registers        → full register snapshot
├── Memory Info          → page table pointer, memory limits
├── File Descriptors     → open file table pointers
├── Scheduling Info      → priority, CPU time used
└── Parent PID
```

### Process States

```text
New → Ready → Running → Waiting (I/O / lock) → Ready → ...
                ↓
           Terminated
```

- **Ready** → has everything except CPU
- **Waiting/Blocked** → needs I/O, lock, or event — not scheduled
- **Running** → on CPU right now

### Why Process Isolation Exists

| Concern | Without Isolation |
|---|---|
| Correctness | Process B corrupts Process A's memory — bugs impossible to trace |
| Security | Any process reads any memory — secrets exposed |
| Stability | One crash brings down unrelated processes |
| Resource Control | Processes monopolize CPU, memory, disk freely |

**Real example:** Chrome runs each tab as a separate process — crash or exploit in one tab cannot touch another.

---

## Threads

### What A Thread Is

A thread is an execution unit **inside** a process. Multiple threads share the process's memory space but each has its own:

```text
Per-thread (private):
├── Stack
└── CPU Registers / Program Counter

Shared across all threads:
├── Heap
├── Text Segment
├── Data / BSS
└── File Descriptors
```

### Concurrency vs Parallelism

| Concept | Meaning |
|---|---|
| Concurrency | Multiple tasks making progress — not necessarily simultaneously |
| Parallelism | Multiple tasks executing at the exact same instant on different cores |

Threads on a single core → **concurrency only**.
Threads on multiple cores → **true parallelism**.

### Why Shared Memory Causes Problems

**Race Condition — Classic Example:**

```text
balance = 1000

Thread 1 (deposit 500):      Thread 2 (withdraw 200):
read balance → 1000           read balance → 1000
add 500 → 1500                subtract 200 → 800
write → 1500                  write → 800

Final: 800   ← WRONG. Should be 1300.
```

Non-deterministic — may occur once in 10,000 requests. Extremely hard to reproduce.

### Synchronization Problems

| Problem | Description |
|---|---|
| Race Condition | Shared state modified without synchronization |
| Deadlock | Thread A waits for Thread B's lock; Thread B waits for Thread A's lock |
| Livelock | Both threads respond to each other but neither makes progress |
| Starvation | Thread never acquires lock because others keep getting it first |
| Priority Inversion | Low-priority thread holds lock needed by high-priority thread |

**Priority Inversion — Mars Pathfinder 1997:**
Low-priority task held mutex. High-priority task blocked. Medium-priority tasks ran freely. Watchdog detected "hang" → spacecraft reset repeatedly. Fixed via priority inheritance.

### Process vs Thread — Engineering Decision

| Dimension | Process | Thread |
|---|---|---|
| Memory isolation | Full | None — shared heap |
| Creation cost | Expensive (full address space) | Cheap (stack + registers only) |
| Context switch cost | Higher (TLB flush) | Lower (same page tables) |
| Failure isolation | Crash doesn't affect others | One crash can kill entire process |
| Communication | IPC — pipes, sockets, shared memory | Direct via shared memory |
| Security boundary | Yes | No |
| Use case | Isolation critical (browser tabs, microservices) | Shared work, parallelism |

**Rule:** Choose threads when sharing is necessary and synchronization is manageable. Choose processes when isolation, fault tolerance, or security boundaries matter more than memory cost.

---

## Context Switching

### What Happens During A Context Switch

```text
Timer interrupt fires (or I/O request, or process blocks)
        ↓
CPU transfers control to OS kernel (user mode → kernel mode)
        ↓
OS saves current process state into PCB in kernel RAM:
  - Program Counter
  - All CPU registers
  - Process state → Ready or Waiting
        ↓
OS scheduler selects next process (lowest vruntime in CFS)
        ↓
OS loads that process's PCB
  - Restores registers
  - Switches page tables
        ↓
TLB flushed → old address translations invalid
        ↓
CPU resumes new process from exact instruction it left off
```

### Triggers For Context Switch

| Trigger | Type | Description |
|---|---|---|
| Timer Interrupt | Preemptive | Hardware timer fires (every ~4ms) — most common |
| I/O Request | Voluntary | Process waits for disk/network — OS switches immediately |
| Lock/Semaphore Block | Voluntary | Process cannot proceed — yields CPU |
| Higher Priority Ready | Preemptive | Scheduler preempts current process |
| System Call | May or may not | Depends on whether process blocks |

### Cost Of Context Switching

**Direct costs:**
- Save/restore CPU registers
- Switch page tables
- TLB flush

**Indirect costs:**
- CPU cache goes cold (L1/L2 had previous process's working set)
- Cache misses on new process until cache warms up

**At scale:** thousands of context switches per second → cache thrashing → measurable throughput degradation. Primary reason async/event-loop models outperform thread-per-request at high concurrency.

---

## Memory Layout

### Full Process Address Space

```text
High Address
┌─────────────────────────┐
│         Stack           │  ← grows downward ↓
│  (local vars, frames,   │
│   return addresses)     │
├─────────────────────────┤
│           ↓             │
│         (gap)           │
│           ↑             │
├─────────────────────────┤
│          Heap           │  ← grows upward ↑
│  (malloc, new)          │
├─────────────────────────┤
│          BSS            │  ← uninitialized globals (zero-filled)
├─────────────────────────┤
│          Data           │  ← initialized globals
├─────────────────────────┤
│          Text           │  ← compiled code (read-only, shared)
└─────────────────────────┘
Low Address
```

### Stack vs Heap

| Dimension | Stack | Heap |
|---|---|---|
| Allocation speed | Extremely fast (move stack pointer) | Slower (allocator finds free block) |
| Deallocation | Automatic on function return | Manual (C) / GC (Java, Go) |
| Size limit | Small — ~8MB on Linux | Large — limited by available RAM |
| Lifetime | Tied to function scope | Until freed or GC collected |
| Fragmentation | None | Yes — grows over time |
| Thread ownership | Each thread has its own | Shared across all threads |

### Dangling Pointer — Critical Bug

```c
int* bad() {
    int x = 42;      // lives on stack
    return &x;       // stack frame destroyed on return
}                    // pointer now points to invalid memory
```

- Stack frame destroyed on function return
- Pointer refers to memory that may be overwritten by next function call
- Non-deterministic — may work accidentally in testing, crash in production

**Root cause:** Lifetime mismatch — pointer outlives the variable it points to.

**Fix:** Return by value, heap-allocate, or pass output buffer from caller.

### Pointer Bug Taxonomy

| Bug | Cause |
|---|---|
| Dangling Pointer | Points to freed/out-of-scope memory |
| Wild Pointer | Uninitialized pointer — random address |
| Memory Leak | Heap allocated, never freed |

---

## Virtual Memory & Paging

### Core Idea

Every process believes it has its own full address space. This is an illusion maintained by the OS + hardware.

```text
Virtual Address  →  [MMU + Page Table]  →  Physical Address
(what process sees)                         (actual RAM location)
```

Process A and Process B can both use virtual address `0x1000` — they map to **different physical frames**.

### Page Fault Mechanism

```text
Process accesses virtual address X
        ↓
MMU checks page table → page NOT in RAM
        ↓
CPU raises Page Fault (hardware interrupt)
        ↓
OS Page Fault Handler takes control
        ↓
OS finds page on disk (swap space)
        ↓
OS loads page into a free RAM frame
        ↓
Page table updated: virtual page → new physical frame
        ↓
Process resumes from exact faulting instruction
        ↓
(OS may context switch during disk I/O wait)
```

**Page fault ≠ context switch.** Page fault is the trigger. Context switch may happen as a consequence during slow disk I/O.

### TLB — Translation Lookaside Buffer

- Hardware cache inside MMU
- Stores recent virtual → physical translations
- TLB hit → single CPU cycle
- TLB miss → must read page table from RAM → slow
- Typical hit rate: 99%+
- **Flushed on every context switch** → cold TLB is a major hidden cost of switching

### Page Replacement Algorithms

| Algorithm | Strategy | Problem |
|---|---|---|
| FIFO | Evict oldest loaded page | Ignores usage — Belady's Anomaly |
| LRU | Evict least recently used | Accurate but expensive to implement purely |
| Clock (Second Chance) | Approximate LRU via reference bit | Used in Linux |
| MRU | Evict most recently used | Useful for sequential scan patterns |

Linux uses **Clock/Second Chance** — not pure LRU. Pure LRU requires tracking every access which is too expensive at hardware speed.

### What Virtual Memory Enables

| Capability | Mechanism |
|---|---|
| Process isolation | Separate page tables per process |
| Use more memory than RAM | Swap cold pages to disk |
| Memory-mapped files (mmap) | Map file into virtual address space — zero-copy reads |
| Shared memory IPC | Two processes map same physical frame |
| Copy-on-Write (fork) | Share pages until write — then copy only modified page |

### Copy-on-Write During Fork

```text
fork() called:
Parent virtual 0x1000 ──→ Physical Frame 0x5000 ←── Child virtual 0x1000
Pages marked read-only in both page tables

Child writes to 0x1000:
CPU detects read-only → Page Fault → kernel handler
Kernel duplicates frame: 0x5000 → new frame 0x6000
Child page table updated → 0x6000
Parent page table unchanged → 0x5000
Both proceed independently
```

Makes fork() fast even for large processes — only dirty pages are ever copied.

### mmap — Zero Copy I/O

```text
Traditional read():
Disk → kernel buffer → process buffer    (2 copies, 2 context switches)

mmap():
Disk → RAM frame → process virtual address    (0 copies, page fault only)
```

Used by: PostgreSQL, SQLite, Kafka, RocksDB, JVM for class loading.

**mmap tradeoffs:**

| Benefit | Cost |
|---|---|
| Zero-copy reads | First access causes page fault latency |
| No system call per access | OS controls eviction — database loses control |
| Large file support | TLB pressure with huge files |
| Transparent prefetching | Write durability harder to guarantee |

**madvise()** lets process hint OS about access patterns:
- `MADV_SEQUENTIAL` → aggressive readahead
- `MADV_RANDOM` → disable prefetch
- `MADV_WILLNEED` → load soon
- `MADV_DONTNEED` → free frames

---

## System Calls

### Why They Exist

Processes run in **user mode (Ring 3)** — restricted. Kernel runs in **kernel mode (Ring 0)** — full hardware access. Hardware enforces this boundary.

Without the boundary:
- Any process reads any memory → security violation
- Buggy process corrupts kernel → system crash
- No resource control → processes monopolize hardware

### CPU Privilege Rings

```text
Ring 0 → Kernel Mode    (OS — full hardware access)
Ring 3 → User Mode      (applications — restricted)
```

If Ring 3 code executes a privileged instruction → CPU raises exception → process killed.

### System Call Mechanism

```text
Process places arguments in CPU registers
        ↓
Executes syscall instruction (x86: syscall / int 0x80)
        ↓
CPU traps to kernel mode (Ring 3 → Ring 0)
Saves user space state
        ↓
Kernel identifies syscall number → dispatches handler
        ↓
Kernel validates arguments + permissions
        ↓
Kernel executes operation
        ↓
Result copied into process memory
        ↓
CPU returns to user mode (Ring 0 → Ring 3)
Restores user registers
        ↓
Process continues — never entered kernel space directly
```

### System Call Cost

| Call Type | Latency |
|---|---|
| Normal function call | ~nanoseconds |
| System call | ~microseconds (100x more expensive) |

**Implication:** Minimize system calls in hot paths. Batch reads with large buffers. Use mmap for file-heavy workloads.

### Common System Call Categories

| Category | Examples |
|---|---|
| Process Control | fork, exec, exit, wait |
| File Operations | open, read, write, close |
| Memory | mmap, brk, munmap |
| Network | socket, bind, listen, accept, connect |
| IPC | pipe, shmget, msgget |

### Meltdown (2018)

Exploited speculative execution to let user-space processes read kernel memory. Fix required full kernel/user page table separation (KPTI) — caused 5-30% performance regression on I/O-heavy workloads. Shows how thin and critical this boundary is.

---

## File Descriptors

### What A File Descriptor Is

When a process opens a file, the OS returns a small non-negative integer — the **file descriptor (fd)**. The process uses this integer for all subsequent operations on that resource.

```text
fd = open("data.txt", O_RDONLY)
→ returns 3
```

### Three-Level Data Structure

```text
Per Process:
File Descriptor Table
  fd 0 → stdin
  fd 1 → stdout
  fd 2 → stderr
  fd 3 → [pointer] ──────────────────────────────┐
                                                   ↓
System Wide:                               Open File Table
                                   {offset, mode, flags, ref count}
                                                   │
                                                   ↓
                                           Inode Table
                                   {disk location, size, permissions}
```

- **File Descriptor Table** — per process
- **Open File Table** — system-wide, shared if processes share same open file (e.g. after fork)
- **Inode Table** — one inode per file regardless of how many times opened

### Everything Is A File

Unix treats every I/O resource as a file descriptor:

```text
Regular file    → fd
Network socket  → fd
Pipe            → fd
Device          → fd
Terminal        → fd
Epoll instance  → fd
Timer (timerfd) → fd
Signal (signalfd) → fd
```

Same `read()` / `write()` system calls work on all of them. Process doesn't know or care what's underneath.

### Epoll — High-Performance I/O Multiplexing

```c
int epfd = epoll_create1(0);           // fd watching other fds
epoll_ctl(epfd, EPOLL_CTL_ADD, sock_fd, &event);  // register socket
epoll_wait(epfd, events, max, timeout); // block until any fd is ready
```

Single thread watches thousands of file descriptors. Only wakes when one is ready. Zero wasted CPU on polling.

Used by: **Nginx, Node.js event loop, Redis** — foundation of high-performance single-threaded servers.

### File Descriptor Limits

```text
Default Linux limit: 1024 per process
Production typical:  65536+ (configured via ulimit -n)
```

Each open socket, file, pipe consumes one fd. A service handling 10,000 concurrent connections needs 10,000+ fds minimum.

**Hitting fd limit causes:**
- New connections refused
- File opens failing
- "Too many open files" in logs

**Diagnosis:**
```bash
lsof -p <pid> | wc -l     # count open fds for process
cat /proc/<pid>/limits     # view current fd limit
ulimit -n 65536            # increase limit (temporary)
```

**fd leak pattern:** Socket opened per request, exception thrown before close → fd never released → slow leak → eventual exhaustion.

### Fork And File Descriptors

Child inherits all parent's file descriptors after fork. Both share the same open file table entry — including **file offset**. Parent reads 100 bytes → child reads from byte 101.

Used intentionally by shells to wire pipes. Unintentional inheritance causes fd leaks in child processes.

---

## Signals

### What A Signal Is

An **asynchronous notification** sent to a process. Arrives at any point — between any two instructions — without the process requesting it.

### Critical Signals

| Signal | Default | Meaning |
|---|---|---|
| SIGTERM (15) | Terminate | Polite shutdown request — **can be caught** |
| SIGKILL (9) | Terminate | Forced kill — **cannot be caught, blocked, or ignored** |
| SIGINT (2) | Terminate | Ctrl+C from terminal |
| SIGSEGV (11) | Core dump | Invalid memory access |
| SIGHUP (1) | Terminate | Terminal closed — used for **config reload** |
| SIGCHLD (17) | Ignore | Child process terminated |
| SIGSTOP (19) | Stop | Pause — **cannot be caught** |
| SIGALRM (14) | Terminate | Timer expired |

### SIGTERM vs SIGKILL

```text
SIGTERM:
Process receives signal
→ Runs cleanup handler:
   - finish in-flight requests
   - flush write buffers
   - close DB connections cleanly
   - release distributed locks
   - log shutdown
→ Exits cleanly

SIGKILL:
Kernel immediately destroys process
→ No cleanup possible
→ In-flight requests dropped
→ Connections not closed
→ Locks not released
→ Buffers not flushed
```

**SIGKILL cannot be caught, blocked, or ignored — ever.**

### Kubernetes Shutdown Sequence

```text
Pod termination triggered
        ↓
SIGTERM sent to process
        ↓
Graceful shutdown period (default 30s)
        ↓
If process still running → SIGKILL
```

**Production bug:** Service ignores SIGTERM → Kubernetes SIGKILLs it → in-flight requests dropped → DB connections leak → distributed locks not released.

**Required pattern:** Catch SIGTERM → stop accepting requests → drain in-flight → close connections → exit.

### SIGHUP — Config Reload Pattern

```bash
kill -HUP <nginx_pid>
```

Nginx catches SIGHUP → reloads config → keeps existing connections alive → zero downtime reload. Standard pattern for all production daemons.

### Signal Handler Safety — Hidden Complexity

Inside a signal handler — **most functions are unsafe** to call.

```text
Process executing malloc() → acquires internal heap lock
Signal arrives → handler calls malloc()
→ Tries to acquire heap lock
→ Same thread already holds it
→ Deadlock
```

**Async-signal-safe functions only inside handlers:**
- Safe: `write()`, `_exit()`, `kill()`, `signal()`
- Unsafe: `printf()`, `malloc()`, `free()`, logging libraries

**Production pitfall:** Logging inside signal handler using standard logging framework → works in testing → deadlocks under production load.

### Signal Delivery Mechanism

Signals are delivered at **kernel → user mode transitions** (after system calls, after interrupts). A process in a tight CPU loop without system calls may not receive signals immediately.

---

## Process Scheduling

### Competing Scheduler Goals

| Goal | Wants |
|---|---|
| Throughput | Run jobs to completion, minimize switching |
| Latency | Preempt frequently, short jobs first |
| Fairness | No process starves |
| CPU Utilization | Always have something running |
| Real-Time | Hard deadlines for audio, hardware |

**These goals conflict.** Every scheduler trades off between them.

### Scheduling Algorithms

**FCFS (First Come First Served)**
- Run to completion in arrival order
- Problem: **Convoy Effect** — one long job blocks many short ones

**SJF (Shortest Job First)**
- Optimal average wait time
- Problem: requires knowing job length in advance — impossible in practice
- Problem: starvation of long jobs

**Round Robin**
- Each process gets a fixed time quantum (~4ms on Linux)
- Preempt after quantum → move to back of queue
- Good fairness and response time
- Quantum too small → context switch overhead dominates
- Quantum too large → degenerates to FCFS

**Priority Scheduling + Aging**
- Higher priority runs first
- Aging: waiting processes gradually get priority bumped → prevents starvation

### Linux CFS — Completely Fair Scheduler

Core idea: every process gets CPU proportional to its weight (priority).

**Virtual Runtime (vruntime):**
```text
vruntime = actual CPU time × (1 / weight)

High priority process (weight 2):
  runs 10ms → vruntime += 5ms

Normal process (weight 1):
  runs 10ms → vruntime += 10ms

Scheduler always picks: lowest vruntime
→ High priority gets more CPU
→ But low priority never starved
```

**Data structure:** Red-Black Tree ordered by vruntime. O(log n) insert, O(1) find minimum (leftmost node).

### Priority Inversion

```text
L (low priority) holds mutex
H (high priority) blocks waiting for mutex
M (medium priority) has no interest in mutex → runs freely

Result: M runs, H waits — medium has effectively higher priority than high
```

**Fix: Priority Inheritance** — H blocked on L's mutex → L temporarily inherits H's priority → L gets CPU → releases lock → H proceeds.

**Mars Pathfinder 1997** — real spacecraft reset caused by this exact scenario.

---

## Connecting Concepts — Backend Engineering Implications

### Thread Model Comparison

| Model | Threads | Context Switches | Memory | Limitation |
|---|---|---|---|---|
| Thread-per-request | 1 per request | High | High (~8MB stack each) | Context switch overhead |
| Event loop (Node.js) | 1 | None | Minimal | Blocks on CPU work |
| Worker pool (Go, Nginx) | N (= cores) | Low | Low | Complexity |

### Go Goroutine Scheduler

```text
Goroutines (user-space threads) mapped to OS threads (M:N model)
Go scheduler switches goroutines in user space
→ No kernel context switch for goroutine switch
→ Much cheaper than OS thread context switch
→ Millions of goroutines feasible
```

### Erlang/Elixir Philosophy

Millions of isolated lightweight processes communicating via messages. No shared memory. No locks. One process crash cannot corrupt another. Extreme reliability at the cost of no shared state.

---

## Failure Scenarios

| Scenario | Root Cause | Mitigation |
|---|---|---|
| Stack overflow | Deep recursion exhausts ~8MB stack | Iterative implementation, increase stack size |
| Heap fragmentation | Long-running service — allocate/free many small objects | Better allocators (jemalloc, tcmalloc), periodic restart |
| Dangling pointer | Return pointer to stack variable | Return by value, heap allocate, caller-provided buffer |
| fd exhaustion | Connection leak / high concurrency / low ulimit | Fix leaks, increase ulimit, monitor fd count |
| SIGKILL dirty shutdown | Service ignores SIGTERM | Implement SIGTERM handler with graceful drain |
| Priority inversion deadlock | Low-priority thread holds lock needed by high-priority | Priority inheritance mutexes |
| TLB thrashing | Excessive context switching | Reduce thread count, use async I/O, huge pages |

---

## Related Concepts

- [[Virtual Memory]]
- [[File Descriptors]]
- [[Concurrency Basics]]
- [[Mutex Semaphore Spinlock]]
- [[Deadlock Livelock Starvation]]
- [[Thread Pools]]
- [[Async Await vs Threads vs Event Loop]]
- [[Linux Basics]]
- [[Memory Visibility and Happens-Before]]
- [[System Calls]]

---

## Revision Summary

1. A process is an isolated execution environment — Text, Data, BSS, Heap, Stack, FDs, PCB
2. Threads share heap and FDs — each has own stack and registers — shared memory causes race conditions
3. Context switch saves PCB to kernel RAM — flushes TLB — most expensive hidden cost is cache cold start
4. Virtual memory gives each process an address space illusion — MMU + page tables do the translation
5. Page fault ≠ context switch — page fault is the mechanism, context switch may follow during disk I/O
6. TLB is a hardware cache for address translations — flushed on every context switch
7. Copy-on-Write makes fork() cheap — pages shared until write — then duplicated per process
8. System calls are expensive (~100x function call) — minimize in hot paths — batch I/O, use mmap
9. Everything is a file descriptor — uniform read()/write() API across files, sockets, pipes, devices
10. Epoll watches thousands of fds on one thread — foundation of Nginx, Node.js, Redis
11. SIGTERM = polite, catchable. SIGKILL = forced, uncatchable. Always handle SIGTERM in production
12. CFS scheduler tracks vruntime — always runs process with lowest vruntime — Red-Black Tree
13. Priority inversion: low-priority holds lock needed by high-priority — fix with priority inheritance
14. fd limit exhaustion = real production failure — set ulimit, monitor, fix leaks

---

## Active Recall Questions

1. What is the difference between a page fault and a context switch? Can one cause the other?
2. Why is the TLB flushed on context switch? What is the performance implication?
3. Two processes both access virtual address `0x1000`. How does the OS prevent them from corrupting each other?
4. Why is `mmap` faster than `read()` for large file access? What does it trade off?
5. A function returns a pointer to a local variable. What goes wrong and why?
6. Why can't you call `printf()` inside a signal handler safely?
7. Your service gets "too many open files" in production. Walk through diagnosis and fix.
8. Kubernetes sends SIGTERM to your pod. Your process ignores it. What happens?
9. What is priority inversion? How did it crash the Mars Pathfinder?
10. Why does Go use goroutines instead of OS threads for concurrency?
11. What is Copy-on-Write during fork()? Why does it make fork() fast?
12. A process is in tight CPU loop — will SIGTERM be delivered immediately? Why or why not?
13. Why does Linux use Clock/Second Chance instead of pure LRU for page replacement?
14. What is the Convoy Effect in FCFS scheduling? Which workloads suffer most?
15. How does epoll allow a single thread to handle 10,000 concurrent connections?
