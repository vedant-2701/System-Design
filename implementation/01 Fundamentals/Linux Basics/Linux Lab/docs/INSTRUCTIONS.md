# Part 1 — Environment Setup

## Native Linux (Ubuntu/Debian)

```bash
# Verify systemd is PID 1
ps -p 1 -o comm=
# Expected output: systemd
```

## WSL2

WSL2 systemd support requires Windows 11 or Windows 10 build 22000+.

```bash
# Check your WSL version
wsl --version    # run this in PowerShell/CMD on Windows
```

Enable systemd in WSL2 — do this once:

```bash
# Inside WSL2 terminal
sudo nano /etc/wsl.conf
```

Add these lines:
```ini
[boot]
systemd=true
```

Then restart WSL2 from PowerShell:
```powershell
wsl --shutdown
wsl
```

Verify systemd is running:
```bash
ps -p 1 -o comm=
# Expected: systemd

systemctl status
# Should show system state, not an error
```

If you see `init` instead of `systemd` — systemd isn't enabled. Redo the `/etc/wsl.conf` step and restart WSL2.

---

# Part 2 — Install Prerequisites

```bash
# Both environments — same commands
sudo apt update

# Go compiler
sudo apt install -y golang-go

# curl (used by monitor script)
sudo apt install -y curl

# Verify
go version
curl --version
```

---

# Part 3 — Get the Code

```bash
# Create a working directory
mkdir -p ~/projects
cd ~/projects

# Copy the lab files — you have two options:

# Option A: manually create the structure
mkdir -p linux-service-lab/{cmd/server,scripts,systemd,logrotate.d}
cd linux-service-lab

# Then copy each file from the downloaded outputs into the right location:
# cmd/server/main.go
# scripts/monitor-myapp.sh
# scripts/install.sh
# systemd/myapp.service
# systemd/myapp-monitor.service
# systemd/myapp-monitor.timer
# logrotate.d/myapp
```

Initialize Go module:
```bash
cd ~/projects/linux-service-lab
go mod init github.com/vedant/linux-service-lab
```

---

# Part 4 — Build the Binary

```bash
cd ~/projects/linux-service-lab

go build -o server ./cmd/server/

# Verify it compiled
ls -lh server
# Expected: -rwxr-xr-x ... 7.1M ... server
```

---

# Part 5 — Smoke Test Locally (Before systemd)

Run the service directly first — verify it works before involving systemd. This isolates problems.

```bash
# Create a temp log directory
mkdir -p /tmp/myapp-test

# Start the service in background
APP_PORT=8080 APP_LOG_FILE=/tmp/myapp-test/myapp.log ./server &
SERVER_PID=$!

echo "Server started with PID: $SERVER_PID"
sleep 1
```

Test each endpoint:
```bash
# Health check
curl http://localhost:8080/health
# Expected: {"status":"ok","uptime":"...","timestamp":"..."}

# Work endpoint
curl "http://localhost:8080/work?n=500000"
# Expected: sum=... elapsed=...

# Write an error log
curl "http://localhost:8080/log?level=error&msg=hello-from-test"

# Check the log file
cat /tmp/myapp-test/myapp.log
# Expected: structured key=value log lines
```

Test graceful shutdown:
```bash
kill -15 $SERVER_PID
wait $SERVER_PID
echo "Exit code: $?"
# Expected exit code: 0 (clean shutdown)
# Log should show: msg=server_stopped_cleanly
cat /tmp/myapp-test/myapp.log
```

Test crash behavior:
```bash
# Start again
APP_PORT=8080 APP_LOG_FILE=/tmp/myapp-test/myapp.log ./server &
SERVER_PID=$!
sleep 1

# Trigger intentional crash
curl http://localhost:8080/crash

# Check exit code — should be 1 (non-zero = systemd will restart)
wait $SERVER_PID
echo "Exit code: $?"
```

---

# Part 6 — Run Monitor Script Manually

```bash
# Start service first
APP_PORT=8080 APP_LOG_FILE=/tmp/myapp-test/myapp.log ./server &
SERVER_PID=$!
sleep 1

# Run monitor against it
SERVICE_NAME=myapp \
HEALTH_URL=http://127.0.0.1:8080/health \
MAX_RSS_MB=400 \
MAX_FD_PERCENT=80 \
MAX_RESTART_COUNT=3 \
ALERT_LOG=/tmp/myapp-test/monitor-alerts.log \
bash scripts/monitor-myapp.sh

# Expected: all checks OK, exit 0
echo "Monitor exit code: $?"

# Stop service, run monitor again — should detect failure
kill -9 $SERVER_PID
sleep 1

SERVICE_NAME=nonexistent \
HEALTH_URL=http://127.0.0.1:8080/health \
MAX_RSS_MB=400 \
MAX_FD_PERCENT=80 \
MAX_RESTART_COUNT=3 \
ALERT_LOG=/tmp/myapp-test/monitor-alerts.log \
bash scripts/monitor-myapp.sh || echo "Monitor exit code: $? (expected non-zero)"

# Check alert log
cat /tmp/myapp-test/monitor-alerts.log
```

