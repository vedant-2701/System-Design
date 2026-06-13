# Question
> It's 9am. You get an alert — your backend service is down. You SSH in.

--- 

**Step 1 — Is the service running at all?**
```bash
systemctl status myapp
```
This single command tells you: running or crashed, restart count, last few log lines, PID. 

Two branches from here:

---

**Branch A — Service is not running:**

```bash
journalctl -u myapp -n 100    # why did it crash?
```

Look for: OOM killed, port already in use, missing config file, dependency failed.

Then check if it's crash-looping:
```bash
systemctl status myapp    # shows restart count
```

If crash-looping — fix the root cause before restarting, otherwise it'll loop again.

---

**Branch B — Service is running but not responding:**

Now you investigate a live process. This is where your flow kicks in.

```bash
# Is it even listening on the right port?
ss -tlnp sport = :8080
```

Three outcomes:
- Port not there → service started but failed to bind → check logs
- Port on 127.0.0.1 → wrong interface binding → config issue
- Port on 0.0.0.0 → listening correctly → go deeper

```bash
# Is it overloaded?
top    # check CPU, wa%, load average trend
```

- High `us` → application CPU bound → thread pool exhausted?
- High `wa` → I/O bottleneck → database slow?
- High load, low CPU → too many processes waiting

```bash
# Check resource exhaustion
cat /proc/<PID>/limits | grep "open files"
ls /proc/<PID>/fd | wc -l    # compare to limit
```

```bash
# Check connection states
ss -tan | awk '{print $1}' | sort | uniq -c | sort -rn
```

- Thousands of `CLOSE_WAIT` → FD leak in application
- Thousands of `TIME_WAIT` → too many short-lived connections
- High `Recv-Q` on listening port → application not accepting fast enough

```bash
# Are dependencies reachable?
ss -tnp dst :5432    # active connections to postgres
ss -tnp dst :6379    # active connections to redis
tcpdump -i eth0 -n host redis-host port 6379    # are packets flowing?
```

```bash
# Finally — logs for the specific timeframe
journalctl -u myapp --since "1 hour ago" -p err
```

**One thing you got right that many engineers miss:**

Zombie processes. Most people forget them entirely. Good instinct to include that.

---