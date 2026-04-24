#!/usr/bin/env bash
set -euo pipefail

BINARY_NAME="modemux"
INSTALL_DIR="/usr/local/bin"
CONFIG_DIR="/etc/modemux"
DATA_DIR="/var/lib/modemux"
LOG_DIR="/var/log/modemux"
SERVICE_USER="modemux"
SERVICE_FILE="/etc/systemd/system/modemux.service"
UDEV_RULES="/etc/udev/rules.d/99-usb-modem.rules"
GITHUB_REPO="KilimcininKorOglu/modemux"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
BOLD='\033[1m'
NC='\033[0m'

log()  { echo -e "${GREEN}[+]${NC} $1"; }
warn() { echo -e "${YELLOW}[!]${NC} $1"; }
err()  { echo -e "${RED}[x]${NC} $1"; exit 1; }
info() { echo -e "${CYAN}[i]${NC} $1"; }
step() { echo -e "\n${BOLD}=== $1 ===${NC}\n"; }

prompt() {
    local varname="$1" message="$2" default="$3"
    local input
    read -rp "$(echo -e "${CYAN}?>>${NC} ${message} [${default}]: ")" input
    eval "$varname=\"${input:-$default}\""
}

prompt_password() {
    local varname="$1" message="$2"
    local pass1 pass2
    while true; do
        read -srp "$(echo -e "${CYAN}?>>${NC} ${message}: ")" pass1
        echo ""
        read -srp "$(echo -e "${CYAN}?>>${NC} Confirm: ")" pass2
        echo ""
        if [ "$pass1" = "$pass2" ] && [ -n "$pass1" ]; then
            eval "$varname=\"$pass1\""
            return
        fi
        warn "Passwords don't match or empty. Try again."
    done
}

prompt_yn() {
    local message="$1" default="${2:-y}"
    local input
    if [ "$default" = "y" ]; then
        read -rp "$(echo -e "${CYAN}?>>${NC} ${message} [Y/n]: ")" input
        input="${input:-y}"
    else
        read -rp "$(echo -e "${CYAN}?>>${NC} ${message} [y/N]: ")" input
        input="${input:-n}"
    fi
    [[ "$input" =~ ^[Yy]$ ]]
}

banner() {
    echo -e "${BOLD}"
    cat << 'EOF'
  __  __       _     _ _      ____
 |  \/  | ___ | |__ (_) | ___|  _ \ _ __ _____  ___   _
 | |\/| |/ _ \| '_ \| | |/ _ \ |_) | '__/ _ \ \/ / | | |
 | |  | | (_) | |_) | | |  __/  __/| | | (_) >  <| |_| |
 |_|  |_|\___/|_.__/|_|_|\___|_|   |_|  \___/_/\_\\__, |
                                                    |___/
EOF
    echo -e "${NC}"
    echo "  Automated Installer"
    echo "  ────────────────────────────────────────"
    echo ""
}

detect_arch() {
    ARCH=$(uname -m)
    case "$ARCH" in
        aarch64|arm64) BINARY_SUFFIX="linux-arm64" ;;
        x86_64|amd64)  BINARY_SUFFIX="linux-amd64" ;;
        *)             err "Unsupported architecture: $ARCH" ;;
    esac
    log "Architecture: $ARCH ($BINARY_SUFFIX)"
}

detect_package_manager() {
    if command -v apt-get &>/dev/null; then
        PKG_MGR="apt"
    elif command -v dnf &>/dev/null; then
        PKG_MGR="dnf"
    elif command -v pacman &>/dev/null; then
        PKG_MGR="pacman"
    elif command -v apk &>/dev/null; then
        PKG_MGR="apk"
    else
        PKG_MGR="unknown"
    fi
    log "Package manager: $PKG_MGR"
}

install_system_deps() {
    step "System Dependencies"

    case "$PKG_MGR" in
        apt)
            log "Updating package lists..."
            apt-get update -qq
            log "Installing modem-manager, libqmi-utils, usb-modeswitch, curl..."
            apt-get install -y -qq modem-manager libqmi-utils usb-modeswitch curl jq
            ;;
        dnf)
            log "Installing ModemManager, libqmi, usb_modeswitch, curl..."
            dnf install -y -q ModemManager libqmi usb_modeswitch curl jq
            ;;
        pacman)
            log "Installing modemmanager, libqmi, usb_modeswitch, curl..."
            pacman -Sy --noconfirm modemmanager libqmi usb_modeswitch curl jq
            ;;
        apk)
            log "Installing modemmanager, libqmi, usb-modeswitch, curl..."
            apk add --no-cache modemmanager libqmi usb-modeswitch curl jq
            ;;
        *)
            warn "Unknown package manager. Install these manually:"
            warn "  modem-manager, libqmi-utils, usb-modeswitch, curl, jq"
            ;;
    esac

    log "Enabling ModemManager service..."
    systemctl enable --now ModemManager 2>/dev/null || warn "ModemManager service not found"
}

