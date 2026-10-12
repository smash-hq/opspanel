# Server Plane - 服务器管理面板

[![Go](https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat&logo=go)](https://golang.org)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
[![Docker](https://img.shields.io/badge/Docker-supported-2496ED?style=flat&logo=docker)](https://www.docker.com)

Go 原生服务器运维平台，提供浏览器控制台管理本机及远程服务器。单机优先，零外部依赖（无需数据库），开箱即用。

## ⚠️ 安全声明

**使用本软件前，请仔细阅读以下安全声明：**

1. **本软件会执行远程命令**：通过 SSH 连接远程服务器并执行系统命令（包括 sudo 操作），可能导致系统配置变更、数据丢失或服务中断。
2. **凭证存储**：服务器连接信息（密码、私钥）存储在本地 SQLite 数据库中，虽有文件权限保护，但**不应用于生产环境或高安全要求场景**。
3. **无认证机制**：当前版本**无内置用户认证和访问控制**，任何能访问面板的人都可以执行所有操作。
4. **责任免除**：**一切因使用本软件引起的安全问题（包括但不限于服务器被入侵、数据泄露、服务中断、配置损坏等）均与本项目无关**。使用者需自行承担所有风险。
5. **使用建议**：仅在内网/开发环境使用，不要暴露到公网，务必配置防火墙和访问控制。

---

## 目录

- [功能特性](#功能特性)
- [快速开始](#快速开始)
- [部署指南](#部署指南)
- [配置](#配置)
- [API 接口](#api-接口)
- [项目结构](#项目结构)
- [开发指南](#开发指南)
- [安全建议](#安全建议)
- [许可证](#许可证)

## 功能特性

### 当前版本 (V0.1)

#### 🖥️ 服务器管理
- **节点管理**：添加、连接、管理本机及远程 SSH 服务器
- **实时监控**：CPU、内存、磁盘、网络、运行时间等系统指标
- **WebSocket 推送**：实时数据自动刷新（仅在系统信息页采集，减少 SSH 开销）

#### 📦 软件管理
- **软件商店**：一键安装/卸载 MySQL、PostgreSQL、Redis、Nginx、Docker、Node.js
- **软件包管理**：自定义软件包定义（安装命令、服务控制命令）
- **软件源管理**：管理镜像源/仓库地址，支持预设和自定义源

#### 📁 文件管理
- **文件浏览**：远程服务器文件目录浏览（树形导航）
- **文件操作**：预览文本文件、下载文件、查看文件详情
- **权限检测**：自动检测 sudo 权限并提示

#### 🐳 Docker 管理
- **镜像管理**：查看、拉取、删除镜像，批量操作支持
- **容器管理**：启动、停止、重启、删除容器，批量操作支持
- **容器终端**：在容器内执行命令
- **容器日志**：实时查看容器日志
- **镜像源管理**：常用镜像源预设 + 自定义源

#### ⚙️ 进程管理
- **进程列表**：查看所有运行进程（PID、用户、CPU、内存、命令）
- **进程控制**：终止进程（SIGTERM/SIGKILL）
- **进程日志**：查看进程相关日志

#### 🌐 运行环境
- **编程语言管理**：检测服务器已安装的语言环境（Go/Java/Python/Node.js/Rust/PHP/Ruby/.NET 等）
- **一键安装**：从软件商店安装语言环境
- **自定义扩展**：支持添加自定义语言和安装脚本

#### 🎨 界面
- **主题切换**：白天/夜间/跟随系统
- **自定义图标**：浏览器标签页图标自定义
- **响应式布局**：适配桌面和移动设备

### 即将推出

- **集群管理**：多机负载均衡与服务编排
- **监控中心**：性能图表、告警通知、日志分析
- **安全中心**：防火墙规则、端口管理、IP 封禁
- **定时任务**：Cron 定时任务管理

## 快速开始

### 环境要求

- Go 1.21+
- Linux / macOS / Windows
- SSH 客户端（连接远程服务器时需要）

### 安装

```bash
# 克隆仓库
git clone https://github.com/smash-hq/server-management.git
cd server-management

# 下载依赖
go mod download
```

### 启动服务

```bash
# 方式一：前台运行（推荐开发使用，Ctrl+C 停止）
make run

# 方式二：后台运行
make start

# 方式三：开发模式（无编译步骤，直接 go run）
make dev

# 方式四：手动编译后运行
make build
./smp-server
```

服务默认监听 `http://localhost:18080`，浏览器访问即可使用。

### 停止与重启

```bash
# 停止服务
make stop

# 查看运行状态
make status

# 重启服务
make restart
```

## 部署指南

### 单机部署（Linux）

```bash
# 1. 编译静态二进制（CGO 禁用，适合国产平台）
make build-linux
# 或手动编译：
CGO_ENABLED=0 GOOS=linux go build -o smp-server ./cmd/server/

# 2. 创建应用目录
sudo mkdir -p /opt/smp/data
sudo cp smp-server /opt/smp/
sudo chmod +x /opt/smp/smp-server

# 3. 启动
cd /opt/smp && ./smp-server
```

### Docker 部署

```bash
# 构建镜像
make docker
# 或：
docker build -t smash-hq/server-management:latest .

# 运行容器
docker run -d \
  --name smp-server \
  -p 18080:18080 \
  -v /opt/smp/data:/app/data \
  --restart unless-stopped \
  smash-hq/server-management:latest
```

### Docker Compose

```yaml
version: "3.8"
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

### 反向代理（Nginx）

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

### 系统服务（systemd）

```bash
# 1. 创建专用用户
sudo useradd -r -s /bin/false smp

# 2. 部署二进制
sudo cp smp-server /opt/smp/
sudo chown -R smp:smp /opt/smp

# 3. 创建服务文件
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

# 4. 启动服务
sudo systemctl daemon-reload
sudo systemctl enable smp-server
sudo systemctl start smp-server

# 5. 查看状态
sudo systemctl status smp-server
```

### 跨平台编译

```bash
# Linux amd64
make build-linux

# Linux arm64（国产平台如鲲鹏、飞腾）
make build-linux-arm64

# macOS
make build-darwin

# Windows
make build-windows
```

## 数据存储

使用 **SQLite**（`modernc.org/sqlite`，纯 Go 实现，无需 CGO）作为本地存储引擎，数据库文件位于 `data/smp.db`。

**首次启动**时自动初始化数据库并写入默认数据：
- **本机服务器**（`srv_local`）：自动检测 hostname、OS/ARCH、当前用户
- **软件包目录**：内置常用软件包定义
- **Docker 镜像源**：Docker Hub/阿里云/腾讯云/DaoCloud/中科大/清华/网易/华为

- **本机条目**（`srv_local`）不可删除，保证面板始终有一个可监控的目标
- 所有配置变更实时持久化到 SQLite（WAL 模式，支持并发读取）
- 预留扩展：集群、监控历史、文件索引等数据均可存入同一数据库

## 配置

### 环境变量

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `SMP_SERVER_PORT` | 18080 | 监听端口 |
| `SMP_SERVER_HOST` | 0.0.0.0 | 监听地址 |
| `SMP_SERVER_PUSH_INTERVAL` | 3 | WebSocket 推送间隔（秒） |
| `SMP_SERVER_DATA_DIR` | ./data | 数据存储目录 |
| `SMP_LOG_LEVEL` | info | 日志级别 |
| `SMP_LOG_FORMAT` | console | 日志格式 |

### 配置文件

```yaml
# config.yaml
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

通过 `--config` 指定配置文件路径：

```bash
./smp-server --config /etc/smp/config.yaml
```

## API 接口

### 服务器管理

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/servers` | 服务器列表 |
| POST | `/api/v1/servers` | 添加服务器 |
| POST | `/api/v1/servers/test` | 连接测试 |
| PUT | `/api/v1/servers/{id}` | 更新配置 |
| DELETE | `/api/v1/servers/{id}` | 删除服务器 |

### 系统信息

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/system/info?server={id}` | 系统快照 |
| GET | `/api/v1/health` | 健康检查 |

### 软件管理

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/software/catalog` | 软件目录（数据库） |
| GET | `/api/v1/software/status?server={id}` | 安装状态 |
| POST | `/api/v1/software/install` | 安装软件 |
| POST | `/api/v1/software/uninstall` | 卸载软件 |
| POST | `/api/v1/software/service` | 服务启停 |

### 软件包与源管理

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/software/packages` | 软件包列表 |
| POST | `/api/v1/software/packages` | 新增软件包 |
| PUT | `/api/v1/software/packages/{id}` | 更新软件包 |
| DELETE | `/api/v1/software/packages/{id}` | 删除软件包 |
| GET | `/api/v1/software/sources` | 软件源列表 |
| POST/PUT/DELETE | `/api/v1/software/sources[/id]` | 软件源 CRUD |

### 文件管理

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/files?server=&path=` | 列出目录 |
| GET | `/api/v1/files/info?server=&path=` | 文件详情 |
| GET | `/api/v1/files/preview?server=&path=&lines=` | 文件预览 |
| GET | `/api/v1/files/download?server=&path=` | 下载文件 |

### Docker 管理

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/docker?server=` | Docker 环境检测 |
| GET | `/api/v1/docker/images?server=` | 镜像列表 |
| POST | `/api/v1/docker/images/pull` | 拉取镜像 |
| DELETE | `/api/v1/docker/images/{id}` | 删除镜像 |
| POST | `/api/v1/docker/images/batch-delete` | 批量删除镜像 |
| GET | `/api/v1/docker/containers?server=&all=` | 容器列表 |
| POST | `/api/v1/docker/containers/{id}/{action}` | 容器操作 |
| POST | `/api/v1/docker/containers/batch` | 批量容器操作 |
| GET | `/api/v1/docker/containers/{id}/logs` | 容器日志 |
| POST | `/api/v1/docker/exec` | 容器内执行命令 |

### Docker 镜像源管理

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/docker-sources` | 镜像源列表 |
| POST | `/api/v1/docker-sources` | 新增镜像源 |
| PUT | `/api/v1/docker-sources/{id}` | 更新镜像源 |
| DELETE | `/api/v1/docker-sources/{id}` | 删除镜像源 |
| POST | `/api/v1/docker-sources/{id}` | 设为默认源 |

### 进程管理

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/processes?server=&lines=` | 进程列表 |
| POST | `/api/v1/processes/kill` | 终止进程 |
| GET | `/api/v1/processes/log?server=&pid=&lines=` | 进程日志 |

### 语言环境管理

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/languages` | 语言列表 |
| POST | `/api/v1/languages` | 新增语言 |
| PUT | `/api/v1/languages/{id}` | 更新语言 |
| DELETE | `/api/v1/languages/{id}` | 删除语言 |
| GET | `/api/v1/languages/detect?server=` | 检测已安装语言 |
| POST | `/api/v1/languages/install` | 安装语言环境 |

### WebSocket

| 路径 | 说明 |
|------|------|
| `/ws?server={id}` | 实时推送系统快照 |

## 项目结构

```
server-management/
├── cmd/server/          # 应用入口
├── internal/
│   ├── api/             # HTTP 处理器
│   ├── config/          # 配置加载（Viper）
│   ├── host/            # 服务器配置抽象与采集调度
│   ├── remote/          # SSH 远程执行（独立包，避免循环依赖）
│   ├── storage/         # SQLite 持久化（服务器/软件包/软件源/镜像源）
│   ├── middleware/      # 中间件（日志、恢复）
│   ├── server/          # HTTP 服务组装
│   ├── software/        # 软件包管理与安装
│   ├── system/          # 系统信息采集（gopsutil）
│   ├── logger/          # 结构化日志（zap）
│   └── ws/              # WebSocket 推送
├── web/                 # 前端
│   ├── static/          # 静态资源（Go embed）
│   │   ├── css/         # 样式表
│   │   ├── js/          # 脚本模块
│   │   └── index.html   # 入口页面
│   └── README.md        # 前端文档
├── docs/                # 文档
├── data/                # 运行时数据（git ignored）
│   ├── .gitkeep
│   └── smp.db           # SQLite 数据库（自动生成）
├── Makefile             # 构建与运行管理
├── Dockerfile           # Docker 多阶段构建
├── go.mod               # Go 模块定义
└── README.md            # 本文件
```

## 开发指南

### Makefile 目标一览

| 目标 | 说明 |
|------|------|
| `make run` | 编译并前台启动服务 |
| `make start` | 编译并后台启动服务 |
| `make stop` | 停止后台服务 |
| `make restart` | 重启服务 |
| `make status` | 查看运行状态 |
| `make dev` | 开发模式（直接 go run） |
| `make build` | 编译当前平台二进制 |
| `make build-linux` | 编译 Linux amd64 二进制 |
| `make build-linux-arm64` | 编译 Linux arm64 二进制（鲲鹏/飞腾） |
| `make build-darwin` | 编译 macOS 二进制 |
| `make build-windows` | 编译 Windows 二进制 |
| `make test` | 运行测试 |
| `make fmt` | 格式化代码 |
| `make vet` | 静态检查 |
| `make deps` | 下载并整理依赖 |
| `make docker` | 构建 Docker 镜像 |
| `make all` | 格式化 + 检查 + 编译 |
| `make clean` | 清理构建产物 |
| `make help` | 显示帮助信息 |

### 常用工作流

```bash
# 开发循环
make dev

# 提交前检查
make all

# 运行测试
make test

# 下载依赖
make deps
```

### 技术栈

- **Web 框架**：Go 标准库 `net/http`
- **配置管理**：Viper（支持配置文件、环境变量、命令行标志）
- **系统采集**：gopsutil（CPU、内存、磁盘、网络）
- **SSH 连接**：golang.org/x/crypto/ssh
- **实时推送**：Gorilla WebSocket
- **日志**：Uber Zap（结构化日志）
- **前端**：原生 HTML/CSS/JS（无构建步骤，Go embed 嵌入）

## 安全建议

> **⚠️ 再次强调：使用本软件可能带来严重安全风险，请务必遵守以下建议。**

### 必须遵守

1. **🚫 不要将面板暴露在公网** — 建议仅通过 VPN 或内网访问
2. **🔒 使用反向代理**（Nginx/Caddy）启用 HTTPS
3. **🛡️ 配置防火墙**，仅允许信任来源访问面板端口
4. **🔑 使用强密码**连接远程服务器，推荐使用 SSH 密钥认证
5. **👤 创建专用用户**运行面板服务，不要使用 root 用户

### 远程服务器安全

6. **🔐 为远程用户配置 NOPASSWD sudo** — 避免 Web 进程拥有完整 root 权限
7. **📋 定期审计** `data/smp.db` 中的服务器配置和访问日志
8. **🗑️ 定期清理**不再使用的服务器配置和敏感凭证

### 数据安全

9. **💾 定期备份** `data/smp.db` 数据库文件
10. **🚫 不要提交 `data/smp.db`** 到版本控制，该文件包含敏感凭证信息
11. **🔒 文件权限**确保 `data/` 目录权限为 `0700`，数据库文件权限为 `0600`

### 责任声明

**本软件按"原样"提供，不附带任何明示或暗示的保证。一切因使用本软件引起的安全问题（包括但不限于服务器被入侵、数据泄露、服务中断、配置损坏、数据丢失等）均与本项目无关。使用者需自行评估风险并承担所有责任。**

## 许可证

[MIT License](LICENSE)
