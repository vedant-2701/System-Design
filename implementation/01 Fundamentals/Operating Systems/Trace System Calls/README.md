# strace — System Call Tracing

## Problem Statement

Every interaction between a user-space program and the OS kernel happens via system calls. These calls are invisible by default — your program calls `printf()`, `malloc()`, or `open()` and the library handles everything underneath. This invisibility is a major debugging liability.

When a production service behaves unexpectedly — slow file reads, hanging on a lock, leaking file descriptors, silently failing to open a config file — the root cause often lives at the system call boundary. Without a tool to observe that boundary, diagnosis becomes guesswork.

`strace` solves this by intercepting and logging every system call a process makes in real time, including its arguments, return values, and time spent. It works via Linux's `ptrace` mechanism — the same mechanism used by debuggers like `gdb`.

This matters in real systems because:
- "Too many open files" errors require seeing which `openat()` calls lack matching `close()` calls
- Hung processes are usually stuck in `futex()` (lock contention) or `epoll_wait()` (I/O wait) — strace reveals which
- Slow I/O paths are diagnosed by seeing whether `read()` is called with tiny buffers (many syscalls) or large ones (efficient)
- Unexpected library loading, config file lookups, and permission failures all appear as syscalls before your code even runs

---

## What We Traced and Why

### 1. `echo "hello"` — Program Startup Overhead

**Command:**
```bash
strace echo "hello"
```

**Key finding:** 44 total system calls to print one line. Only 1 (`write`) did useful work. The other 43 were startup — dynamic linker bootstrapping, loading `libc.so.6` via `mmap`, setting up thread-local storage, querying stack limits.

**Engineering insight:** Short-lived processes pay full startup cost on every invocation. A process pool or long-running daemon amortizes this across thousands of requests. This is the OS-level reason why spawning a process per request is expensive.

**Startup sequence observed:**
```text
execve()          → replace process image with target binary
access()          → check /etc/ld.so.preload (security/profiler hooks)
openat() + mmap() → load libc.so.6 into virtual address space
arch_prctl()      → set up thread-local storage (even for single-threaded programs)
prlimit64()       → query stack size limit (result: 8MB — matches theory)
write()           → the actual work
exit_group()      → multi-thread aware exit
```

---

### 2. `cat /tmp/sample.txt` — Buffered I/O and EOF Protocol

**Command:**
```bash
strace -e trace=openat,read,write,close cat /tmp/sample.txt
```

**Key finding:**
```text
read(3, "line one\nline two\nline three\n", 131072) = 29
read(3, "", 131072) = 0
```

Two observations with direct engineering implications:

**128KB read buffer:** `cat` requests 128KB even for a 29-byte file because it cannot know the file size before reading. The alternative — reading byte-by-byte — would require one syscall per byte. At ~1μs per syscall, reading a 1MB file byte-by-byte costs ~1 second in syscall overhead alone. Buffered I/O (`BufferedReader` in Java, `bufio.Reader` in Go, `fread()` in C) exists entirely to address this.

**Second `read()` returning 0:** EOF is not signaled by an error. It is signaled by `read()` returning 0 bytes. The process must attempt another read to discover it has reached the end. This protocol is uniform across files, pipes, sockets, and devices — the abstraction is identical regardless of the underlying resource.

---

### 3. `sh -c "echo hello | cat"` — Pipes Are Just dup2 + fork

**Command:**
```bash
strace -e trace=clone,execve,wait4,pipe2,dup2 -f sh -c "echo hello | cat"
```

**Key finding:** A shell pipeline is entirely constructed from four system calls:

```text
pipe2([3, 4], 0)           → create pipe: fd3=read end, fd4=write end

clone()  → fork child 1 (echo)
  dup2(4, 1)               → child's stdout = pipe write end
  [echo writes to stdout → data enters pipe]

clone()  → fork child 2 (cat)
  dup2(3, 0)               → child's stdin = pipe read end
  execve("/usr/bin/cat")   → replace child with cat binary
  [cat reads from stdin → reads from pipe → prints to terminal]

wait4(-1) × 2              → parent waits for both children
```

The pipe is not a special kernel object with unique APIs. It is two file descriptors — one readable, one writable. `dup2` rewires a child's standard streams to those descriptors before `exec`. The child program never knows it is reading from a pipe instead of a terminal. This is the "everything is a file descriptor" abstraction made concrete.

---

### 4. C program with `malloc()` — brk vs mmap Threshold

**Program:** Allocates 100 bytes (small) and 10MB (large), then frees both.

**Key finding:**
```text
Small malloc (100 bytes):
  brk(0x55e10ff74000)               → expand heap by moving break pointer

Large malloc (10MB):
  mmap(NULL, 10489856, PROT_READ|PROT_WRITE, MAP_PRIVATE|MAP_ANONYMOUS)
  munmap(0x7f35f67ff000, 10489856)  → freed immediately back to OS
```

