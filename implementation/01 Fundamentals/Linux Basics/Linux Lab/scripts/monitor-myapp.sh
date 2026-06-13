#!/usr/bin/env bash
# monitor-myapp.sh — Process health monitor for myapp
#
# Responsibilities:
#   - Verify the service is active in systemd
#   - Verify the health endpoint responds with HTTP 200
#   - Check FD usage against system limit
#   - Check RSS memory against configured threshold
#   - Check systemd restart count for crash-loop detection
#   - Emit structured log lines (key=value) readable by both humans and log parsers
#
# This script DETECTS and REPORTS. It does NOT restart the service.
# Restarting is systemd's responsibility. Two systems managing restarts
# creates race conditions and makes incidents harder to diagnose.
#
# Run via: systemd timer (myapp-monitor.timer)
# Logs to: journald (captured automatically since StandardOutput=journal in unit)

set -euo pipefail

# ---------------------------------------------------------------------------
# Configuration — override via environment or EnvironmentFile in unit file
# ---------------------------------------------------------------------------
SERVICE_NAME="${SERVICE_NAME:-myapp}"
HEALTH_URL="${HEALTH_URL:-http://127.0.0.1:8080/health}"
HEALTH_TIMEOUT_SEC="${HEALTH_TIMEOUT_SEC:-5}"
MAX_FD_PERCENT="${MAX_FD_PERCENT:-80}"       # alert if FD usage exceeds 80% of limit
MAX_RSS_MB="${MAX_RSS_MB:-400}"              # alert if RSS exceeds 400MB
MAX_RESTART_COUNT="${MAX_RESTART_COUNT:-3}"  # alert if systemd restarted more than N times
ALERT_LOG="${ALERT_LOG:-/var/log/myapp/monitor-alerts.log}"

# ---------------------------------------------------------------------------
# Logging — structured key=value format, readable by grep and log aggregators
# ---------------------------------------------------------------------------
log() {
    local level="$1"
    shift
    # ISO8601 timestamp + structured fields
    # All output goes to stdout → journald captures it automatically
    echo "timestamp=$(date -u +%Y-%m-%dT%H:%M:%SZ) level=${level} service=${SERVICE_NAME} $*"
}

log_alert() {
    local msg="$1"
    log "ALERT" "$msg"
    # Also append to alert log file for logrotate and external monitoring.
    # tee alternative: we log to both journald (stdout) and file (append).
    echo "$(date -u +%Y-%m-%dT%H:%M:%SZ) ALERT service=${SERVICE_NAME} ${msg}" >> "${ALERT_LOG}"
}

# ---------------------------------------------------------------------------
# Check 1: Is the service active in systemd?
# ---------------------------------------------------------------------------
check_service_active() {
    local state
    state=$(systemctl is-active "${SERVICE_NAME}" 2>/dev/null || true)

    if [[ "${state}" != "active" ]]; then
        log_alert "check=service_active status=FAIL systemd_state=${state}"
        return 1
    fi

    log "INFO" "check=service_active status=OK systemd_state=${state}"
    return 0
}

# ---------------------------------------------------------------------------
# Check 2: Health endpoint responds with HTTP 200
# Checking "is it active" is necessary but not sufficient — the process can be
# running and listening but the application layer can be deadlocked or OOM.
# ---------------------------------------------------------------------------
check_health_endpoint() {
    local http_code
    # --silent suppresses curl progress output
    # --output /dev/null discards body — we only care about status code
    # --max-time prevents hanging if service is slow/deadlocked
    http_code=$(curl \
        --silent \
        --output /dev/null \
        --write-out "%{http_code}" \
        --max-time "${HEALTH_TIMEOUT_SEC}" \
        "${HEALTH_URL}" 2>/dev/null || echo "000")

    if [[ "${http_code}" != "200" ]]; then
        log_alert "check=health_endpoint status=FAIL http_code=${http_code} url=${HEALTH_URL}"
        return 1
    fi

    log "INFO" "check=health_endpoint status=OK http_code=${http_code}"
    return 0
}

