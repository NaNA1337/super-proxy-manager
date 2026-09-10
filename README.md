# super-proxy-manager

Modern, independent Multi-Host Web Manager & Control Panel for [super-proxy](https://github.com/NaNA1337/super-proxy).

Designed as a high-density, NOC-style control panel that communicates strictly through each `super-proxy` daemon's TLS Agent API without altering core daemon routing, FSM, or failover logic.

```text
Browser (HTTPS)
      │
      ▼
┌──────────────────────────────────────────────────────────┐
│  Web Manager (super-proxy-web)                           │
│  Port: :8080 (Localhost / Private Network or Behind Caddy)│
│  ├── React 18 + TypeScript + Vite + Tailwind (NOC Dark)  │
│  │   ├── Multi-Host Dropdown Selector (HOST: Tokyo-01)   │
│  │   ├── Hosts Fleet Management & 3-Step Wizard Modal    │
│  │   ├── Dashboard (3-second NOC status overview)        │
│  │   ├── Nodes Inventory & Transparent Score Factors     │
│  │   ├── Slot Controller (Safe daemon-authorized actions)│
│  │   ├── Policy Routing & Fail-closed Leak Protection    │
│  │   ├── Prometheus Metrics Real-time Visualizer         │
│  │   ├── System Events Stream (INFO / WARN / ERROR)      │
│  │   ├── Client Exporters (VLESS, Clash Meta, sing-box)  │
│  │   ├── Subscriptions Manager (Host-bound, SHA-256)     │
│  │   ├── Read-Only Settings & Architecture Inspector     │
│  │   └── Mutation Audit Trail                            │
│  └── Go Backend (Single Binary with Embedded Frontend)   │
│      ├── Pure Go SQLite persistence (WAL Mode)           │
│      ├── First-boot random bootstrap setup flow          │
│      ├── Forced initial administrator password change    │
│      ├── AES-256-GCM encrypted host credential storage   │
│      ├── SSRF guards against metadata/loopback exploits  │
│      ├── Subscriptions (SHA-256 store, [REDACTED] logs)  │
│      └── Multi-Host Client Pooling & Health Watcher      │
└─────────────────────────────┬────────────────────────────┘
                              │
                    TLS Mutual / Token Auth
                    https://<daemon-host>:60000
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

## Key Features

- **Multi-Host Management**: Manage multiple distributed super-proxy nodes from a single dashboard. Switch seamlessly via the top navbar (`HOST: Tokyo-01 ▼`).
- **Zero Hardcoded Passwords**: Absolutely no hardcoded or default passwords in source code or configuration files.
- **First-Time Setup Bootstrap**: Generates a high-entropy temporary password at first boot, displayed strictly once in terminal logs.
- **Forced Password Change**: Mandatory password change (minimum 12 characters) on first login before any control panel routes can be accessed.
- **Pure-Go SQLite Persistence**: Uses `modernc.org/sqlite` (CGO-free). Database schema auto-initializes in `data/super-proxy-manager.db`.
- **Encrypted Credential Storage**: Daemon API tokens are stored encrypted at rest using AES-256-GCM. Tokens are never exposed in API responses or logs.
- **SSRF Protection**: Strict URL validation blocking cloud metadata services (`169.254.169.254`, `metadata.google.internal`) and unauthorized protocols.
- **Client Link Exporters**: Exports live client configurations directly from active daemon runtime:
  - **VLESS Reality / TLS**: Standard URI with pure client-side QR Code.
  - **Clash Meta / Mihomo**: Ready-to-use YAML proxy block.
  - **sing-box**: Valid JSON outbound object.
  - **Xray-core**: Valid JSON outbound object.
  - **Copy All Client Links**: One-click structured text compilation of all active exit nodes.
- **3-Second NOC Dashboard**: Real-time status answering daemon liveness, active slots, egress country/IP/ASN, live traffic counters, and fail-closed leak protection.

---

## Security Architecture

| Security Domain | Implementation |
| :--- | :--- |
| **Authentication** | Password hashing via `bcrypt`, session IDs via `crypto/rand`, cookies with `HttpOnly; SameSite=Strict; Secure`. |
| **First-Boot Hardening**| Temporary bootstrap password invalidated immediately upon password change. All old sessions revoked. |
| **Token Encryption** | Host tokens encrypted at rest via AES-256-GCM (`MANAGER_MASTER_KEY` or auto-generated `data/.master.key`). |
| **SSRF Guards** | Strict scheme check (`http`/`https`), link-local metadata endpoints (`169.254.169.254`, `100.100.100.200`) blocked. |
| **Role-Based Access** | `admin` (management, host creation, rotation) vs `readonly` (inspection only). |
| **CSRF Defense** | Cryptographic CSRF token verification on all state-changing endpoints (`POST`, `PUT`, `DELETE`). |
| **Rate Limiting** | Token-bucket rate limiter via `golang.org/x/time/rate` (login: 5 req/min). |
| **Zero Secret Leakage** | Automated validators assert zero exposure of private keys, tokens, or OpenVPN credentials in ShareLinks or API responses. |
| **No Command Injection**| Zero calls to `os/exec` or shell execution anywhere in the codebase. |

---

## Quickstart

### 1. Requirements

- Linux / macOS / Windows
- Go 1.22+ (only if compiling from source)
- Node.js 18+ (only if modifying frontend)

### 2. Build

```bash
# Build standalone binary with embedded frontend assets
go build -o bin/super-proxy-web ./backend/cmd/server
```

### 3. Run

```bash
./bin/super-proxy-web
```

On first startup, the terminal will display the bootstrap setup banner:

```text
========================================================
SUPER-PROXY MANAGER FIRST-TIME SETUP
========================================================
Admin username: admin
Temporary password: <high-entropy-random-password>
IMPORTANT: This password will be invalidated after first password change.
Open: http://localhost:8443
========================================================
```

1. Open `http://localhost:8443` in your browser.
2. Log in with `admin` and the temporary password.
3. You will be prompted to set your permanent administrator password (minimum 12 characters).
4. Navigate to **Hosts** and click **+ Add Host** to connect your super-proxy daemon servers.

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
    reverse_proxy 127.0.0.1:8443
}
```

---

## Testing

Run unit tests, property tests, and the race detector:

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
