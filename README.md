# Server Plane ⚡

**一个二进制，管理你所有的服务器。**

[![Go](https://img.shields.io/badge/Go-1.26+-00ADD8?style=flat&logo=go)](https://golang.org)
[![License: Non-Commercial](https://img.shields.io/badge/license-Non--Commercial-orange.svg)](LICENSE)
[![Zero Dependency](https://img.shields.io/badge/依赖-零外部服务-success?style=flat)](#为什么选择-server-plane)
[![Single Binary](https://img.shields.io/badge/交付-单二进制-8A2BE2?style=flat)](#为什么选择-server-plane)
[![PRs Welcome](https://img.shields.io/badge/PRs-welcome-ff69b4.svg)](CONTRIBUTING.md)

Go 原生服务器运维面板。单二进制 + 内嵌前端 + SQLite，**零外部依赖、零 Agent**——远程服务器只要开着 SSH，浏览器里就是完整的控制台：实时监控、软件商店、Docker、文件、进程，一屏尽览。**所有数据仅存本地，不上云、无遥测。**

```bash
git clone https://github.com/smash-hq/server-management.git
cd server-management && make run
# 打开 http://localhost:18080 —— 就这么简单
```

## 为什么选择 Server Plane

| | Server Plane | 传统重型面板 |
|---|:---:|:---:|
| 安装交付 | **单二进制，scp 过去就能跑** | 安装脚本 + 面板服务 + Agent 常驻 |
| 外部依赖 | **无（SQLite 内置，纯 Go 免 CGO）** | MySQL / PostgreSQL |
| 远程接管 | **只需 SSH，被管机零侵入** | 每台机器装 Agent |
| 资源占用 | **一个进程，闲时几乎为零** | Agent + 面板双进程 |
| 国产平台 | **开箱支持 arm64（鲲鹏/飞腾）** | 视发行版而定 |
| 技术栈 | 原生 Go + 原生前端，无构建链 | Node/Python 混合构建 |

适合：个人开发机、小团队内网服务器、嵌入式/国产化环境的轻量运维。

> ⚠️ **内网工具，请勿暴露公网**：面板内置单管理员登录认证，但安全机制较薄（无 HTTPS、无审计日志、多因素缺失），SSH 凭证存储于本地 SQLite。详见[安全声明](#️-安全声明)。

## 功能总览

| 模块 | 能力 |
|------|------|
| 🖥️ **实时监控** | CPU / 内存 / 磁盘 / 网络 / 运行时间，WebSocket 3 秒级推送，按需采集降低 SSH 开销 |
| 📦 **软件商店** | MySQL、PostgreSQL、Redis、Nginx、Docker、Node.js 一键安装卸载，支持自定义软件包与服务控制命令 |
| 🌐 **语言环境** | 自动检测已安装语言（Go/Java/Python/Node.js/Rust/PHP/Ruby/.NET），多版本安装，支持自定义扩展 |
| 🐳 **Docker 管理** | 镜像拉取删除、容器启停/终端/日志、批量操作、镜像源切换（Docker Hub/阿里云/腾讯云/DaoCloud 等） |
| 📁 **文件管理** | 远程目录树形浏览、文本预览、文件下载、sudo 权限检测 |
| ⚙️ **进程管理** | 全量进程列表（CPU/内存/命令），SIGTERM/SIGKILL 终止，进程日志查看 |
| 🎨 **界面体验** | 白天/夜间/跟随系统三态主题，标签页图标自定义，桌面与移动端响应式 |
| 🔐 **登录认证** | 单管理员账户登录，bcrypt 密码哈希，Cookie 会话 7 天滑动续期，支持在线修改密码 |

**底层特性**：SSH 连接池（按需建连、空闲回收、断线自动重建、30s 保活）、SQLite WAL 并发读写、优雅启停、多平台交叉编译。

**路线图**：集群管理 · 监控中心（图表/告警/日志分析）· 安全中心（防火墙/端口/IP 封禁）· 定时任务

## 快速开始

### 环境要求

- Go 1.26+
- Linux / macOS / Windows
- SSH 客户端（接管远程服务器时需要）

### 启动

```bash
# 前台运行（推荐开发使用，Ctrl+C 停止）
make run

# 后台运行
make start

# 开发模式（无编译步骤，直接 go run，改前端刷新即生效）
make dev

# 手动编译后运行
make build && ./smp-server
```

服务默认监听 `http://localhost:18080`，浏览器访问即可使用。

> 🔑 **默认账户**：首次启动自动创建管理员账户 **admin / admin123**，请登录后尽快在面板中修改密码。查看版本：`./smp-server --version`。

### 停止与重启

```bash
make stop      # 停止
make status    # 查看状态
make restart   # 重启
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
```

### 跨平台编译

```bash
make build-linux          # Linux amd64
make build-linux-arm64    # Linux arm64（鲲鹏、飞腾等国产平台）
make build-darwin         # macOS
make build-windows        # Windows
```

## 数据存储

> 🔒 **数据不出内网**：所有数据（服务器配置、SSH 凭证、监控快照、面板设置）仅存储在本地 `data/` 目录的 SQLite 文件中。项目**无任何遥测、数据上报或云端同步逻辑**，部署机器完全不联网也能正常使用。

使用 **SQLite**（`modernc.org/sqlite`，纯 Go 实现，无需 CGO）作为本地存储引擎，数据库文件位于 `data/smp.db`。

**首次启动**时自动初始化数据库并写入默认数据：

- **本机服务器**（`srv_local`）：自动检测 hostname、OS/ARCH、当前用户，不可删除
- **软件包目录**：内置常用软件包定义
- **Docker 镜像源**：Docker Hub / 阿里云 / 腾讯云 / DaoCloud / 中科大 / 清华 / 网易 / 华为

所有配置变更实时持久化到 SQLite（WAL 模式，支持并发读取）。

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

完整示例见 [config.example.yaml](config.example.yaml)，将其复制为 `config.yaml`（或 `/etc/smp/config.yaml`）后按需修改：

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

通过 `--config` 指定配置文件路径：

```bash
./smp-server --config /etc/smp/config.yaml
```

## API 接口

### 认证

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/api/v1/auth/login` | 登录，成功后下发会话 Cookie |
| POST | `/api/v1/auth/logout` | 登出并销毁会话 |
| GET | `/api/v1/auth/me` | 当前登录用户信息 |
| POST | `/api/v1/auth/change-password` | 修改管理员密码 |

除登录接口与静态资源外，所有 API（含 `/api/v1/health`）与 WebSocket 均需携带有效会话 Cookie。

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
| GET/POST/PUT/DELETE | `/api/v1/software/sources[/id]` | 软件源 CRUD |

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
| GET/POST | `/api/v1/docker-sources` | 镜像源列表 / 新增 |
| PUT/DELETE | `/api/v1/docker-sources/{id}` | 更新 / 删除镜像源 |

### 进程管理

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/processes?server=&lines=` | 进程列表 |
| POST | `/api/v1/processes/kill` | 终止进程 |
| GET | `/api/v1/processes/log?server=&pid=&lines=` | 进程日志 |

### 语言环境管理

| 方法 | 路径 | 说明 |
|------|------|------|
| GET/POST | `/api/v1/languages` | 语言列表 / 新增 |
| PUT/DELETE | `/api/v1/languages/{id}` | 更新 / 删除语言 |
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
│   ├── remote/          # SSH 连接池与远程执行（独立包，避免循环依赖）
│   ├── storage/         # SQLite 持久化（服务器/软件包/软件源/镜像源）
│   ├── middleware/      # 中间件（日志、恢复、安全头）
│   ├── server/          # HTTP 服务组装
│   ├── software/        # 软件包与语言环境管理
│   ├── system/          # 系统信息采集（gopsutil）
│   ├── logger/          # 结构化日志（zap）
│   └── ws/              # WebSocket 推送
├── web/                 # 前端（go:embed 嵌入二进制）
│   ├── static.go        # embed.FS 声明
│   └── static/
│       ├── index.html   # 单页入口
│       ├── css/
│       │   ├── base.css / layout.css / cards.css / states.css
│       │   └── components/  # 组件样式（按域拆分，按序加载保持级联）
│       ├── js/              # 原生 JS（ES Module，无构建链）
│       │   ├── app.js       # 应用装配入口
│       │   ├── api/         # API 客户端（按业务域拆分）
│       │   ├── store/       # 响应式状态（按业务域拆分，共享 ctx 模式）
│       │   └── utils.js / ws.js
│       └── vendor/          # 本地第三方库（vue、xterm，内网零外联）
├── docs/                # 文档
├── data/                # 运行时数据（git ignored，smp.db 自动生成）
├── config.example.yaml  # 配置文件示例
├── Makefile             # 构建与运行管理
├── Dockerfile           # Docker 多阶段构建
├── CHANGELOG.md         # 变更日志
├── SECURITY.md          # 安全策略
├── .github/workflows/   # CI 与通知工作流
└── go.mod               # Go 模块定义
```

## 参与贡献

欢迎提交 PR：软件包目录、语言环境定义、新功能与文档完善都是绝佳的切入点。

详见 [CONTRIBUTING.md](CONTRIBUTING.md)（开发环境、代码规范、提交规范与 PR 流程）。

## ⚠️ 安全声明

**使用本软件前，请仔细阅读以下安全声明：**

1. **本软件会执行远程命令**：通过 SSH 连接远程服务器并执行系统命令（包括 sudo 操作），可能导致系统配置变更、数据丢失或服务中断。
2. **凭证存储**：服务器连接信息（密码、私钥）存储在本地 SQLite 数据库中，虽有文件权限保护，但**不应用于生产环境或高安全要求场景**。
3. **认证机制较薄**：面板内置单管理员登录认证（bcrypt + Cookie 会话），但无多因素、无审计日志、无失败锁定；会话凭证一旦泄露即等同完全控制权限，请务必配合内网隔离与 HTTPS 使用。
4. **责任免除**：**一切因使用本软件引起的安全问题（包括但不限于服务器被入侵、数据泄露、服务中断、配置损坏等）均与本项目无关**。使用者需自行承担所有风险。
5. **维护边界**：开发者仅对正常功能范围内的缺陷提供自愿维护，**不构成服务承诺**；使用环境、误操作、二次开发引入的问题及数据恢复等诉求不在维护范围内，详见[协议第七条](LICENSE)。
6. **使用建议**：仅在内网/开发环境使用，不要暴露到公网，务必配置防火墙和访问控制。

### 安全清单

- 🚫 不要将面板暴露在公网 — 仅通过 VPN 或内网访问
- 🔒 使用反向代理（Nginx/Caddy）启用 HTTPS
- 🛡️ 配置防火墙，仅允许信任来源访问面板端口
- 🔑 推荐使用 SSH 密钥认证，避免弱密码
- 👤 创建专用用户运行面板服务，远程用户仅授予 NOPASSWD sudo 的最小命令集
- 💾 定期备份 `data/smp.db`，不要将其提交到版本控制
- 🔒 确保数据目录权限 `0700`、数据库文件权限 `0600`

## 许可证

[非商业使用许可协议](LICENSE)

- ✅ **免费使用**：个人学习、组织内部运维（部署在自有服务器/内网管理自己的业务）
- 💼 **需要商业授权**：销售/转售、集成到对外收费的产品、基于本项目提供 SaaS 或付费托管等直接营利行为
- 📮 商业授权请联系项目维护者（联系方式待补充）

本软件按「原样」提供，不附带任何明示或暗示的保证，详见[免责声明](LICENSE)。
