# Server Plane 前端

浏览器控制台界面，Vue 3 单页应用，通过 Go `embed.FS` 嵌入二进制交付。

## 技术栈

| 类别 | 选型 |
|------|------|
| 框架 | Vue 3（CDN 全局构建） |
| 语言 | 原生 JavaScript（ES Module） |
| 样式 | 原生 CSS（CSS 变量 + 分层架构） |
| 构建 | 无（浏览器原生 ES Module，零构建步骤） |
| 部署 | Go `embed.FS` 单二进制嵌入 |

## 目录结构

```
web/
├── README.md              # 本文件
├── static/
│   ├── index.html         # 入口 HTML（挂载点 + 资源引用）
│   ├── css/
│   │   ├── base.css       # 基础层：CSS 变量、全局重置、排版
│   │   ├── layout.css     # 布局层：侧栏、主区、网格、响应式
│   │   ├── components.css # 组件层：按钮、卡片、面板、表单、模态框
│   │   └── states.css     # 状态层：状态点、加载、错误、Toast、动画
│   └── js/
│       ├── app.js         # 入口：Vue.createApp → mount
│       ├── store.js       # 状态中心：响应式状态、计算属性、业务操作
│       ├── api.js         # API 客户端：REST 接口封装
│       ├── ws.js          # WebSocket 管理器：连接、重连、消息分发
│       └── utils.js       # 工具函数：格式化、转换（纯函数）
└── static.go              # Go embed 指令（//go:embed static）
```

## 模块依赖

```
app.js ──► store.js ──► api.js
                 │       ws.js
                 │       utils.js
                 ▼
           Vue (全局)
```

- `utils.js` / `api.js` / `ws.js` — 无状态，纯函数或工厂函数
- `store.js` — 组合层，导入以上三个模块，创建响应式状态
- `app.js` — 入口，导入 store，挂载 Vue 实例

## 开发指南

### 本地开发

```bash
# 启动服务（前端资源自动嵌入）
go run ./cmd/server/

# 浏览器打开
open http://localhost:8080
```

修改 `static/` 下的文件后重新 `go run` 即可生效（ES Module 无缓存问题，强制刷新浏览器即可）。

### 添加新功能

1. **新页面/视图** — 在 `index.html` 添加模板，在 `store.js` 添加状态和操作
2. **新 API** — 在 `api.js` 添加函数，在 `store.js` 中调用
3. **新组件样式** — 在 `components.css` 添加对应选择器
4. **新工具函数** — 在 `utils.js` 添加纯函数

### 后端 API 约定

- 基础路径：`/api/v1/`
- 响应格式：`{ code: 0, message: "ok", data: ... }`
- `code === 0` 表示成功，非 0 表示失败（`message` 为错误描述）
- WebSocket：`/ws?server=<id>`，推送 `{ type: "snapshot", data: { ... } }`

#### 服务器管理 API

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/servers` | 列表（不含密码） |
| POST | `/api/v1/servers` | 新增 |
| POST | `/api/v1/servers/test` | 连接测试（不保存） |
| PUT | `/api/v1/servers/{id}` | 更新 |
| DELETE | `/api/v1/servers/{id}` | 删除 |
| GET | `/api/v1/system/info?server={id}` | 系统快照 |

### 编码约定

- JS：camelCase 命名，JSDoc 注释所有导出函数
- CSS：kebab-case 类名，BEM 风格用于复合组件（如 `panel-heading`）
- Vue：Composition API（`setup()`），模板中直接使用 store 导出的状态/方法
- 不引入构建工具，保持零依赖、单二进制部署

## 浏览器兼容性

ES Module 需要现代浏览器（Chrome 61+、Firefox 60+、Safari 11+、Edge 16+）。
