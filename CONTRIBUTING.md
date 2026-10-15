# 贡献指南

感谢你关注 OpsPanel！欢迎通过 Issue 和 Pull Request 参与共建。

## 目录

- [环境要求](#环境要求)
- [快速上手](#快速上手)
- [开发工作流](#开发工作流)
- [代码规范](#代码规范)
- [提交规范](#提交规范)
- [Pull Request 流程](#pull-request-流程)
- [贡献软件包与语言定义](#贡献软件包与语言定义)
- [贡献与许可](#贡献与许可)
- [安全问题报告](#安全问题报告)

## 环境要求

- **Go 1.26+**
- macOS / Linux / Windows（跨平台开发均可）
- 无需 Node.js：前端为原生 HTML/CSS/JS，`go:embed` 直接嵌入，没有构建步骤

## 快速上手

```bash
# 1. Fork 并克隆
git clone https://github.com/<你的用户名>/opspanel.git
cd opspanel

# 2. 下载依赖
go mod download

# 3. 启动开发模式（改代码后 Ctrl+C 重启即生效）
make dev
```

浏览器打开 <http://localhost:18080>，前端静态文件修改后刷新页面即可。

## 开发工作流

```bash
make dev      # 开发模式：直接 go run，无需编译
make all      # 提交前必跑：gofmt + go vet + go build
make test     # 运行测试
make clean    # 清理构建产物
```

**提交 PR 前请确保 `make all` 通过。**

## 代码规范

- 保持 Go 官方风格：提交前执行 `make fmt`（gofmt）
- 通过 `make vet` 静态检查
- 注释使用中文，解释「为什么」而非「做了什么」
- 新增依赖需说明理由，优先使用标准库与现有依赖
- 前端改动保持原生三件套（HTML/CSS/JS），不引入构建工具与框架

### 项目结构速览

```
cmd/server/       入口
internal/
  api/            HTTP 处理器
  config/         配置加载（Viper）
  host/           采集调度（本机 gopsutil / 远程 SSH）
  remote/         SSH 连接池与远程执行
  software/       软件包与语言环境安装
  storage/        SQLite 持久化
  server/         HTTP 服务组装
  ws/             WebSocket 推送
web/static/       前端（go:embed 嵌入，无构建链 ES Module）
  js/api/         API 客户端（按业务域拆分：auth / docker / files / software…）
  js/store/       响应式状态（按业务域拆分，共享 ctx 模式，index.js 组装）
  css/components/ 组件样式（按域拆分，index.html 按序加载保持级联）
  vendor/         本地第三方库（vue、xterm，内网零外联）
```

## 提交规范

Commit message 使用 `类型: 中文描述` 格式，与现有历史保持一致：

```
feat: 新增软件源批量管理
fix: 修复 SSH 连接池空闲超时未释放
docs: 完善部署文档
refactor: 抽离 Docker 检测逻辑
```

常用类型：`feat` / `fix` / `docs` / `refactor` / `chore` / `test`

## Pull Request 流程

1. 从 `main` 拉取功能分支：`feat/<简短描述>` 或 `fix/<简短描述>`
2. 开发完成后推送分支并向 `main` 发起 PR
3. PR 描述包含：改动内容、动机、自测方式
4. 涉及用户可见功能或行为变更时，同步更新 [CHANGELOG.md](CHANGELOG.md) 的 Unreleased 节
5. 确保本地 `make all` 通过（CI 会执行 `make fmt-check vet build test`，任一失败将无法合并）
6. 等待维护者评审，按反馈迭代后合并

## 贡献软件包与语言定义

本项目最有价值的贡献之一是丰富内置的软件包目录与语言环境定义（存储于 SQLite，首次启动写入默认数据）：

- **软件包**：安装命令、服务启停命令、检测方式（见 `internal/software/`）
- **语言环境**：多版本检测与安装脚本（Go / Java / Python / Node.js / Rust / PHP / Ruby / .NET 等）

新增定义时请注意：

- 优先支持 Ubuntu/Debian 与 CentOS/RHEL 两大家族的包管理器差异
- 安装脚本需幂等（重复执行不产生副作用）
- 涉及 sudo 的命令需兼容 `sudo -n` 免密场景
- 在 PR 中注明测试过的系统发行版

## 贡献与许可

本项目采用[非商业使用许可协议](LICENSE)：个人学习与组织内部运维免费，直接营利行为需商业授权。

**提交 PR 即表示你同意**：

1. 你的贡献内容按本项目的非商业使用许可协议对外分发；
2. 授权项目维护者以你的名义对贡献部分行使商业许可授予权利（即贡献可被纳入商业授权范围，收益归属项目）；
3. 贡献内容为你的原创或你有权许可的代码，不侵犯第三方权利。

如无法接受上述条款，请先与维护者沟通。

## 安全问题报告

OpsPanel 定位为**内网工具**：内置单管理员登录认证，但安全机制较薄（无 HTTPS、无多因素、无审计日志），请勿在公网暴露使用（详见 [SECURITY.md](SECURITY.md) 与 [README 风险提示](README.zh-CN.md#风险提示)）。

如发现代码层面的安全缺陷（命令注入、路径穿越、凭证处理不当等），请勿公开 Issue，通过 [GitHub Security Advisories](https://github.com/smash-hq/opspanel/security/advisories/new) 私下报告，我们会在修复后致谢。