create_user() {
    step "Service User"

    if id "$SERVICE_USER" &>/dev/null; then
        log "User $SERVICE_USER already exists"
    else
        log "Creating system user: $SERVICE_USER"
        useradd --system --no-create-home --shell /usr/sbin/nologin "$SERVICE_USER"
    fi

    usermod -aG dialout "$SERVICE_USER" 2>/dev/null || true
    log "User $SERVICE_USER added to dialout group"
}

create_directories() {
    step "Directories"

    mkdir -p "$CONFIG_DIR" "$DATA_DIR" "$LOG_DIR"
    chown "$SERVICE_USER:$SERVICE_USER" "$DATA_DIR" "$LOG_DIR"
    chmod 750 "$DATA_DIR" "$LOG_DIR"
    log "Created: $CONFIG_DIR, $DATA_DIR, $LOG_DIR"
}

install_binary() {
    step "Binary Installation"

    local binary_path=""

    if [ -f "bin/${BINARY_NAME}-${BINARY_SUFFIX}" ]; then
        binary_path="bin/${BINARY_NAME}-${BINARY_SUFFIX}"
    elif [ -f "bin/${BINARY_NAME}" ]; then
        binary_path="bin/${BINARY_NAME}"
    elif [ -f "./${BINARY_NAME}" ]; then
        binary_path="./${BINARY_NAME}"
    fi

    if [ -z "$binary_path" ]; then
        log "Binary not found locally. Downloading latest release..."
        download_latest_release
        return
    fi

    log "Installing: $binary_path -> $INSTALL_DIR/$BINARY_NAME"
    cp "$binary_path" "$INSTALL_DIR/$BINARY_NAME"
    chmod 755 "$INSTALL_DIR/$BINARY_NAME"

    local ver
    ver=$("$INSTALL_DIR/$BINARY_NAME" version 2>/dev/null || echo "unknown")
    log "Installed: $ver"
}

download_latest_release() {
    local download_url="https://github.com/$GITHUB_REPO/releases/latest/download/${BINARY_NAME}_${BINARY_SUFFIX}.tar.gz"
    local tmp_dir

    tmp_dir=$(mktemp -d)
    trap "rm -rf $tmp_dir" RETURN

    log "Downloading from: $download_url"
    if curl -fsSL "$download_url" -o "$tmp_dir/release.tar.gz" 2>/dev/null; then
        tar -xzf "$tmp_dir/release.tar.gz" -C "$tmp_dir"
        cp "$tmp_dir/$BINARY_NAME" "$INSTALL_DIR/$BINARY_NAME"
        chmod 755 "$INSTALL_DIR/$BINARY_NAME"
        log "Downloaded and installed successfully"
    else
        warn "Download failed. Build manually: make build-arm64"
        warn "Then re-run this script."
        err "No binary available"
    fi
}

detect_modems() {
    step "Modem Detection"

    if ! command -v mmcli &>/dev/null; then
        warn "mmcli not found. Skipping modem detection."
        DETECTED_MODEMS=0
        return
    fi

    sleep 2

    local modem_list
    modem_list=$(mmcli -L 2>/dev/null || true)

    if echo "$modem_list" | grep -q "/Modem/"; then
        DETECTED_MODEMS=$(echo "$modem_list" | grep -c "/Modem/")
        log "Detected $DETECTED_MODEMS modem(s):"
        echo ""

        for i in $(seq 0 $((DETECTED_MODEMS - 1))); do
            local model manufacturer
            model=$(mmcli -m "$i" -J 2>/dev/null | jq -r '.modem.generic.model // "Unknown"' 2>/dev/null || echo "Unknown")
            manufacturer=$(mmcli -m "$i" -J 2>/dev/null | jq -r '.modem.generic.manufacturer // "Unknown"' 2>/dev/null || echo "Unknown")
            info "  [$i] $manufacturer $model"
        done
        echo ""
    else
        DETECTED_MODEMS=0
        warn "No USB modems detected."
        warn "Plug in a modem and restart the service later."
    fi
}

