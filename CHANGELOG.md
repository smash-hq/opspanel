# Changelog

本项目的所有显著变更都将记录在本文件中。

格式基于 [Keep a Changelog](https://keepachangelog.com/zh-CN/1.1.0/)，
版本号遵循[语义化版本](https://semver.org/lang/zh-CN/)。

## [Unreleased]

### Added

- **服务器管理**：SSH 接管远程服务器（密码 / 私钥），首启自动注册本机节点（`srv_local`，不可删除），连接测试、增删改查
- **实时监控**：CPU / 内存 / 磁盘 / 网络 / 运行时间采集，WebSocket 按需推送（默认 3 秒间隔）
- **软件商店**：MySQL、PostgreSQL、Redis、Nginx、Docker、Node.js 等一键安装卸载，服务启停控制，支持自定义软件包与软件源
- **语言环境**：自动检测已安装语言（Go / Java / Python / Node.js / Rust / PHP / Ruby / .NET），多版本安装与卸载，支持自定义扩展
- **Docker 管理**：镜像拉取 / 删除 / 批量删除，容器启停 / 交互式终端 / 日志 / 批量操作，镜像源切换（Docker Hub / 阿里云 / 腾讯云 / DaoCloud 等）
- **文件管理**：远程目录树形浏览、文本预览、文件下载、sudo 权限检测
- **进程管理**：全量进程列表，SIGTERM / SIGKILL 终止，进程日志查看
- **Web 终端**：基于 xterm.js 的浏览器交互式 SSH 终端
- **登录认证**：单管理员账户（默认 `admin` / `admin123`，bcrypt 哈希），Cookie 会话 7 天滑动续期，在线修改密码，支持自定义 favicon
- **存储层**：SQLite（`modernc.org/sqlite`，纯 Go 免 CGO）持久化，WAL 模式支持并发读取
- **SSH 连接池**：按需建连、空闲回收、断线自动重建、30s 保活
- **工程化**：Makefile 一键构建 / 运行，多平台交叉编译（含 Linux arm64），Docker 多阶段构建，GitHub Actions CI 与飞书通知工作流

### Changed

- 前端代码骨架按业务域模块化拆分：JS 拆分为 `js/api/`（12 个接口域文件）与 `js/store/`（16 个状态域模块，共享 ctx 模式），CSS 拆分为 `css/components/` 下 27 个按域样式文件，保持原生 ES Module 无构建链直载
- 前端第三方库（Vue、xterm.js）本地 vendor 化，移除外网 CDN 依赖，内网环境零外联可用
- 服务启停统一由 Makefile 管理，移除 start.sh / stop.sh
- 管理模式重构为「以节点为中心」
- 版本号与 Git 提交哈希构建时通过 `-ldflags` 注入，支持 `--version` 查看

### Fixed

- SSH 采集改为每命令独立 Session、底层连接复用，修复长时间运行后采集失败
- 修复 Docker 操作链路失效与并发阻塞、环境检测误判
- 修复文件下载与首页查询问题
- 修复删除软件包无效（误调不存在的接口函数），及其保存自定义软件包时误重载目录数据
- 修复软件源删除确认弹窗按钮无响应
- 修复 Docker 镜像源删除与「设为默认」操作无效（误调接口函数）
- 修复进程终止按钮无响应
- 修复语言环境页切换软件源事件失效

[Unreleased]: https://github.com/smash-hq/server-management/compare/v0.1.0...HEAD
