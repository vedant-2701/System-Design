#!/usr/bin/env bash
# install.sh — Sets up myapp on a fresh Linux system.
#
# Designed to be idempotent: running it multiple times produces the same result.
# This matters because:
#   - Re-running on config changes should not break running services
#   - Failed partial installs should be recoverable by re-running
#
# Must be run as root (uses useradd, systemctl, chown).
# Usage: sudo ./install.sh

set -euo pipefail

# ---------------------------------------------------------------------------
# Constants
# ---------------------------------------------------------------------------
SERVICE_USER="myapp"
SERVICE_GROUP="myapp"
INSTALL_DIR="/opt/myapp"
LOG_DIR="/var/log/myapp"
CONFIG_DIR="/etc/myapp"
DATA_DIR="/opt/myapp/data"

# 1. Get the absolute path of the directory where install.sh is located
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# 2. Get the parent directory (where the 'scripts' folder lives)
PARENT_DIR="$(dirname "$SCRIPT_DIR")"

BINARY_SRC="${PARENT_DIR}/server"   # built Go binary
MONITOR_SRC="${PARENT_DIR}/scripts/monitor-myapp.sh"

SYSTEMD_DIR="/etc/systemd/system"

# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------
log() {
    echo "[install] $*"
}

require_root() {
    if [[ "${EUID}" -ne 0 ]]; then
        echo "ERROR: This script must be run as root (use sudo)" >&2
        exit 1
    fi
}

# ---------------------------------------------------------------------------
# Step 1: Create service user
# An unprivileged dedicated user — service runs with minimal permissions.
# --system: no home directory, no login shell — reduces attack surface.
# The || true makes it idempotent — doesn't fail if user already exists.
# ---------------------------------------------------------------------------
setup_user() {
    log "Creating service user: ${SERVICE_USER}"
    if ! id "${SERVICE_USER}" &>/dev/null; then
        useradd \
            --system \
            --no-create-home \
            --shell /sbin/nologin \
            --comment "myapp service account" \
            "${SERVICE_USER}"
        log "User created: ${SERVICE_USER}"
    else
        log "User already exists: ${SERVICE_USER} (skipping)"
    fi
}

# ---------------------------------------------------------------------------
# Step 2: Create directories with correct ownership and permissions
# Each directory's permissions are intentional:
#   /opt/myapp       → 750: service user owns, group can read, others nothing
#   /var/log/myapp   → 755: writable by service, readable by admins (for log inspection)
#   /etc/myapp       → 750: config with secrets — restricted to service user and root
#   /opt/myapp/data  → 750: writable runtime data
# ---------------------------------------------------------------------------
setup_directories() {
    log "Creating directories"

    mkdir -p "${INSTALL_DIR}/bin"
    mkdir -p "${DATA_DIR}"
    mkdir -p "${LOG_DIR}"
    mkdir -p "${CONFIG_DIR}"

    chown -R "${SERVICE_USER}:${SERVICE_GROUP}" "${INSTALL_DIR}"
    chown -R "${SERVICE_USER}:${SERVICE_GROUP}" "${LOG_DIR}"
    chown -R "root:${SERVICE_GROUP}" "${CONFIG_DIR}"

    chmod 750 "${INSTALL_DIR}"
    chmod 750 "${DATA_DIR}"
    chmod 755 "${LOG_DIR}"
    chmod 750 "${CONFIG_DIR}"

    log "Directories created and permissions set"
}

# ---------------------------------------------------------------------------
# Step 3: Install binaries
# ---------------------------------------------------------------------------
setup_binaries() {
    log "Installing service binary"

    if [[ ! -f "${BINARY_SRC}" ]]; then
        echo "ERROR: Binary not found at ${BINARY_SRC}" >&2
        echo "       Run: go build -o server ./cmd/server/ first" >&2
        exit 1
    fi

    cp "${BINARY_SRC}" "${INSTALL_DIR}/bin/server"
    chown "${SERVICE_USER}:${SERVICE_GROUP}" "${INSTALL_DIR}/bin/server"
    chmod 750 "${INSTALL_DIR}/bin/server"

    log "Installing monitor script"
    mkdir -p "${INSTALL_DIR}/scripts"
    cp "${MONITOR_SRC}" "${INSTALL_DIR}/scripts/monitor-myapp.sh"
    chown "${SERVICE_USER}:${SERVICE_GROUP}" "${INSTALL_DIR}/scripts/monitor-myapp.sh"
    chmod 750 "${INSTALL_DIR}/scripts/monitor-myapp.sh"

    log "Binaries installed"
}

