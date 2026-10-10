# SSH 用户 Sudo 权限配置指南

当使用非 root 用户连接远程服务器时，部分系统信息采集可能需要 root 权限。Server Plane 会自动尝试使用 `sudo -n`（非交互式 sudo）作为回退。如果 sudo 不可用，页面会显示权限警告。

## 目录

- [快速配置（推荐）](#快速配置推荐)
- [仅授权必要命令（更安全）](#仅授权必要命令更安全)
- [验证配置](#验证配置)
- [常见问题](#常见问题)

---

## 快速配置（推荐）

允许指定用户免密执行所有 sudo 命令：

### 步骤

1. **登录远程服务器**（使用 root 或有 sudo 权限的用户）

2. **创建 sudoers 配置文件**：

   ```bash
   sudo visudo -f /etc/sudoers.d/your_username
   ```

3. **添加以下行**（将 `your_username` 替换为实际用户名）：

   ```
   your_username ALL=(ALL) NOPASSWD: ALL
   ```

4. **保存并退出**（vim 中按 `:wq`）

5. **验证权限**：

   ```bash
   su - your_username
   sudo -n whoami
   # 应输出: root
   ```

---

## 仅授权必要命令（更安全）

为最小化安全风险，仅授权 Server Plane 采集所需的命令：

```bash
sudo visudo -f /etc/sudoers.d/server-plane
```

添加以下内容（将 `your_username` 替换为实际用户名）：

```
your_username ALL=(ALL) NOPASSWD: /usr/bin/lscpu, /usr/bin/top, /usr/bin/free, /bin/df, /bin/cat, /usr/bin/hostname
```

> **说明**：当前版本的采集命令大多通过 `/proc` 和 `/sys` 文件系统读取信息，通常不需要 root 权限。此配置主要为未来扩展功能准备。

---

## 验证配置

### 方法一：SSH 登录验证

```bash
ssh your_username@your_server
sudo -n whoami
# 应输出: root（无需输入密码）
```

### 方法二：在 Server Plane 中测试

1. 添加服务器时填写凭据
2. 点击 **「⚡ 测试连接」** 按钮
3. 如果连接成功但显示权限警告，说明 sudo 配置未生效
4. 配置 sudo 后重新连接，警告应消失

---

## 常见问题

### Q: 为什么使用 `visudo` 而不是直接编辑 `/etc/sudoers`？

`visudo` 会在保存前检查语法错误，防止因配置错误导致所有用户都无法使用 sudo。

### Q: `NOPASSWD: ALL` 有什么风险？

允许用户免密执行任何命令。如果该用户的 SSH 密钥泄露，攻击者可以执行任意操作。生产环境建议：

- 使用 SSH 密钥认证（禁用密码登录）
- 仅授权必要命令（见上方「仅授权必要命令」）
- 限制来源 IP（在 `/etc/hosts.allow` 中配置）

### Q: 配置后仍然显示权限警告？

1. 确认用户名拼写正确
2. 确认文件在 `/etc/sudoers.d/` 目录下
3. 确认文件权限为 `0440`：
   ```bash
   sudo chmod 0440 /etc/sudoers.d/your_username
   ```
4. 检查 `/etc/sudoers` 主配置中是否包含 `#includedir /etc/sudoers.d`

### Q: 能否使用 root 用户直接连接？

可以。使用 root 用户连接将获取完整权限，无需配置 sudo。但出于安全考虑，建议：

- 使用普通用户 + sudo 方式
- 在 SSH 配置中禁用 root 远程登录：
  ```
  PermitRootLogin no
  ```

---

## 参考

- `man sudoers` — sudo 配置手册
- `man visudo` — visudo 手册
- [OpenSSH 文档](https://www.openssh.com/manual.html)