configure_interactively() {
    step "Configuration"

    if [ -f "$CONFIG_DIR/config.yaml" ]; then
        if prompt_yn "Config already exists at $CONFIG_DIR/config.yaml. Overwrite?" "n"; then
            log "Backing up existing config..."
            cp "$CONFIG_DIR/config.yaml" "$CONFIG_DIR/config.yaml.bak.$(date +%s)"
        else
            log "Keeping existing config"
            return
        fi
    fi

    log "Let's configure Modemux."
    echo ""

    prompt CONF_PORT "API & Dashboard port" "8080"
    prompt CONF_APN "Default carrier APN" "internet"

    echo ""
    info "Admin credentials (for API & Dashboard):"
    prompt CONF_ADMIN_USER "Admin username" "admin"
    prompt_password CONF_ADMIN_PASS "Admin password"

    echo ""
    info "Proxy credentials (for HTTP/SOCKS5 clients):"
    prompt CONF_PROXY_USER "Proxy username" "proxy"
    prompt_password CONF_PROXY_PASS "Proxy password"

    prompt CONF_HTTP_PORT "HTTP proxy starting port" "8901"
    prompt CONF_SOCKS5_PORT "SOCKS5 proxy starting port" "1081"
    prompt CONF_COOLDOWN "Rotation cooldown" "10s"

    local conf_loglevel="info"
    if prompt_yn "Enable debug logging?" "n"; then
        conf_loglevel="debug"
    fi

    log "Writing config to $CONFIG_DIR/config.yaml..."

    cat > "$CONFIG_DIR/config.yaml" << YAML
server:
  host: "0.0.0.0"
  api_port: ${CONF_PORT}
  log_level: "${conf_loglevel}"

auth:
  users:
    ${CONF_ADMIN_USER}: "${CONF_ADMIN_PASS}"

modems:
  scan_interval: 30s
  auto_connect: true
  default_apn: "${CONF_APN}"
  overrides: []

proxy:
  http_port_start: ${CONF_HTTP_PORT}
  socks5_port_start: ${CONF_SOCKS5_PORT}
  auth_required: true
  username: "${CONF_PROXY_USER}"
  password: "${CONF_PROXY_PASS}"

rotation:
  cooldown: ${CONF_COOLDOWN}
  timeout: 30s
  auto_rotate: false
  auto_interval: 5m

storage:
  database_path: "${DATA_DIR}/modemux.db"
  retention_days: 30

wireguard:
  enabled: false
  vps_endpoint: ""
  vps_public_key: ""
  local_private_key: ""
  tunnel_subnet: "10.0.0.0/24"
YAML

    chown "$SERVICE_USER:$SERVICE_USER" "$CONFIG_DIR/config.yaml"
    chmod 640 "$CONFIG_DIR/config.yaml"
    log "Config written successfully"
}

install_systemd_service() {
    step "Systemd Service"

    cat > "$SERVICE_FILE" << 'SERVICE'
[Unit]
Description=Mobile Proxy Management Service
Documentation=https://github.com/KilimcininKorOglu/modemux
After=network-online.target ModemManager.service
Wants=network-online.target ModemManager.service

[Service]
Type=simple
User=modemux
Group=modemux
ExecStart=/usr/local/bin/modemux serve --config /etc/modemux/config.yaml
WorkingDirectory=/var/lib/modemux
Restart=always
RestartSec=5

LimitNOFILE=65536

NoNewPrivileges=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/var/lib/modemux
PrivateTmp=true
ProtectKernelTunables=true
ProtectControlGroups=true
RestrictSUIDSGID=true

SupplementaryGroups=dialout

StandardOutput=journal
StandardError=journal
SyslogIdentifier=modemux

[Install]
WantedBy=multi-user.target
SERVICE

    systemctl daemon-reload
    log "Systemd service installed"
}

