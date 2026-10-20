# OpsPanel

English | [简体中文](README.zh-CN.md)

**A lightweight server ops panel in a single binary — zero agents, zero dependencies.**

[![Go](https://img.shields.io/badge/Go-1.26+-00ADD8?style=flat&logo=go)](https://golang.org)
[![License: Non-Commercial](https://img.shields.io/badge/license-Non--Commercial-orange.svg)](LICENSE)
[![Single Binary](https://img.shields.io/badge/delivery-single%20binary-8A2BE2?style=flat)](#deployment)

## What is it

OpsPanel is a server operations panel written in native Go: **single binary + embedded frontend + SQLite, zero external dependencies, zero agents**. As long as your servers have SSH open, you get a full console right in the browser. All data stays local — no cloud, no telemetry.

```bash
git clone https://github.com/smash-hq/opspanel.git
cd opspanel && make run
# Open http://localhost:18080 (default account admin / admin123 — change it soon)
```

## Screenshots

**Login** — bilingual UI, switchable on the login page and in the top bar

![Login](docs/images/en/login.png)

**System dashboard** — CPU / memory / disk / uptime at a glance

![System](docs/images/en/system-info.png)

**systemd services** — search all units, start / stop / restart, edit unit files, view logs

![Services](docs/images/en/services.png)

**Language environments** — auto-detection with one-click install / uninstall

![Languages](docs/images/en/language.png)

**Software packages** — one-click install / uninstall from the store, with service control

![Packages](docs/images/en/config-software.png)

**Docker** — image and container management, container terminal

![Docker](docs/images/en/docker-container.png)

**App deployment** — upload jars / binaries / scripts, remote start / stop / restart, live logs

![Apps](docs/images/en/apps.png)

**Web terminal** — interactive SSH shell with a built-in common-commands palette (incl. GPU / NPU groups)

![Terminal](docs/images/en/terminal.png)

**GPU / NPU** — Ascend / Cambricon device info, pulled on demand via SSH

![GPU](docs/images/en/gpu.png)

**Software repositories** — visual management for apt / yum / apk sources

![Repositories](docs/images/en/config-repo.png)

## What it does

| Module | Capabilities |
|--------|--------------|
| 🖥️ Real-time monitoring | CPU / memory / disk / network, WebSocket push every 3s |
| 📦 Software store | One-click install/uninstall (MySQL / Redis / Nginx etc.), service control, custom packages and sources |
| 🌐 Language environments | Auto-detection and multi-version management for 8 languages (Go / Java / Python etc.), live install / uninstall logs |
| 🐳 Docker | Image / container management, container terminal and logs, batch operations, registry mirror switching |
| 🧭 systemd services | Search all units, start / stop / restart, edit unit files, view logs, boot state at a glance |
| 📁 File management | Directory browsing, text preview, download |
| ⚙️ Process management | Process list, kill, logs |
| 🚀 App deployment | Chunked-upload deploy, remote start/stop/restart, log viewer |
| ⏰ Scheduled tasks | Visual crontab, expression validation with next-run preview |
| 🔔 Notification center | In-app + SMTP email, automatic app-failure alerts |
| 🎮 GPU/NPU | Ascend / Cambricon device info (power / temp / utilization / memory) |
| ⌨️ Web terminal | Interactive SSH terminal with a searchable common-commands palette (incl. GPU / NPU groups) |
| 🌍 Bilingual UI | 中文 / English interface, switchable on the login page and in the top bar |
| 🛡️ Safe operations | Destructive actions (delete / uninstall / service control) require a second confirmation, enforced server-side |
| 🔐 Login auth | Single admin, bcrypt, cookie sessions |

Ideal for lightweight ops on personal dev machines and small-team intranet servers; Linux arm64 works out of the box (Kunpeng, Phytium, and other domestic platforms).

## Deployment

### Run from source

```bash
make run                    # Foreground (Ctrl+C to stop)
make dev                    # Dev mode — frontend changes apply on refresh
make build && ./opspanel    # Build manually, then run
```

### Linux binary

```bash
make build-linux          # linux/amd64
make build-linux-arm64    # linux/arm64 (Kunpeng, Phytium)

sudo mkdir -p /opt/opspanel/data
sudo cp opspanel /opt/opspanel/ && sudo chmod +x /opt/opspanel/opspanel
cd /opt/opspanel && ./opspanel
```

### Configuration

By default the server listens on `0.0.0.0:18080` and stores data in `./data`. Adjust via environment variables (`OPSPANEL_SERVER_PORT`, `OPSPANEL_SERVER_HOST`, `OPSPANEL_SERVER_DATA_DIR`, `OPSPANEL_LOG_LEVEL`, etc.) or a config file — full example in [config.example.yaml](config.example.yaml), custom path via `./opspanel --config /etc/opspanel/config.yaml`.

> Behind a reverse proxy, the `/ws` path must support WebSocket upgrade. Handy commands: `make stop` / `make restart` / `make status`.

## Risks

**Read [SECURITY.md](SECURITY.md) and the [LICENSE](LICENSE) before use:**

1. **Intranet / trusted environments only — never expose to the public internet.** The panel has single-admin login only: no HTTPS, no MFA, no audit logs, no lockout on failed logins.
2. **SSH credentials are stored in a local SQLite file** (protected by file permissions) — not suitable for production or high-security environments.
3. **This software executes commands on remote servers (including sudo)**; misoperation may cause configuration changes, data loss, or service interruptions. Use at your own risk.
4. The authors bear no responsibility for security issues arising from use — see Article 7 of the license.

**Recommendations**: enable HTTPS via reverse proxy · restrict access sources with a firewall · prefer SSH key authentication · grant the remote user only a minimal NOPASSWD command set · back up `data/opspanel.db` regularly.

## Contributing

Contributions are welcome! Issues and PRs alike: the software catalog, language environment definitions, new features, and docs are all great starting points — see [CONTRIBUTING.md](CONTRIBUTING.md).

## License

[Non-Commercial Use License](LICENSE) — free for personal learning and internal ops within an organization; commercial use requires a license.