# ---------------------------------------------------------------------------
# Check 3: File descriptor usage
# FD exhaustion causes "too many open files" — service stops accepting
# connections while appearing healthy to systemd.
# ---------------------------------------------------------------------------
check_fd_usage() {
    local pid
    pid=$(systemctl show -p MainPID --value "${SERVICE_NAME}" 2>/dev/null || echo "")

    if [[ -z "${pid}" || "${pid}" == "0" ]]; then
        log "WARN" "check=fd_usage status=SKIP reason=pid_not_found"
        return 0
    fi

    local fd_dir="/proc/${pid}/fd"
    if [[ ! -d "${fd_dir}" ]]; then
        log "WARN" "check=fd_usage status=SKIP reason=proc_fd_not_accessible pid=${pid}"
        return 0
    fi

    local open_fds
    open_fds=$(ls "${fd_dir}" 2>/dev/null | wc -l || echo "0")

    # Read the per-process FD limit from /proc — more accurate than system default.
    # Field format: "Max open files  1024  4096  files"
    # Soft limit is column 4, hard limit is column 5.
    local fd_limit
    fd_limit=$(awk '/Max open files/ {print $4}' "/proc/${pid}/limits" 2>/dev/null || echo "1024")

    local usage_percent
    usage_percent=$(( open_fds * 100 / fd_limit ))

    if (( usage_percent >= MAX_FD_PERCENT )); then
        log_alert "check=fd_usage status=FAIL pid=${pid} open_fds=${open_fds} fd_limit=${fd_limit} usage_percent=${usage_percent}"
        return 1
    fi

    log "INFO" "check=fd_usage status=OK pid=${pid} open_fds=${open_fds} fd_limit=${fd_limit} usage_percent=${usage_percent}"
    return 0
}

# ---------------------------------------------------------------------------
# Check 4: RSS memory usage
# RSS growing unboundedly indicates a memory leak.
# We alert before OOM kill — OOM kill leaves no warning in app logs.
# ---------------------------------------------------------------------------
check_memory_usage() {
    local pid
    pid=$(systemctl show -p MainPID --value "${SERVICE_NAME}" 2>/dev/null || echo "")

    if [[ -z "${pid}" || "${pid}" == "0" ]]; then
        log "WARN" "check=memory_usage status=SKIP reason=pid_not_found"
        return 0
    fi

    # /proc/<pid>/status reports VmRSS in kilobytes
    local rss_kb
    rss_kb=$(awk '/VmRSS/ {print $2}' "/proc/${pid}/status" 2>/dev/null || echo "0")

    local rss_mb=$(( rss_kb / 1024 ))

    if (( rss_mb >= MAX_RSS_MB )); then
        log_alert "check=memory_usage status=FAIL pid=${pid} rss_mb=${rss_mb} threshold_mb=${MAX_RSS_MB}"
        return 1
    fi

    log "INFO" "check=memory_usage status=OK pid=${pid} rss_mb=${rss_mb} threshold_mb=${MAX_RSS_MB}"
    return 0
}

# ---------------------------------------------------------------------------
# Check 5: Systemd restart count
# A high restart count means the service is crash-looping.
# systemd may still report "active" between restarts — this catches that.
# ---------------------------------------------------------------------------
check_restart_count() {
    local restart_count
    restart_count=$(systemctl show -p NRestarts --value "${SERVICE_NAME}" 2>/dev/null || echo "0")

    if (( restart_count > MAX_RESTART_COUNT )); then
        log_alert "check=restart_count status=FAIL restart_count=${restart_count} threshold=${MAX_RESTART_COUNT}"
        return 1
    fi

    log "INFO" "check=restart_count status=OK restart_count=${restart_count} threshold=${MAX_RESTART_COUNT}"
    return 0
}

# ---------------------------------------------------------------------------
# Main — run all checks, accumulate failures, exit non-zero if any failed.
# Exiting non-zero causes the systemd timer's service unit to record failure,
# visible via: systemctl status myapp-monitor.service
# ---------------------------------------------------------------------------
main() {
    log "INFO" "msg=monitor_run_start"

    local failed=0

    # Run checks in dependency order:
    # If service is not active, skip endpoint/FD/memory checks — they'll all fail
    # for the wrong reason and produce misleading alerts.
    if ! check_service_active; then
        failed=1
        log "WARN" "msg=skipping_dependent_checks reason=service_not_active"
    else
        check_health_endpoint  || failed=1
        check_fd_usage         || failed=1
        check_memory_usage     || failed=1
        check_restart_count    || failed=1
    fi

    if (( failed == 1 )); then
        log "WARN" "msg=monitor_run_complete overall_status=DEGRADED"
        exit 1
    fi

    log "INFO" "msg=monitor_run_complete overall_status=HEALTHY"
    exit 0
}

main