# ---------------------------------------------------------------------------
# Step 4: Install environment config
# Only created if it doesn't exist — avoids overwriting operator customizations
# on reinstall. Use --force flag to override.
# ---------------------------------------------------------------------------
setup_config() {
    local config_file="${CONFIG_DIR}/env"
    local force="${FORCE_CONFIG:-false}"

    if [[ -f "${config_file}" && "${force}" != "true" ]]; then
        log "Config already exists at ${config_file} (skipping — set FORCE_CONFIG=true to overwrite)"
        return 0
    fi

    log "Writing default config to ${config_file}"
    cat > "${config_file}" << 'EOF'
# myapp environment configuration
# Edit these values before starting the service.

APP_PORT=8080
APP_LOG_FILE=/var/log/myapp/myapp.log
APP_SERVICE_NAME=myapp

# Monitor thresholds
HEALTH_URL=http://127.0.0.1:8080/health
HEALTH_TIMEOUT_SEC=5
MAX_FD_PERCENT=80
MAX_RSS_MB=400
MAX_RESTART_COUNT=3
ALERT_LOG=/var/log/myapp/monitor-alerts.log
EOF

    # Config may contain secrets in a real service — restrict to root + group only.
    chown "root:${SERVICE_GROUP}" "${config_file}"
    chmod 640 "${config_file}"

    log "Config written"
}

# ---------------------------------------------------------------------------
# Step 5: Install systemd units
# daemon-reload is mandatory after installing new unit files — without it,
# systemd keeps the cached (non-existent) version and start/enable fail.
# ---------------------------------------------------------------------------
setup_systemd() {
    log "Installing systemd unit files"

    cp "${PARENT_DIR}/systemd/myapp.service"         "${SYSTEMD_DIR}/myapp.service"
    cp "${PARENT_DIR}/systemd/myapp-monitor.service" "${SYSTEMD_DIR}/myapp-monitor.service"
    cp "${PARENT_DIR}/systemd/myapp-monitor.timer"   "${SYSTEMD_DIR}/myapp-monitor.timer"

    log "Reloading systemd daemon"
    systemctl daemon-reload

    log "Enabling service and timer"
    systemctl enable myapp.service
    systemctl enable myapp-monitor.timer

    log "Systemd units installed and enabled"
}

# ---------------------------------------------------------------------------
# Step 6: Install logrotate config
# ---------------------------------------------------------------------------
setup_logrotate() {
    log "Installing logrotate config"
    cp "${PARENT_DIR}/logrotate.d/myapp" /etc/logrotate.d/myapp
    chmod 644 /etc/logrotate.d/myapp
    log "Logrotate config installed"
}

# ---------------------------------------------------------------------------
# Step 7: Start services (optional — only if --start flag passed)
# Separating install from start is intentional:
# Allows operators to review config before first start.
# ---------------------------------------------------------------------------
start_services() {
    log "Starting myapp service"
    systemctl start myapp.service

    log "Starting myapp monitor timer"
    systemctl start myapp-monitor.timer

    log "Verifying service is active"
    sleep 2
    if systemctl is-active --quiet myapp.service; then
        log "myapp.service is active"
    else
        echo "ERROR: myapp.service failed to start" >&2
        systemctl status myapp.service >&2
        exit 1
    fi
}

# ---------------------------------------------------------------------------
# Main
# ---------------------------------------------------------------------------
main() {
    require_root

    log "Starting myapp installation"

    setup_user
    setup_directories
    setup_binaries
    setup_config
    setup_systemd
    setup_logrotate

    log ""
    log "Installation complete."
    log ""
    log "Next steps:"
    log "  1. Review config:       ${CONFIG_DIR}/env"
    log "  2. Start service:       sudo systemctl start myapp.service"
    log "  3. Start monitor timer: sudo systemctl start myapp-monitor.timer"
    log "  4. Check status:        systemctl status myapp.service"
    log "  5. Follow logs:         journalctl -u myapp.service -f"
    log "  6. Monitor logs:        journalctl -u myapp-monitor.service -f"

    # Auto-start if --start flag is passed
    if [[ "${1:-}" == "--start" ]]; then
        start_services
    fi
}

main "$@"