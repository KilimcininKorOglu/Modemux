# Modemux

Self-hosted mobile proxy management system. Turns USB LTE modems into HTTP and SOCKS5 proxies with real mobile carrier IPs, automated IP rotation, and a web dashboard -- all in a single Go binary.

Built for Linux SBCs (Orange Pi, Raspberry Pi, or any ARM64/x86 device). No Docker required, no external process supervision, no CGO dependencies.

## Why Mobile Proxies

Mobile IPs come from carrier CGNAT pools shared by thousands of real users. Websites cannot block them without blocking legitimate mobile traffic. This gives mobile proxies significantly lower ban rates compared to datacenter or residential alternatives.

| Approach                  | Cost                                |
|---------------------------|-------------------------------------|
| **This project**          | ~$120 hardware + $10/mo (VPS + SIM) |
| SaaS proxy builders       | $50-100/mo + hardware               |
| Phone-based solutions     | $6-10/mo per device + phones        |
| Commercial mobile proxies | $5-15/GB                            |

## Features

- HTTP and SOCKS5 proxy bound to real mobile carrier IP
- IP rotation in ~5 seconds via AT commands (AT+CGATT detach/reattach)
- Support for 1-3 USB modems simultaneously, each with independent proxy ports
- REST API with basic auth for all management operations
- Real-time web dashboard with live modem status and one-click rotation
- Auto-reconnect on modem disconnect with exponential backoff
- SQLite-backed IP rotation history and event logging
- Per-modem cooldown to prevent carrier flagging
- Single static binary -- cross-compiles to ARM64 from any host
- Systemd integration with hardened service unit
- WireGuard tunnel setup for external access via VPS

## Hardware

| Component                                | Price |
|------------------------------------------|-------|
| Orange Pi Zero 3 4GB (or any Linux SBC)  | ~$55  |
| TP-Link UE330 USB hub + Gigabit Ethernet | ~$25  |
| Huawei E3372s-153 USB LTE modem          | ~$20  |
| 32GB microSD                             | ~$10  |
| Enclosure (3D printed or generic)        | ~$8   |

Compatible modems: Huawei E3372h/s, ZTE MF833V, and any USB modem supported by ModemManager. LTE modules (Sierra EM7455, Quectel EC25) provide better speeds.

## Quick Start

### Prerequisites

