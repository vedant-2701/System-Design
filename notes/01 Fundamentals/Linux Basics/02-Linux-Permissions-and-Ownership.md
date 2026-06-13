# Linux Permissions and Ownership

## Tags
#linux #security #operating-systems #foundations

---

## Overview

- Every file and directory has an owner (user), a group, and a permission set
- Permissions control who can read, write, and execute
- Linux checks permissions at every directory in a path — not just the final file
- Three special bits extend the standard permission model: setuid, setgid, sticky bit

---

## Permission Structure

```
-rwxr-xr-- 1 vedant engineers 4096 Jun 1 12:00 server.sh
│└─┬──┘└─┬──┘└─┬──┘
│  │     │     └── others: r--
│  │     └──────── group (engineers): r-x
│  └────────────── owner (vedant): rwx
└────────────────── file type: - = file, d = directory, l = symlink
```

### Permission bits

| Bit | File meaning | Directory meaning |
|-----|-------------|-------------------|
| `r` | Read file contents | List filenames with `ls` |
| `w` | Modify file contents | Create/delete files inside |
| `x` | Execute as program | Traverse (cd into it, access contents) |

**Critical:** `r` without `x` on a directory = can list filenames but cannot `cd` in or access any file inside, regardless of file permissions.

---

## Octal Representation

```
r=4, w=2, x=1

755 → owner:rwx(7), group:r-x(5), others:r-x(5)
644 → owner:rw-(6), group:r--(4), others:r--(4)
600 → owner:rw-(6), group:---(0), others:---(0)
400 → owner:r--(4), group:---(0), others:---(0)
```

### Common production patterns

| Permission | Use case |
|-----------|----------|
| `600` | Private keys, config files with secrets |
| `400` | SSH private key (read-only even for owner) |
| `644` | Public config files, static assets |
| `755` | Executables, public directories |
| `700` | Private directories (only owner accesses) |
| `770` | Shared team directories (group access, others blocked) |

---

## Path Permission Check

To access `/var/log/app/service.log`, kernel checks:
```
x on /var
x on /var/log
x on /var/log/app
r on service.log   ← only checked if all above pass
```

**Production implication:** Permission errors often aren't on the file itself — they're on a parent directory missing `x`. Always check the full path.

---

## Special Permission Bits

### Sticky Bit (t)

```bash
ls -la /
drwxrwxrwt tmp    # t instead of x = sticky bit set
chmod +t /shared/uploads
chmod 1777 /tmp   # leading 1 = sticky bit
```

**Effect on directories:** Only the file's owner or root can delete files inside, even if others have write permission on the directory.

**Use cases:** `/tmp`, shared upload directories, any multi-user write directory where deletion should be restricted to file owners.

### setuid (s on owner execute bit)

```bash
ls -la /usr/bin/passwd
-rwsr-xr-x root root /usr/bin/passwd   # s = setuid
```

**Effect:** Executable runs as the file's **owner** (often root), not the invoking user.

**Use case:** `passwd` needs to write `/etc/shadow` (root-owned) but must be runnable by regular users.

**Security risk:** Vulnerability in a setuid binary = root exploit. Audit regularly:
```bash
find / -perm -4000 -type f 2>/dev/null
```

### setgid (s on group execute bit)

**On executables:** Runs with the file's group instead of user's group.

**On directories:** New files created inside inherit the directory's group instead of creator's primary group.

```bash
chmod g+s /shared/project    # new files get project group automatically
```

**Use case:** Shared team directories where all files must belong to the same group regardless of who created them.

---

## Tradeoffs

| Approach | Benefit | Risk |
|----------|---------|------|
| `777` permissions | Easy access | Any user can modify/delete — security disaster |
| Sticky bit on shared dir | Prevents accidental deletion | Users can still overwrite others' files |
| setuid binaries | Controlled privilege escalation | Attack surface for privilege escalation exploits |
| `400` on config files | Immutable, owner can't accidentally overwrite | Must `chmod 600` to edit |

---

## Failure Scenarios

- SSH key at `644` → SSH refuses to connect: "Permissions too open"
- Missing `x` on parent directory → permission denied accessing file even though file permissions are correct
- No sticky bit on shared `/tmp`-like directory → users delete each other's files
- setuid binary with vulnerability → privilege escalation to root
- Wrong group on shared directory → developers can't read each other's files, fixed with `usermod -aG <group> <user>` not by relaxing permissions

---

## Common Mistakes

- Setting `777` "to fix permission errors" in production — never do this
- Checking file permissions but not parent directory permissions when debugging access issues
- Using `chmod +x` on a config file — config files should never be executable
- Loosening permissions instead of adding user to correct group

---

## Interview Perspective

- "What does execute mean on a directory?" → traverse/cd, not run
- "What is the sticky bit?" → only owner can delete their files in shared directories
- "Why does SSH reject a `644` private key?" → private keys must not be readable by others — setuid/setgid/sticky all tested here
- "How does Linux check permissions for a deeply nested file?" → checks `x` on every parent directory in path

---

## Revision Summary

- `r/w/x` mean different things on files vs directories — `x` on directory = traverse
- Kernel checks permissions at every component of a path, not just the final file
- `600` for secrets (read/write owner only), `400` for truly read-only (SSH keys)
- Sticky bit: only owner can delete own files — used on `/tmp`, shared dirs
- setuid: binary runs as file owner (root escalation risk) — audit with `find / -perm -4000`
- setgid on directory: new files inherit directory's group — essential for team dirs
- Fix access by adding user to group (`usermod -aG`), not by opening permissions

---

## Active Recall Questions

1. What does `x` on a directory actually control?
2. A developer can list files in `/app/config/` but gets "permission denied" when opening any file. What's the likely issue?
3. What permissions would you set on a directory shared by a team where nobody should delete others' files?
4. Why is a setuid binary a security concern and how do you find all of them?
5. Why does SSH reject a private key with `644` permissions?
6. What is the difference between `600` and `400` and when do you use each?

---

## Related Concepts

- [[Linux Filesystem Structure]]
- [[Linux Process Management]]
- [[Linux Systemd and Services]]
- [[SSH and Remote Access]]
