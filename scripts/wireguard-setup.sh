#!/usr/bin/env bash
set -euo pipefail

GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

log()  { echo -e "${GREEN}[+]${NC} $1"; }
warn() { echo -e "${YELLOW}[!]${NC} $1"; }

echo "======================================"
echo "  Modemux WireGuard Tunnel Setup"
echo "======================================"
echo ""

if [ "$(id -u)" -ne 0 ]; then
    echo "Run as root: sudo $0"
    exit 1
fi

if ! command -v wg &>/dev/null; then
    log "Installing WireGuard..."
    if command -v apt-get &>/dev/null; then
        apt-get update -qq && apt-get install -y -qq wireguard
    elif command -v dnf &>/dev/null; then
        dnf install -y -q wireguard-tools
    elif command -v pacman &>/dev/null; then
        pacman -Sy --noconfirm wireguard-tools
    fi
fi

read -rp "VPS public IP or hostname: " VPS_ENDPOINT
read -rp "VPS WireGuard port [51820]: " VPS_PORT
VPS_PORT=${VPS_PORT:-51820}
read -rp "Tunnel subnet [10.0.0.0/24]: " TUNNEL_SUBNET
TUNNEL_SUBNET=${TUNNEL_SUBNET:-10.0.0.0/24}

PROXY_IP="10.0.0.2"
VPS_IP="10.0.0.1"

log "Generating WireGuard keys..."
PROXY_PRIVKEY=$(wg genkey)
PROXY_PUBKEY=$(echo "$PROXY_PRIVKEY" | wg pubkey)
VPS_PRIVKEY=$(wg genkey)
VPS_PUBKEY=$(echo "$VPS_PRIVKEY" | wg pubkey)

log "Writing proxy box config: /etc/wireguard/wg-proxy.conf"
cat > /etc/wireguard/wg-proxy.conf << EOF
[Interface]
PrivateKey = $PROXY_PRIVKEY
Address = $PROXY_IP/24
DNS = 1.1.1.1

[Peer]
PublicKey = $VPS_PUBKEY
Endpoint = $VPS_ENDPOINT:$VPS_PORT
AllowedIPs = $VPS_IP/32
PersistentKeepalive = 25
EOF
chmod 600 /etc/wireguard/wg-proxy.conf

echo ""
log "VPS config (copy to your VPS at /etc/wireguard/wg-proxy.conf):"
echo "--------------------------------------------------------------"
cat << EOF
[Interface]
PrivateKey = $VPS_PRIVKEY
Address = $VPS_IP/24
ListenPort = $VPS_PORT

# Enable IP forwarding and NAT
PostUp = iptables -A FORWARD -i wg-proxy -j ACCEPT; iptables -t nat -A POSTROUTING -o eth0 -j MASQUERADE
PostDown = iptables -D FORWARD -i wg-proxy -j ACCEPT; iptables -t nat -D POSTROUTING -o eth0 -j MASQUERADE

[Peer]
PublicKey = $PROXY_PUBKEY
AllowedIPs = $PROXY_IP/32
EOF
echo "--------------------------------------------------------------"
echo ""

log "Port forwarding commands for VPS (add to PostUp):"
echo "  iptables -t nat -A PREROUTING -p tcp --dport 8080 -j DNAT --to-destination $PROXY_IP:8080"
echo "  iptables -t nat -A PREROUTING -p tcp --dport 8901 -j DNAT --to-destination $PROXY_IP:8901"
echo "  iptables -t nat -A PREROUTING -p tcp --dport 1081 -j DNAT --to-destination $PROXY_IP:1081"
echo ""

read -rp "Start WireGuard tunnel now? [y/N]: " START_NOW
if [[ "$START_NOW" =~ ^[Yy]$ ]]; then
    log "Starting tunnel..."
    wg-quick up wg-proxy
    systemctl enable wg-quick@wg-proxy
    log "Tunnel active! Test: ping $VPS_IP"
else
    echo ""
    echo "  Start later with: sudo wg-quick up wg-proxy"
    echo "  Enable on boot:   sudo systemctl enable wg-quick@wg-proxy"
fi