install_udev_rules() {
    step "USB Modem udev Rules"

    cat > "$UDEV_RULES" << 'UDEV'
# Huawei USB modems
ACTION=="add", SUBSYSTEM=="tty", ATTRS{idVendor}=="12d1", GROUP="dialout", MODE="0660"
ACTION=="add", SUBSYSTEM=="net", ATTRS{idVendor}=="12d1", GROUP="dialout", MODE="0660"

# ZTE USB modems
ACTION=="add", SUBSYSTEM=="tty", ATTRS{idVendor}=="19d2", GROUP="dialout", MODE="0660"
ACTION=="add", SUBSYSTEM=="net", ATTRS{idVendor}=="19d2", GROUP="dialout", MODE="0660"

# Quectel USB modems
ACTION=="add", SUBSYSTEM=="tty", ATTRS{idVendor}=="2c7c", GROUP="dialout", MODE="0660"
ACTION=="add", SUBSYSTEM=="net", ATTRS{idVendor}=="2c7c", GROUP="dialout", MODE="0660"

# Sierra Wireless
ACTION=="add", SUBSYSTEM=="tty", ATTRS{idVendor}=="1199", GROUP="dialout", MODE="0660"
ACTION=="add", SUBSYSTEM=="net", ATTRS{idVendor}=="1199", GROUP="dialout", MODE="0660"

# Fibocom
ACTION=="add", SUBSYSTEM=="tty", ATTRS{idVendor}=="2cb7", GROUP="dialout", MODE="0660"
ACTION=="add", SUBSYSTEM=="net", ATTRS{idVendor}=="2cb7", GROUP="dialout", MODE="0660"
UDEV

    udevadm control --reload-rules
    udevadm trigger 2>/dev/null || true
    log "udev rules installed for Huawei, ZTE, Quectel, Sierra, Fibocom"
}

configure_firewall() {
    step "Firewall"

    if command -v ufw &>/dev/null; then
        log "Configuring UFW..."
        ufw allow "$CONF_PORT"/tcp comment "Modemux API" 2>/dev/null || true
        ufw allow "$CONF_HTTP_PORT":"$((CONF_HTTP_PORT + 2))"/tcp comment "Modemux HTTP" 2>/dev/null || true
        ufw allow "$CONF_SOCKS5_PORT":"$((CONF_SOCKS5_PORT + 2))"/tcp comment "Modemux SOCKS5" 2>/dev/null || true
        log "UFW rules added"
    elif command -v firewall-cmd &>/dev/null; then
        log "Configuring firewalld..."
        firewall-cmd --permanent --add-port="$CONF_PORT"/tcp 2>/dev/null || true
        firewall-cmd --permanent --add-port="$CONF_HTTP_PORT"-"$((CONF_HTTP_PORT + 2))"/tcp 2>/dev/null || true
        firewall-cmd --permanent --add-port="$CONF_SOCKS5_PORT"-"$((CONF_SOCKS5_PORT + 2))"/tcp 2>/dev/null || true
        firewall-cmd --reload 2>/dev/null || true
        log "firewalld rules added"
    else
        info "No firewall detected. Ensure ports $CONF_PORT, $CONF_HTTP_PORT-$((CONF_HTTP_PORT+2)), $CONF_SOCKS5_PORT-$((CONF_SOCKS5_PORT+2)) are open."
    fi
}

start_service() {
    step "Starting Service"

    systemctl enable modemux
    log "Service enabled on boot"

    systemctl start modemux
    log "Service started"

    sleep 3

    if systemctl is-active --quiet modemux; then
        log "Service is running"
    else
        warn "Service failed to start. Check logs:"
        warn "  journalctl -u modemux -n 20 --no-pager"
        return
    fi

    local health
    health=$(curl -sf "http://127.0.0.1:${CONF_PORT}/healthz" 2>/dev/null || echo "")
    if echo "$health" | grep -q "ok"; then
        log "Health check passed"
    else
        warn "Health check failed. Service may still be starting."
    fi
}