---

# Part 7 — Install via systemd

Now hand control to systemd. Everything from here requires root.

```bash
cd ~/projects/linux-service-lab

# Make scripts executable
chmod +x scripts/install.sh
chmod +x scripts/monitor-myapp.sh

# Run installer
sudo bash scripts/install.sh
```

Expected output:
```
[install] Starting myapp installation
[install] Creating service user: myapp
[install] User created: myapp
[install] Creating directories
[install] Directories created and permissions set
[install] Installing service binary
[install] Installing monitor script
[install] Binaries installed
[install] Writing default config to /etc/myapp/env
[install] Config written
[install] Installing systemd unit files
[install] Reloading systemd daemon
[install] Enabling service and timer
[install] Systemd units installed and enabled
[install] Installation complete.
[install] Next steps:
...
```

Verify what got installed:
```bash
# Check files on disk
ls -la /opt/myapp/bin/
ls -la /opt/myapp/scripts/
ls -la /var/log/myapp/
ls -la /etc/myapp/

# Check unit files
ls -la /etc/systemd/system/myapp*

# Check service user was created
id myapp
```

---

# Part 8 — Start and Observe the Service

```bash
# Start the service
sudo systemctl start myapp.service

# Check status immediately
systemctl status myapp.service
```

You should see:
```
● myapp.service - My Backend Service (linux-service-lab)
     Loaded: loaded (/etc/systemd/system/myapp.service; enabled; ...)
     Active: active (running) since ...
   Main PID: 12345 (server)
```

Follow logs live in a second terminal:
```bash
journalctl -u myapp.service -f
```

Test the running service:
```bash
curl http://localhost:8080/health
```

---

# Part 9 — Observe systemd Restart Behavior

This is the important part — watch systemd automatically recover from crashes.

Open two terminals:

**Terminal 1 — watch logs live:**
```bash
journalctl -u myapp.service -f
```

**Terminal 2 — trigger crash:**
```bash
# Check current PID
systemctl status myapp.service | grep "Main PID"

# Trigger crash via endpoint
curl http://localhost:8080/crash

# Watch Terminal 1 — you'll see:
# 1. Service logs the crash request
# 2. Process exits
# 3. After 5 seconds (RestartSec), systemd restarts it
# 4. New PID appears

# Verify new PID is different — service restarted
systemctl status myapp.service | grep "Main PID"

# Check restart count
systemctl show myapp.service -p NRestarts
```

Now trigger a crash loop to see `StartLimitBurst` kick in:
```bash
# Crash it 3 times quickly
curl http://localhost:8080/crash; sleep 1
curl http://localhost:8080/crash; sleep 1
curl http://localhost:8080/crash; sleep 1
curl http://localhost:8080/crash; sleep 1

# After 3 restarts in 60 seconds, systemd gives up
systemctl status myapp.service
# Expected: Active: failed (Result: start-limit-hit)

# This is correct behavior — forces you to investigate before blindly restarting
```

Reset and restart after fixing (simulated):
```bash
sudo systemctl reset-failed myapp.service
sudo systemctl start myapp.service
systemctl status myapp.service
```

---

# Part 10 — Start and Observe the Monitor Timer

```bash
# Start the timer
sudo systemctl start myapp-monitor.timer

# Verify timer is active and see next run time
systemctl status myapp-monitor.timer
systemctl list-timers myapp-monitor.timer
```

Wait 60 seconds for first run, or trigger manually:
```bash
# Trigger monitor job immediately without waiting for timer
sudo systemctl start myapp-monitor.service

# Check monitor output
journalctl -u myapp-monitor.service -n 30
```

Expected output:
```
timestamp=2024-01-04T14:00:01Z level=INFO service=myapp check=service_active status=OK
timestamp=2024-01-04T14:00:01Z level=INFO service=myapp check=health_endpoint status=OK http_code=200
timestamp=2024-01-04T14:00:01Z level=INFO service=myapp check=fd_usage status=OK open_fds=8 fd_limit=65536
timestamp=2024-01-04T14:00:01Z level=INFO service=myapp check=memory_usage status=OK rss_mb=12
timestamp=2024-01-04T14:00:01Z level=INFO service=myapp check=restart_count status=OK restart_count=0
timestamp=2024-01-04T14:00:01Z level=INFO service=myapp msg=monitor_run_complete overall_status=HEALTHY
```

