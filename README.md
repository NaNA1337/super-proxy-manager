# super-proxy-manager

Modern, independent Web Manager & Control Panel for [super-proxy](https://github.com/NaNA1337/super-proxy).

Designed as a high-density, NOC-style control panel that communicates strictly through the `super-proxy` daemon's localhost TLS Agent API without altering core daemon routing, FSM, or failover logic.

```text
Browser (HTTPS)
      │
      ▼
┌──────────────────────────────────────────────────────────┐
│  Web Manager (super-proxy-web)                           │
│  Port: :8080 (Localhost / Private Network or Behind Caddy)│
│  ├── React 18 + TypeScript + Vite + Tailwind (NOC Dark)  │
│  │   ├── Dashboard (3-second NOC status overview)        │
│  │   ├── Nodes Inventory & Transparent Score Factors     │
│  │   ├── Slot Controller (Safe daemon-authorized actions)│
│  │   ├── Policy Routing & Fail-closed Leak Protection    │
│  │   ├── Prometheus Metrics Real-time Visualizer         │
│  │   ├── System Events Stream (INFO / WARN / ERROR)      │
│  │   ├── ShareLink Generator (Pure Client QR Code)       │
│  │   ├── Subscriptions Manager (SHA-256, One-time Token) │
│  │   ├── Read-Only Settings & Architecture Inspector     │
│  │   └── Mutation Audit Trail                            │
│  └── Go Backend (Single Binary Embed)                    │
│      ├── bcrypt password auth, CSRF token, Rate limiting │
│      ├── Subscriptions (SHA-256 store, [REDACTED] logs)  │
│      ├── ShareLink Protocol Adapters & Secret Validator  │
│      └── Daemon Agent API Forwarding & Health Watcher    │
└─────────────────────────────┬────────────────────────────┘
                              │
                    TLS Mutual / Token Auth
                    https://127.0.0.1:60000
                              ▼
┌──────────────────────────────────────────────────────────┐
│  super-proxy Daemon (Core Engine)                        │
│  ├── FSM, Node Scoring, Failover, Reputation             │
│  ├── Policy Routing (fwmark 0x100-0x102, Table 100-102)  │
│  ├── Xray Core (SOCKS5 / VLESS Reality Inbound)           │
│  └── OpenVPN Gate Multi-Tunnel Egress                     │
└──────────────────────────────────────────────────────────┘
```

---

## Features

- **3-Second NOC Dashboard**: Real-time status answering daemon liveness, active slots (0, 1, 2), egress country/IP/ASN, live traffic counters, tunnel uptime, and DNS/IPv6 fail-closed leak protection.
- **Transparent Node Scoring**: Full visibility into node reputation breakdown (`+ low abuse risk`, `+ high throughput`, `- hosting ASN`). VPN Gate egress is treated neutrally as a network trait rather than falsely flagged as malicious.
- **Safe Slot Controller**: Only daemon-authorized rotation actions (`POST /api/v1/slots/{id}/switch`) are exposed to administrators.
- **Kernel Policy Routing View**: Inspect fwmark to routing table mappings (`fwmark 0x100 -> table 100 -> tun100`) and packet counter flow.
- **Prometheus Telemetry**: Real-time visualization of slot health, tunnel latency, throughput, and failover events.
- **Multi-Protocol ShareLinks**: Standards-compliant VLESS (Reality & TLS) and SOCKS5 generation. Protocols not active in the runtime Xray daemon are explicitly flagged as `NOT SUPPORTED` rather than generating invalid links.
- **Pure Client-Side QR Codes**: Rendered directly via HTML5 Canvas (zero third-party external API requests, preventing credential leakage).
- **Hardened Subscriptions**:
  - Tokens are generated via cryptographic randomness (64-char hex).
  - Database/memory stores strictly `sha256(token)`.
  - Raw token is presented strictly once upon creation.
  - Revoked tokens return HTTP 401 immediately.
  - HTTP access logs automatically redact token paths to `/sub/[REDACTED]`.
- **NOC Dark Theme**: Custom high-contrast dark theme optimized for technical operation centers.

---

## Security Architecture

| Security Domain | Implementation |
| :--- | :--- |
| **Authentication** | Password hashing via `bcrypt`, session IDs via `crypto/rand`, cookies with `HttpOnly; SameSite=Lax`. |
| **Role-Based Access** | `admin` (management & rotation) vs `readonly` (telemetry & inspection only). |
| **CSRF Defense** | Cryptographic CSRF token verification on all state-changing endpoints (`POST`, `DELETE`). |
| **Rate Limiting** | Token-bucket rate limiter via `golang.org/x/time/rate` (login: 5 req/min, subscription: 30 req/min). |
| **Zero Secret Leakage** | Automated validators assert zero exposure of private keys (`x25519`, `private_key`), OpenVPN certificates, or `.ovpn` secrets in ShareLinks or API responses. |
| **No Command Injection** | Manager backend contains zero calls to `os/exec` or shell execution. |
| **No XSS / Unsafe DOM** | React JSX rendering with zero occurrences of `dangerouslySetInnerHTML`. |

---

## Quickstart

### 1. Requirements

- Go 1.22+
- Node.js 18+ (only needed when rebuilding the frontend)

### 2. Build

```bash
# Optional: rebuild frontend assets (pre-built assets already embedded in backend/embedded/dist)
cd frontend
npm install
npm run build
cp -r dist/* ../backend/embedded/dist/
cd ..

# Build single standalone binary
go build -o bin/super-proxy-web ./backend/cmd/server
```

### 3. Run

```bash
# Environment variables
export MANAGER_PORT="8080"
export DAEMON_URL="https://127.0.0.1:60000"
export DAEMON_TOKEN="your-agent-api-token"
export ADMIN_PASSWORD="your-secure-admin-password"
export READONLY_PASSWORD="your-secure-readonly-password"

./bin/super-proxy-web
```

Access the panel at `http://127.0.0.1:8080`.

---

## Production Deployment

### Systemd

See [deploy/super-proxy-web.service](deploy/super-proxy-web.service):

```bash
sudo cp deploy/super-proxy-web.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable --now super-proxy-web
```

### HTTPS Reverse Proxy (Caddy)

See [deploy/Caddyfile.example](deploy/Caddyfile.example):

```caddy
manager.yourdomain.com {
    reverse_proxy 127.0.0.1:8080
}
```

---

## Testing

Run unit tests, property tests, and race detector:

```bash
go test -count=1 -race ./...
```

Frontend production build check:

```bash
cd frontend && npm run build
```

---

## License

Apache-2.0