setup_wireguard_optional() {
    step "WireGuard VPS Tunnel (Optional)"

    info "WireGuard creates a tunnel to a VPS with a public IP so you"
    info "can access your proxy from anywhere, not just your local network."
    echo ""

    if ! prompt_yn "Set up a WireGuard VPS tunnel?" "n"; then
        log "Skipping WireGuard setup"
        WG_ENABLED="false"
        return
    fi

    if ! command -v wg &>/dev/null; then
        log "Installing WireGuard..."
        case "$PKG_MGR" in
            apt)    apt-get install -y -qq wireguard ;;
            dnf)    dnf install -y -q wireguard-tools ;;
            pacman) pacman -Sy --noconfirm wireguard-tools ;;
            apk)    apk add --no-cache wireguard-tools ;;
            *)      warn "Install wireguard-tools manually"; return ;;
        esac
    fi

    echo ""
    prompt WG_VPS_ENDPOINT "VPS public IP or hostname" ""
    if [ -z "$WG_VPS_ENDPOINT" ]; then
        warn "No VPS endpoint provided. Skipping WireGuard."
        WG_ENABLED="false"
        return
    fi

    prompt WG_VPS_PORT "VPS WireGuard listen port" "51820"
    prompt WG_PROXY_IP "Proxy box tunnel IP" "10.0.0.2"
    prompt WG_VPS_IP "VPS tunnel IP" "10.0.0.1"

    log "Generating WireGuard keys..."
    local proxy_privkey proxy_pubkey vps_privkey vps_pubkey
    proxy_privkey=$(wg genkey)
    proxy_pubkey=$(echo "$proxy_privkey" | wg pubkey)
    vps_privkey=$(wg genkey)
    vps_pubkey=$(echo "$vps_privkey" | wg pubkey)

    log "Writing proxy box config: /etc/wireguard/wg-proxy.conf"
    mkdir -p /etc/wireguard
    cat > /etc/wireguard/wg-proxy.conf << WGEOF
[Interface]
PrivateKey = $proxy_privkey
Address = $WG_PROXY_IP/24

[Peer]
PublicKey = $vps_pubkey
Endpoint = $WG_VPS_ENDPOINT:$WG_VPS_PORT
AllowedIPs = $WG_VPS_IP/32
PersistentKeepalive = 25
WGEOF
    chmod 600 /etc/wireguard/wg-proxy.conf

    echo ""
    echo -e "${BOLD}Copy this config to your VPS at /etc/wireguard/wg-proxy.conf:${NC}"
    echo "────────────────────────────────────────"
    cat << VPSEOF
[Interface]
PrivateKey = $vps_privkey
Address = $WG_VPS_IP/24
ListenPort = $WG_VPS_PORT
PostUp = iptables -A FORWARD -i wg-proxy -j ACCEPT; iptables -t nat -A POSTROUTING -o eth0 -j MASQUERADE; iptables -t nat -A PREROUTING -p tcp --dport $CONF_PORT -j DNAT --to-destination $WG_PROXY_IP:$CONF_PORT; iptables -t nat -A PREROUTING -p tcp --dport $CONF_HTTP_PORT -j DNAT --to-destination $WG_PROXY_IP:$CONF_HTTP_PORT; iptables -t nat -A PREROUTING -p tcp --dport $CONF_SOCKS5_PORT -j DNAT --to-destination $WG_PROXY_IP:$CONF_SOCKS5_PORT
PostDown = iptables -D FORWARD -i wg-proxy -j ACCEPT; iptables -t nat -D POSTROUTING -o eth0 -j MASQUERADE

[Peer]
PublicKey = $proxy_pubkey
AllowedIPs = $WG_PROXY_IP/32
VPSEOF
    echo "────────────────────────────────────────"
    echo ""

    if prompt_yn "Start WireGuard tunnel now?" "y"; then
        wg-quick up wg-proxy 2>/dev/null || warn "Failed to start tunnel. Configure VPS first, then: wg-quick up wg-proxy"
        systemctl enable wg-quick@wg-proxy 2>/dev/null || true
        log "WireGuard tunnel started and enabled on boot"
    else
        info "Start later: sudo wg-quick up wg-proxy"
        info "Enable on boot: sudo systemctl enable wg-quick@wg-proxy"
    fi

    WG_ENABLED="true"

    sed -i "s|wireguard:|wireguard:|" "$CONFIG_DIR/config.yaml"
    sed -i "s|enabled: false|enabled: true|" "$CONFIG_DIR/config.yaml"
    sed -i "s|vps_endpoint: \"\"|vps_endpoint: \"$WG_VPS_ENDPOINT:$WG_VPS_PORT\"|" "$CONFIG_DIR/config.yaml"
    sed -i "s|vps_public_key: \"\"|vps_public_key: \"$vps_pubkey\"|" "$CONFIG_DIR/config.yaml"
    sed -i "s|local_private_key: \"\"|local_private_key: \"$proxy_privkey\"|" "$CONFIG_DIR/config.yaml"
    log "WireGuard config updated in $CONFIG_DIR/config.yaml"
}