- Go 1.21+ and [templ](https://templ.guide/) CLI
- On target device: `modem-manager`, `libqmi-utils`, `usb-modeswitch`

### Build

```bash
go install github.com/a-h/templ/cmd/templ@latest

make build           # Production binary
make build-dev       # Development binary (includes mock controller)
make build-arm64     # Cross-compile for ARM64 SBC
```

### Run Without Hardware

```bash
./bin/modemux-dev serve --mock --mock-modems 3
```

Open `http://localhost:8080` for the dashboard. API available at `http://localhost:8080/api/` with credentials `admin:changeme`.

### Run With Real Modems

```bash
cp configs/config.example.yaml config.yaml
# Edit config.yaml: set APN, credentials, ports
./bin/modemux serve
```

## CLI

```
modemux <command> [flags]

Commands:
  detect     Detect connected USB LTE modems
  status     Show modem status (accepts modem index)
  rotate     Rotate IP for a modem (accepts modem index)
  serve      Start proxy server, API, and dashboard
  version    Print version
```

Examples:

```bash
modemux detect                    # List all connected modems
modemux detect --json             # JSON output
modemux status 0                  # Status of modem 0
modemux rotate 0                  # Rotate IP on modem 0
modemux serve --config my.yaml    # Start with custom config
```

## API

All `/api/*` endpoints require basic auth. Responses follow `{"success": bool, "data": ..., "error": "...", "timestamp": "..."}` format.

| Method | Endpoint                  | Description                    |
|--------|---------------------------|--------------------------------|
| GET    | `/healthz`                | Liveness probe (no auth)       |
| GET    | `/readyz`                 | Readiness check (no auth)      |
| GET    | `/api/status`             | System overview and uptime     |
| GET    | `/api/modems`             | List all modems with status    |
| GET    | `/api/modems/:id`         | Detailed modem information     |
| POST   | `/api/modems/:id/rotate`  | Trigger IP rotation            |
| GET    | `/api/modems/:id/history` | Rotation history for one modem |
| GET    | `/api/ip-history`         | Full rotation history          |
| GET    | `/api/events`             | Server-Sent Events stream      |

Rotate example:

```bash
curl -X POST -u admin:changeme http://localhost:8080/api/modems/0/rotate
```

```json
{
  "success": true,
  "data": {
    "modemId": "0",
    "oldIp": "31.223.1.5",
    "newIp": "31.223.2.8",
    "durationMs": 4823,
    "rotationId": 42
  }
}
```

## Dashboard

The web dashboard runs on the same port as the API (default: 8080). Built with Go Templ and HTMX for server-rendered real-time updates without JavaScript frameworks. Protected by cookie-based session authentication.

Features:
- Login page with session-based auth (credentials from `config.yaml`)
- System status bar: uptime, connected modems, total rotations, unique IPs, memory usage
- Modem cards with operator, signal bars, IP, uptime, rotation count, IMEI, cooldown indicator
- One-click IP rotation with in-place card updates and cooldown timer
- IP rotation history table with pagination
- Event log page (`/events`) with per-modem filtering
- Navigation bar with Dashboard, Events links and logout button
- Auto-refresh: modem cards every 10s, status bar every 30s
- Responsive design for mobile and tablet

## Configuration

Copy `configs/config.example.yaml` to `config.yaml` and adjust:

```yaml
server:
  host: "0.0.0.0"
  api_port: 8080
  log_level: "info"              # debug, info, warn, error

auth:
  users:
    admin: "your-secure-password"

modems:
  scan_interval: 30s
  auto_connect: true
  default_apn: "internet"        # Your carrier's APN
  overrides:                     # Per-modem APN overrides
    - imei: "860000000000001"
      apn: "special.apn"
      label: "Turkcell Modem"

proxy:
  http_port_start: 8901          # Modem 0: 8901, Modem 1: 8902, ...
  socks5_port_start: 1081        # Modem 0: 1081, Modem 1: 1082, ...
  auth_required: true
  username: "proxy"
  password: "your-proxy-password"

rotation:
  cooldown: 10s                  # Minimum time between rotations
  timeout: 30s                   # Max wait for new IP after rotation

storage:
  database_path: "./data/modemux.db"
  retention_days: 30
```

Config is auto-discovered from `./config.yaml`, `/etc/modemux/config.yaml`, or `~/.config/modemux/config.yaml`. Override with `--config` flag.

## Deployment

### Automated Install (Recommended)

On the target SBC:

```bash
make build-arm64
scp bin/modemux-linux-arm64 user@device:~/
scp -r scripts/ configs/ user@device:~/
ssh user@device
sudo ./scripts/install.sh
```

The installer is fully interactive -- it walks you through every step:

1. Installs system dependencies (modem-manager, libqmi, usb-modeswitch)
2. Creates service user with dialout group
3. Detects connected USB modems
4. Asks for APN, credentials, ports, and generates `config.yaml`
5. Sets up udev rules for Huawei, ZTE, Quectel, Sierra, Fibocom modems
6. Configures firewall (ufw or firewalld)
7. Optionally sets up a WireGuard VPS tunnel for external access
8. Installs and starts the systemd service
9. Runs a health check to verify everything works

To uninstall: `sudo ./scripts/install.sh --uninstall`

### Docker

```bash
docker build -t modemux .
docker run -d \
  --name modemux \
  --device /dev/ttyUSB0 \
  --device /dev/ttyUSB1 \
  -p 8080:8080 \
  -p 8901-8903:8901-8903 \
  -p 1081-1083:1081-1083 \
  -v ./config.yaml:/etc/modemux/config.yaml \
  -v ./data:/var/lib/modemux \
  modemux
```

### External Access via WireGuard

The install script offers WireGuard tunnel setup as an optional step. If you skipped it during install, run it separately:

```bash
sudo ./scripts/wireguard-setup.sh
```

This generates WireGuard key pairs, creates configs for both the proxy box and VPS, sets up port forwarding rules, and prints the VPS config you need to copy.

## How IP Rotation Works

1. Send `AT+CGATT=0` via modem serial port (detach from network)
2. Wait 2 seconds
3. Send `AT+CGATT=1` (reattach to network)
4. Poll modem status until a new IP is assigned (~3 seconds)
5. Record rotation in SQLite (old IP, new IP, duration)

Total cycle: ~5 seconds. The modem receives a fresh IP from the carrier's CGNAT pool on each reattach.

## Architecture

```
                          +-------------------+
                          |    Web Browser     |
                          |  (Dashboard/API)   |
                          +--------+----------+
                                   |
                          +--------v----------+
                          |   Fiber v3 HTTP   |
                          |  :8080 (API+Web)  |
                          +--------+----------+
                                   |
              +--------------------+--------------------+
              |                    |                     |
     +--------v-------+  +--------v--------+  +--------v--------+
     | Modem Supervisor|  |    Rotator      |  |   SQLite Store  |
     | (scan + manage) |  | (AT+CGATT flow) |  | (history/events)|
     +--------+-------+  +--------+--------+  +-----------------+
              |                    |
     +--------v-------+  +--------v--------+
     |  Worker (per    |  |  Cooldown       |
     |  modem goroutine)|  |  (rate limiter) |
     +--------+-------+  +-----------------+
              |
     +--------v-----------+
     | Proxy Manager       |
     |  HTTP :8901 :8902   |
     |  SOCKS5 :1081 :1082 |
     +--------+------------+
              |
     +--------v--------+
     |  USB LTE Modems  |
     |  (via mmcli/AT)  |
     +------------------+
```

## Project Structure

```
cmd/modemux/       CLI entry point and dependency wiring
internal/
  config/              YAML config loading
  modem/               Controller interface, mmcli wrapper, serial AT,
                       supervisor-worker pattern, state machine, mock (dev only)
  proxy/               HTTP proxy (CONNECT), SOCKS5 proxy, lifecycle manager
  rotation/            AT+CGATT rotation engine, cooldown rate limiter
  store/               SQLite storage, migrations, typed queries
  api/                 Fiber v3 REST API, basic auth, SSE hub
  web/                 Templ+HTMX dashboard, embedded static assets
  system/              Network interface utilities
scripts/               install.sh, wireguard-setup.sh, systemd service
configs/               Example configuration
```

## Tech Stack

| Component | Choice             | Rationale                              |
|-----------|--------------------|----------------------------------------|
| Language  | Go                 | Single binary, cross-compile, low RAM  |
| Web       | Fiber v3           | Fast, Express-like, middleware support |
| Templates | Templ + HTMX       | Type-safe SSR, no JS build step        |
| Database  | modernc.org/sqlite | Pure Go, no CGO, embedded              |
| Serial    | go.bug.st/serial   | Pure Go, AT command interface          |
| SOCKS5    | go-socks5          | Pure Go, custom dialer support         |
| Config    | gopkg.in/yaml.v3   | Standard YAML parsing                  |

## License

MIT
