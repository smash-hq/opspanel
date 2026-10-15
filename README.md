# Server Plane ⚡

English | [简体中文](README.zh-CN.md)

**One binary to manage all your servers.**

[![Go](https://img.shields.io/badge/Go-1.26+-00ADD8?style=flat&logo=go)](https://golang.org)
[![License: Non-Commercial](https://img.shields.io/badge/license-Non--Commercial-orange.svg)](LICENSE)
[![Zero Dependency](https://img.shields.io/badge/dependency-zero%20external%20services-success?style=flat)](#why-server-plane)
[![Single Binary](https://img.shields.io/badge/delivery-single%20binary-8A2BE2?style=flat)](#why-server-plane)
[![PRs Welcome](https://img.shields.io/badge/PRs-welcome-ff69b4.svg)](CONTRIBUTING.md)

A native Go server operations panel. Single binary + embedded frontend + SQLite, **zero external dependencies, zero agents** — as long as your remote servers have SSH open, you get a full console right in the browser: real-time monitoring, software store, Docker, files, and processes, all in one screen. **All data stays local — no cloud, no telemetry.**

```bash
git clone https://github.com/smash-hq/server-management.git
cd server-management && make run
# Open http://localhost:18080 — that's it
```

## Why Server Plane

| | Server Plane | Traditional heavy panels |
|---|:---:|:---:|
| Installation | **Single binary, runs right after scp** | Install scripts + panel service + resident agent |
| External dependencies | **None (built-in SQLite, pure Go, no CGO)** | MySQL / PostgreSQL |
| Remote takeover | **SSH only, zero intrusion on managed hosts** | Install an agent on every machine |
| Resource usage | **One process, near-zero when idle** | Agent + panel, two processes |
| Domestic platforms | **arm64 out of the box (Kunpeng/Phytium)** | Depends on the distro |
| Tech stack | Native Go + native frontend, no build chain | Mixed Node/Python builds |

Ideal for: personal dev machines, small-team intranet servers, and lightweight ops on embedded/domestic-platform environments.

> ⚠️ **Intranet tool — do not expose to the public internet**: the panel ships with single-admin login, but its security mechanisms are thin (no HTTPS, no audit logs, no MFA), and SSH credentials are stored in a local SQLite database. See the [Security Disclaimer](#️-security-disclaimer).

## Features

| Module | Capabilities |
|--------|--------------|
| 🖥️ **Real-time monitoring** | CPU / memory / disk / network / uptime, pushed over WebSocket every 3 seconds, on-demand collection to minimize SSH overhead |
| 📦 **Software store** | One-click install/uninstall of MySQL, PostgreSQL, Redis, Nginx, Docker, Node.js; custom software packages and service control commands |
| 🌐 **Language environments** | Auto-detection of installed languages (Go/Java/Python/Node.js/Rust/PHP/Ruby/.NET), multi-version installs, custom extensions |
| 🐳 **Docker management** | Image pull/delete, container start/stop/terminal/logs, batch operations, registry switching (Docker Hub/Alibaba Cloud/Tencent Cloud/DaoCloud, etc.) |
| 📁 **File management** | Remote directory tree browsing, text preview, file download, sudo permission detection |
| ⚙️ **Process management** | Full process list (CPU/memory/command), SIGTERM/SIGKILL termination, process log viewer |
| 🎨 **UI experience** | Light/dark/system-follow themes, customizable tab icons, responsive desktop and mobile layouts |
| 🔐 **Login auth** | Single-admin login, bcrypt password hashing, cookie session with 7-day sliding renewal, online password change |

**Under the hood**: SSH connection pool (on-demand connections, idle recycling, auto-reconnect, 30s keepalive), SQLite WAL concurrent reads/writes, graceful shutdown, cross-platform cross-compilation.

**Roadmap**: cluster management · monitoring center (charts/alerts/log analysis) · security center (firewall/ports/IP banning) · scheduled tasks

## Quick Start

### Requirements

- Go 1.26+
- Linux / macOS / Windows
- SSH client (required for taking over remote servers)

### Run

```bash
# Foreground (recommended for development, Ctrl+C to stop)
make run

# Background
make start

# Dev mode (no compile step, direct go run — frontend changes take effect on refresh)
make dev

# Build manually and run
make build && ./smp-server
```

The server listens on `http://localhost:18080` by default — open it in a browser and you're set.

> 🔑 **Default account**: the first launch automatically creates the admin account **admin / admin123**. Please change the password in the panel as soon as possible. Check the version with `./smp-server --version`.

### Stop & Restart

```bash
make stop      # Stop
make status    # Check status
make restart   # Restart
```

## Deployment

### Single-machine deployment (Linux)

```bash
# 1. Build the static binary (CGO disabled, suitable for domestic platforms)
make build-linux
# Or build manually:
CGO_ENABLED=0 GOOS=linux go build -o smp-server ./cmd/server/

# 2. Create the app directory
sudo mkdir -p /opt/smp/data
sudo cp smp-server /opt/smp/
sudo chmod +x /opt/smp/smp-server

# 3. Start
cd /opt/smp && ./smp-server
```

### Docker

```bash
# Build the image
make docker
# Or:
docker build -t smash-hq/server-management:latest .

# Run the container
docker run -d \
  --name smp-server \
  -p 18080:18080 \
  -v /opt/smp/data:/app/data \
  --restart unless-stopped \
  smash-hq/server-management:latest
```

### Docker Compose

```yaml
services:
  smp:
    image: smash-hq/server-management:latest
    container_name: smp-server
    ports:
      - "18080:18080"
    volumes:
      - ./data:/app/data
    restart: unless-stopped
```

```bash
docker compose up -d
docker compose logs -f
docker compose down
```

### Reverse proxy (Nginx)

```nginx
server {
    listen 80;
    server_name smp.yourdomain.com;

    location / {
        proxy_pass http://127.0.0.1:18080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    }

    location /ws {
        proxy_pass http://127.0.0.1:18080/ws;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_read_timeout 86400;
    }
}
```

### Systemd service

```bash
# 1. Create a dedicated user
sudo useradd -r -s /bin/false smp

# 2. Deploy the binary
sudo cp smp-server /opt/smp/
sudo chown -R smp:smp /opt/smp

# 3. Create the service file
sudo tee /etc/systemd/system/smp-server.service > /dev/null << 'EOF'
[Unit]
Description=Server Plane Management Panel
After=network.target

[Service]
Type=simple
User=smp
WorkingDirectory=/opt/smp
ExecStart=/opt/smp/smp-server
Restart=on-failure
RestartSec=5

[Install]
WantedBy=multi-user.target
EOF

# 4. Start the service
sudo systemctl daemon-reload
sudo systemctl enable smp-server
sudo systemctl start smp-server
```

### Cross-compilation

```bash
make build-linux          # Linux amd64
make build-linux-arm64    # Linux arm64 (Kunpeng, Phytium, and other domestic platforms)
make build-darwin         # macOS
make build-windows        # Windows
```

## Data Storage

> 🔒 **Data never leaves your intranet**: all data (server configs, SSH credentials, monitoring snapshots, panel settings) is stored only in the local SQLite file under `data/`. The project has **no telemetry, data reporting, or cloud sync logic whatsoever** — the machine running it doesn't even need internet access.

Uses **SQLite** (`modernc.org/sqlite`, pure Go, no CGO required) as the local storage engine; the database file lives at `data/smp.db`.

**On first launch**, the database is initialized automatically with default data:

- **Local server** (`srv_local`): auto-detects hostname, OS/ARCH, and current user; cannot be deleted
- **Software catalog**: built-in definitions for common software
- **Docker registries**: Docker Hub / Alibaba Cloud / Tencent Cloud / DaoCloud / USTC / Tsinghua / NetEase / Huawei

All configuration changes are persisted to SQLite in real time (WAL mode, concurrent reads supported).

## Configuration

### Environment variables

| Variable | Default | Description |
|----------|---------|-------------|
| `SMP_SERVER_PORT` | 18080 | Listen port |
| `SMP_SERVER_HOST` | 0.0.0.0 | Listen address |
| `SMP_SERVER_PUSH_INTERVAL` | 3 | WebSocket push interval (seconds) |
| `SMP_SERVER_DATA_DIR` | ./data | Data directory |
| `SMP_LOG_LEVEL` | info | Log level |
| `SMP_LOG_FORMAT` | console | Log format |

### Configuration file

See [config.example.yaml](config.example.yaml) for a full example. Copy it to `config.yaml` (or `/etc/smp/config.yaml`) and adjust as needed:

```yaml
server:
  host: "0.0.0.0"
  port: 18080
  push_interval: 3
  shutdown_timeout: 10
  data_dir: "./data"

log:
  level: "info"
  format: "console"
```

Specify the config file path with `--config`:

```bash
./smp-server --config /etc/smp/config.yaml
```

## API

### Auth

| Method | Path | Description |
|--------|------|-------------|
| POST | `/api/v1/auth/login` | Log in; a session cookie is issued on success |
| POST | `/api/v1/auth/logout` | Log out and destroy the session |
| GET | `/api/v1/auth/me` | Current logged-in user |
| POST | `/api/v1/auth/change-password` | Change the admin password |

Except for the login endpoint and static assets, all APIs (including `/api/v1/health`) and WebSockets require a valid session cookie.

### Server management

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/servers` | List servers |
| POST | `/api/v1/servers` | Add a server |
| POST | `/api/v1/servers/test` | Test connection |
| PUT | `/api/v1/servers/{id}` | Update configuration |
| DELETE | `/api/v1/servers/{id}` | Delete a server |

### System information

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/system/info?server={id}` | System snapshot |
| GET | `/api/v1/health` | Health check |

### Software management

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/software/catalog` | Software catalog (from DB) |
| GET | `/api/v1/software/status?server={id}` | Install status |
| POST | `/api/v1/software/install` | Install software |
| POST | `/api/v1/software/uninstall` | Uninstall software |
| POST | `/api/v1/software/service` | Service start/stop |

### Software packages & sources

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/software/packages` | List software packages |
| POST | `/api/v1/software/packages` | Add a software package |
| PUT | `/api/v1/software/packages/{id}` | Update a software package |
| DELETE | `/api/v1/software/packages/{id}` | Delete a software package |
| GET/POST/PUT/DELETE | `/api/v1/software/sources[/id]` | Software source CRUD |

### File management

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/files?server=&path=` | List a directory |
| GET | `/api/v1/files/info?server=&path=` | File details |
| GET | `/api/v1/files/preview?server=&path=&lines=` | File preview |
| GET | `/api/v1/files/download?server=&path=` | Download a file |

### Docker management

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/docker?server=` | Docker environment detection |
| GET | `/api/v1/docker/images?server=` | List images |
| POST | `/api/v1/docker/images/pull` | Pull an image |
| DELETE | `/api/v1/docker/images/{id}` | Delete an image |
| POST | `/api/v1/docker/images/batch-delete` | Batch delete images |
| GET | `/api/v1/docker/containers?server=&all=` | List containers |
| POST | `/api/v1/docker/containers/{id}/{action}` | Container operation |
| POST | `/api/v1/docker/containers/batch` | Batch container operations |
| GET | `/api/v1/docker/containers/{id}/logs` | Container logs |
| POST | `/api/v1/docker/exec` | Execute a command inside a container |

### Docker registry management

| Method | Path | Description |
|--------|------|-------------|
| GET/POST | `/api/v1/docker-sources` | List registries / add one |
| PUT/DELETE | `/api/v1/docker-sources/{id}` | Update / delete a registry |

### Process management

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/processes?server=&lines=` | List processes |
| POST | `/api/v1/processes/kill` | Kill a process |
| GET | `/api/v1/processes/log?server=&pid=&lines=` | Process logs |

### Language environment management

| Method | Path | Description |
|--------|------|-------------|
| GET/POST | `/api/v1/languages` | List languages / add one |
| PUT/DELETE | `/api/v1/languages/{id}` | Update / delete a language |
| GET | `/api/v1/languages/detect?server=` | Detect installed languages |
| POST | `/api/v1/languages/install` | Install a language environment |

### WebSocket

| Path | Description |
|------|-------------|
| `/ws?server={id}` | Real-time system snapshot push |

## Project Structure

```
server-management/
├── cmd/server/          # Application entry point
├── internal/
│   ├── api/             # HTTP handlers
│   ├── config/          # Configuration loading (Viper)
│   ├── host/            # Server config abstraction & collection scheduling
│   ├── remote/          # SSH connection pool & remote execution (standalone package, avoids import cycles)
│   ├── storage/         # SQLite persistence (servers/software packages/sources/registries)
│   ├── middleware/      # Middleware (logging, recovery, security headers)
│   ├── server/          # HTTP service assembly
│   ├── software/        # Software package & language environment management
│   ├── system/          # System info collection (gopsutil)
│   ├── logger/          # Structured logging (zap)
│   └── ws/              # WebSocket push
├── web/                 # Frontend (embedded into the binary via go:embed)
│   ├── static.go        # embed.FS declaration
│   └── static/
│       ├── index.html   # Single-page entry
│       ├── css/
│       │   ├── base.css / layout.css / cards.css / states.css
│       │   └── components/  # Component styles (split by domain, loaded in order to preserve the cascade)
│       ├── js/              # Native JS (ES Modules, no build chain)
│       │   ├── app.js       # App assembly entry
│       │   ├── api/         # API clients (split by business domain)
│       │   ├── store/       # Reactive state (split by business domain, shared ctx pattern)
│       │   └── utils.js / ws.js
│       └── vendor/          # Local third-party libs (vue, xterm — zero external calls on intranets)
├── docs/                # Documentation
├── data/                # Runtime data (git ignored, smp.db auto-generated)
├── config.example.yaml  # Example configuration
├── Makefile             # Build & run management
├── Dockerfile           # Docker multi-stage build
├── CHANGELOG.md         # Changelog
├── SECURITY.md          # Security policy
├── .github/workflows/   # CI & notification workflows
└── go.mod               # Go module definition
```

## Contributing

PRs are welcome: the software catalog, language environment definitions, new features, and documentation are all great starting points.

See [CONTRIBUTING.md](CONTRIBUTING.md) (dev environment, code style, commit conventions, and the PR process).

## ⚠️ Security Disclaimer

**Please read the following security disclaimer carefully before using this software:**

1. **This software executes remote commands**: it connects to remote servers over SSH and runs system commands (including sudo operations), which may cause configuration changes, data loss, or service interruptions.
2. **Credential storage**: server connection info (passwords, private keys) is stored in a local SQLite database. Although protected by file permissions, it is **not suitable for production or high-security environments**.
3. **Thin authentication**: the panel ships with single-admin login (bcrypt + cookie sessions), but has no MFA, no audit logs, and no lockout on failures. If a session credential leaks, it is equivalent to full control — always pair it with intranet isolation and HTTPS.
4. **Liability disclaimer**: **the authors bear no responsibility for any security issues arising from the use of this software (including but not limited to server compromise, data leaks, service interruptions, or configuration corruption)**. Users assume all risks.
5. **Maintenance boundary**: the developer voluntarily maintains defects within normal functional scope; **this does not constitute a service commitment**. Issues arising from the usage environment, misoperation, secondary development, or data recovery are out of scope — see [Article 7 of the license](LICENSE).
6. **Recommendations**: use only on intranets/dev environments, never expose to the public internet, and always configure firewall and access control.

### Security checklist

- 🚫 Never expose the panel to the public internet — access via VPN or intranet only
- 🔒 Enable HTTPS via a reverse proxy (Nginx/Caddy)
- 🛡️ Configure the firewall to allow panel access only from trusted sources
- 🔑 Prefer SSH key authentication over weak passwords
- 👤 Run the panel under a dedicated user, and grant the remote user only a minimal NOPASSWD sudo command set
- 💾 Back up `data/smp.db` regularly; never commit it to version control
- 🔒 Keep the data directory at `0700` and the database file at `0600`

## License

[Non-Commercial Use License](LICENSE)

- ✅ **Free to use**: personal learning and internal ops within an organization (deployed on your own servers/intranet to manage your own workloads)
- 💼 **Commercial license required**: selling/reselling, integrating into paid products, providing SaaS or paid hosting based on this project, and other for-profit activities
- 📮 For commercial licensing, contact the project maintainers (contact info to be added)

This software is provided "as is", without warranty of any kind, express or implied. See the [disclaimer](LICENSE) for details.