get_device_ip() {
    local ip
    ip=$(ip -4 route get 1.1.1.1 2>/dev/null | grep -oP 'src \K[0-9.]+' || hostname -I 2>/dev/null | awk '{print $1}' || echo "localhost")
    echo "$ip"
}

print_summary() {
    local device_ip
    device_ip=$(get_device_ip)

    echo ""
    echo -e "${BOLD}────────────────────────────────────────${NC}"
    echo -e "${GREEN}${BOLD}  Installation Complete${NC}"
    echo -e "${BOLD}────────────────────────────────────────${NC}"
    echo ""
    echo -e "  Dashboard:    ${CYAN}http://${device_ip}:${CONF_PORT}${NC}"
    echo -e "  API:          ${CYAN}http://${device_ip}:${CONF_PORT}/api/${NC}"
    echo -e "  Auth:         ${CONF_ADMIN_USER} / ********"
    echo ""
    echo -e "  HTTP Proxy:   ${device_ip}:${CONF_HTTP_PORT}  (user: ${CONF_PROXY_USER})"
    echo -e "  SOCKS5 Proxy: ${device_ip}:${CONF_SOCKS5_PORT}  (user: ${CONF_PROXY_USER})"
    echo ""
    echo -e "  Modems found: ${DETECTED_MODEMS}"
    if [ "${WG_ENABLED:-false}" = "true" ]; then
        echo -e "  WireGuard:    ${GREEN}active${NC} (tunnel to ${WG_VPS_ENDPOINT})"
    else
        echo -e "  WireGuard:    skipped"
    fi
    echo ""
    echo -e "  ${BOLD}Useful commands:${NC}"
    echo -e "    Status:       sudo systemctl status modemux"
    echo -e "    Logs:         journalctl -u modemux -f"
    echo -e "    Restart:      sudo systemctl restart modemux"
    echo -e "    Edit config:  sudo nano ${CONFIG_DIR}/config.yaml"
    echo -e "    Test rotate:  curl -X POST -u ${CONF_ADMIN_USER}:*** http://127.0.0.1:${CONF_PORT}/api/modems/0/rotate"
    echo ""

    if [ "$DETECTED_MODEMS" -eq 0 ]; then
        echo -e "  ${YELLOW}No modems detected. Plug in a USB modem and restart:${NC}"
        echo -e "    sudo systemctl restart modemux"
        echo ""
    fi
}

uninstall() {
    step "Uninstalling Modemux"

    systemctl stop modemux 2>/dev/null || true
    systemctl disable modemux 2>/dev/null || true

    rm -f "$SERVICE_FILE"
    rm -f "$INSTALL_DIR/$BINARY_NAME"
    rm -f "$UDEV_RULES"

    systemctl daemon-reload
    udevadm control --reload-rules 2>/dev/null || true

    log "Binary, service, and udev rules removed"

    if [ -f /etc/wireguard/wg-proxy.conf ]; then
        if prompt_yn "Remove WireGuard tunnel config?" "n"; then
            wg-quick down wg-proxy 2>/dev/null || true
            systemctl disable wg-quick@wg-proxy 2>/dev/null || true
            rm -f /etc/wireguard/wg-proxy.conf
            log "WireGuard config removed"
        fi
    fi

    if prompt_yn "Remove config and data ($CONFIG_DIR, $DATA_DIR)?" "n"; then
        rm -rf "$CONFIG_DIR" "$DATA_DIR" "$LOG_DIR"
        log "Config and data removed"
    else
        info "Config and data preserved at $CONFIG_DIR and $DATA_DIR"
    fi

    if prompt_yn "Remove service user ($SERVICE_USER)?" "n"; then
        userdel "$SERVICE_USER" 2>/dev/null || true
        log "User removed"
    fi

    echo ""
    log "Uninstall complete"
}

main() {
    banner

    if [ "$(id -u)" -ne 0 ]; then
        err "This script must be run as root: sudo $0"
    fi

    if [ "${1:-}" = "--uninstall" ] || [ "${1:-}" = "uninstall" ]; then
        uninstall
        exit 0
    fi

    detect_arch
    detect_package_manager

    install_system_deps
    create_user
    create_directories
    install_binary
    detect_modems
    configure_interactively
    install_systemd_service
    install_udev_rules
    configure_firewall
    setup_wireguard_optional
    start_service
    print_summary
}

main "$@"
