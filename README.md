# Server Plane - 服务器管理面板

[![Go](https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat&logo=go)](https://golang.org)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
[![Docker](https://img.shields.io/badge/Docker-supported-2496ED?style=flat&logo=docker)](https://www.docker.com)

Go 原生服务器运维平台，提供浏览器控制台管理本机及远程服务器。单机优先，零外部依赖（无需数据库），开箱即用。

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

- **服务器管理**：添加、连接、管理本机及远程 SSH 服务器
- **实时监控**：CPU、内存、磁盘、网络、运行时间等系统指标
- **软件商店**：一键安装/卸载 MySQL、PostgreSQL、Redis、Nginx、Docker、Node.js
- **WebSocket 推送**：实时数据自动刷新
- **国产平台适配**：麒麟 V10 等国产 Linux 自动回退采集方式

### 即将推出

- **集群管理**：多机负载均衡与服务编排
- **监控中心**：性能图表、告警通知、日志分析
- **文件管理**：Web 文件浏览、上传、编辑
- **Web 终端**：浏览器内 SSH 终端
- **防火墙**：端口管理、IP 封禁、规则模板

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

```bash
# 创建 docker-compose.yml（参见下方示例）
docker compose up -d

# 查看日志
docker compose logs -f

# 停止
docker compose down
```

`docker-compose.yml` 示例：

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
| GET | `/api/v1/software/catalog` | 软件目录 |
| GET | `/api/v1/software/status?server={id}` | 安装状态 |
| POST | `/api/v1/software/install` | 安装软件 |
| POST | `/api/v1/software/uninstall` | 卸载软件 |
| POST | `/api/v1/software/service` | 服务启停 |

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
│   ├── host/            # 服务器存储与 SSH 采集
│   ├── middleware/      # 中间件（日志、恢复）
│   ├── server/          # HTTP 服务组装
│   ├── software/        # 软件包管理
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
│   └── servers.example.json  # 服务器配置示例
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
- **SSH 连接**：golang.org/x/crypto/websocket
- **实时推送**：Gorilla WebSocket
- **日志**：Uber Zap（结构化日志）
- **前端**：原生 HTML/CSS/JS（无构建步骤，Go embed 嵌入）

## 安全建议

1. **不要将面板暴露在公网**，建议通过 VPN 或内网访问
2. **使用反向代理**（Nginx/Caddy）启用 HTTPS
3. **配置防火墙**，仅允许信任来源访问
4. **使用强密码**连接远程服务器
5. **定期备份** `data/servers.json` 配置文件
6. **为远程用户配置 NOPASSWD sudo**，避免 Web 进程拥有完整 root 权限
7. **不要提交 `data/servers.json`**，该文件包含敏感凭证信息

## 许可证

[MIT License](LICENSE)