**Engineering insight:** glibc uses `brk()` for allocations below ~128KB (MMAP_THRESHOLD) and `mmap()` for larger ones. Large allocations freed via `munmap()` return memory to the OS immediately — RSS drops. Small allocations freed return to the heap free list but do not necessarily reduce RSS. This is why long-running services that allocate and free many small objects show steadily growing RSS that monitoring dashboards flag as memory leaks — they are not leaks, but heap fragmentation. Production systems use `jemalloc` or `tcmalloc` to manage this better.

---

## Strace as a Production Debugging Tool

### Diagnose slow syscalls
```bash
strace -T ./program
# -T prints time spent in each syscall
# reveals: is slowness in disk I/O? network? lock contention?
```

### Find file descriptor leaks
```bash
strace -e trace=openat,close -p <pid>
# watch for openat() calls without matching close()
# identifies leaked fds in real time on a running process
```

### Diagnose hung processes
```bash
strace -p <pid>
# stuck in read() or epoll_wait() → waiting for I/O (usually expected)
# stuck in futex()               → waiting for a lock (possible deadlock)
# stuck in nothing / looping     → CPU-bound or busy-wait bug
```

### Summarize syscall frequency
```bash
strace -c ./program
# prints table: syscall | calls | total time | % time
# reveals which syscalls dominate — where to focus optimization
```

### Trace child processes
```bash
strace -f ./program
# follows all forked children
# essential for tracing process pools, shells, daemons
```

---

## Key Syscalls Observed and Their Meaning

| Syscall | What It Does | When You See It |
|---|---|---|
| `execve` | Replace process image with new binary | Every program start |
| `mmap` | Map memory or file into address space | Library loading, large malloc, file I/O |
| `brk` | Expand/query heap boundary | Small malloc |
| `openat` | Open file, return fd | Any file or device access |
| `read` | Read bytes from fd | File, pipe, socket reads |
| `write` | Write bytes to fd | File, pipe, socket writes |
| `close` | Release file descriptor | End of file/socket use |
| `pipe2` | Create a pipe (two fds) | Shell pipelines, IPC |
| `dup2` | Rewire one fd to another | Shell pipe wiring, redirect |
| `clone` | Fork a new process or thread | Process/thread creation |
| `wait4` | Wait for child process exit | Parent after fork |
| `futex` | Fast userspace mutex | Lock acquire/release |
| `epoll_wait` | Wait for any fd to be ready | Event-loop servers (Nginx, Node.js) |
| `prlimit64` | Query/set resource limits | Stack size, fd limits |
| `munmap` | Release mapped memory | Large free, library unload |

---

## Alternatives Considered

**`ltrace`** — traces library calls (e.g. `malloc`, `printf`) instead of syscalls. Useful when the problem is in library behavior rather than kernel interaction. Less useful for I/O and process debugging.

**`perf`** — kernel-level profiler. Better for CPU hotspot analysis and high-frequency sampling. Does not show per-call arguments like strace does. Use perf when you need performance data, strace when you need behavioral understanding.

**`/proc/<pid>/fd`** — lists open file descriptors for a running process. Useful for fd leak diagnosis without attaching strace. Less detailed than strace but zero overhead.

**eBPF (bpftrace, bcc)** — modern, low-overhead alternative to strace. Can trace syscalls without ptrace overhead, supports aggregation and filtering at kernel level. Requires kernel 4.9+. strace remains simpler for one-off diagnosis; eBPF is better for production tracing under load.

---

## Production Considerations

**strace has overhead.** ptrace intercepts every syscall — typically 2-10x slowdown on syscall-heavy workloads. Never run strace on a production process under load unless prepared for that impact. For production-safe tracing, use eBPF tools.

**strace on multi-threaded processes requires `-f`.** Without `-f`, only the main thread is traced. With `-f`, each thread/process gets a `[pid N]` prefix in output.

**Output volume.** A busy server generates enormous strace output. Use `-e trace=<syscall>` to filter, `-o <file>` to write to disk, and `-c` for summary-only mode.

**Permissions.** Tracing another process requires either the same UID or `CAP_SYS_PTRACE`. In containers, ptrace is often disabled by the seccomp profile — strace will fail with "Operation not permitted."

---

## Future Improvements / Extensions

- Use `strace -c` to build a syscall frequency baseline for a service, then alert when that profile changes significantly (could indicate performance regression or behavioral change after a deploy)
- Combine with `opensnoop` (eBPF) for production-safe file access monitoring
- Write a script that parses strace output to automatically detect fd leaks: `openat` calls without matching `close` within a time window