Now stop the service and run monitor — watch it detect the failure:
```bash
sudo systemctl stop myapp.service

sudo systemctl start myapp-monitor.service

journalctl -u myapp-monitor.service -n 10
# Expected: check=service_active status=FAIL

# Check alert log
cat /var/log/myapp/monitor-alerts.log
```

Restart the service:
```bash
sudo systemctl start myapp.service
```

---

# Part 11 — Test Log Rotation

```bash
# Check current log file
ls -lh /var/log/myapp/
cat /var/log/myapp/myapp.log

# Write some log entries
curl "http://localhost:8080/log?level=error&msg=before-rotation"
curl "http://localhost:8080/log?level=info&msg=normal-entry"

# Test logrotate config syntax — dry run, no actual rotation
sudo logrotate --debug /etc/logrotate.d/myapp

# Force rotation (ignores date check — useful for testing)
sudo logrotate --force /etc/logrotate.d/myapp

# See what happened
ls -lh /var/log/myapp/
# Expected: myapp.log (new empty file) + myapp.log.1 (rotated)

# Write a new entry — should go to new myapp.log
curl "http://localhost:8080/log?level=info&msg=after-rotation"

# Verify new entries go to new file
cat /var/log/myapp/myapp.log
# Should show after-rotation entry

# Verify old entries are in rotated file
cat /var/log/myapp/myapp.log.1
# Should show before-rotation entry
```

---

# Part 12 — Useful Day-to-Day Commands

```bash
# --- Service control ---
sudo systemctl start myapp.service
sudo systemctl stop myapp.service
sudo systemctl restart myapp.service
sudo systemctl reload myapp.service      # SIGHUP — reopen log file

# --- Status ---
systemctl status myapp.service           # current state + last few log lines
systemctl show myapp.service -p NRestarts  # restart count
systemctl list-timers myapp-monitor.timer  # next monitor run time

# --- Logs ---
journalctl -u myapp.service -f           # follow live
journalctl -u myapp.service -n 50        # last 50 lines
journalctl -u myapp.service -p err       # errors only
journalctl -u myapp.service --since "10 minutes ago"
journalctl -u myapp-monitor.service -n 20  # monitor run history

# --- Diagnostics ---
ls /proc/$(systemctl show myapp.service -p MainPID --value)/fd | wc -l  # FD count
ss -tlnp sport = :8080                   # verify listening
ss -tan | awk '{print $1}' | sort | uniq -c  # connection states
```

---

# Part 13 — Cleanup

Remove everything installed:

```bash
# Stop services and timers
sudo systemctl stop myapp.service
sudo systemctl stop myapp-monitor.timer
sudo systemctl stop myapp-monitor.service

# Disable from boot
sudo systemctl disable myapp.service
sudo systemctl disable myapp-monitor.timer

# Remove unit files
sudo rm /etc/systemd/system/myapp.service
sudo rm /etc/systemd/system/myapp-monitor.service
sudo rm /etc/systemd/system/myapp-monitor.timer

# Reload systemd so it forgets the removed units
sudo systemctl daemon-reload

# Reset any failed state
sudo systemctl reset-failed

# Remove installed files
sudo rm -rf /opt/myapp
sudo rm -rf /var/log/myapp
sudo rm -rf /etc/myapp
sudo rm /etc/logrotate.d/myapp

# Remove service user
sudo userdel myapp

# Verify nothing remains
systemctl status myapp.service 2>&1
# Expected: Unit myapp.service could not be found.
```

---

# WSL2-Specific Notes

A few things behave differently in WSL2:

**Port access:** `localhost` in WSL2 maps to `localhost` on Windows too. So `curl http://localhost:8080/health` works from both WSL2 terminal and Windows browser.

**`/proc` filesystem:** Fully functional in WSL2 — FD checks in the monitor script work correctly.

**Log rotation:** Works correctly. logrotate runs via cron which works in WSL2.

**One WSL2 quirk to watch for:**
```bash
# If systemctl hangs or errors with "Failed to connect to bus"
# it means systemd isn't fully initialized yet — wait 10-15 seconds after wsl startup
systemctl status    # if this works, everything is ready
```

---

> ### *When something doesn't match expected output, that's the learning.*