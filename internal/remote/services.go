package remote

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"
)

// ErrNoSystemd 目标机无 systemd（如 macOS 开发机、精简容器），服务管理不可用。
var ErrNoSystemd = errors.New("目标机未检测到 systemd，服务管理不可用")

// unitNamePattern 限制 systemd 单元名字符集，防止拼入 shell 命令造成注入。
// systemd 单元名仅含字母数字与 . _ @ -（@ 用于模板实例化）。
var unitNamePattern = regexp.MustCompile(`^[A-Za-z0-9_.@-]+$`)

// serviceActions 允许的服务操作白名单。
var serviceActions = map[string]bool{
	"start":   true,
	"stop":    true,
	"restart": true,
	"enable":  true,
	"disable": true,
}

// ServiceInfo 表示单个 systemd 服务的状态。
type ServiceInfo struct {
	Name        string `json:"name"`         // 单元名（如 nginx.service）
	Description string `json:"description"`  // 单元描述
	ActiveState string `json:"active_state"` // active/inactive/failed/activating...
	SubState    string `json:"sub_state"`    // running/dead/auto-restart...
	Enabled     string `json:"enabled"`      // enabled/disabled/static/masked...
}

// ListServices 列出目标机的 systemd 服务。
// 单次往返合并 list-units（运行状态+描述）与 list-unit-files（开机自启状态+未加载单元），
// 按单元名合并去重；状态排序：failed > activating > active > 其他。
func (rc *RemoteCollector) ListServices(ctx context.Context, srv *Server) ([]ServiceInfo, error) {
	conn, err := rc.Acquire(srv)
	if err != nil {
		return nil, err
	}
	defer rc.Release(srv.ID)

	ctx, cancel := withDefaultTimeout(ctx, DefaultCmdTimeout)
	defer cancel()

	// 先探测 systemctl 是否存在（macOS 本机等无 systemd 场景直接返回标记），
	// 再合并两条列表命令，减少 SSH 往返延迟
	cmd := `command -v systemctl >/dev/null 2>&1 || { echo __NOSYSTEMD__; exit 0; }; ` +
		`systemctl list-units --type=service --all --no-legend --no-pager 2>/dev/null; ` +
		`echo ---SEP---; ` +
		`systemctl list-unit-files --type=service --no-legend --no-pager 2>/dev/null`
	out, err := conn.RunOnce(ctx, cmd)
	if err != nil {
		return nil, err
	}
	if strings.HasPrefix(out, "__NOSYSTEMD__") {
		return nil, ErrNoSystemd
	}

	services := make(map[string]*ServiceInfo)
	parts := strings.SplitN(out, "---SEP---", 2)

	// 第一段：list-units（已加载单元，列为 UNIT LOAD ACTIVE SUB DESCRIPTION）
	for _, line := range strings.Split(parts[0], "\n") {
		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}
		name, load, active, sub := fields[0], fields[1], fields[2], fields[3]
		if !strings.HasSuffix(name, ".service") || load == "not-found" {
			continue
		}
		desc := ""
		if len(fields) > 4 {
			desc = strings.Join(fields[4:], " ")
		}
		services[name] = &ServiceInfo{
			Name:        name,
			Description: desc,
			ActiveState: active,
			SubState:    sub,
		}
	}

	// 第二段：list-unit-files（全部单元文件，列为 UNIT FILE STATE [PRESET...]），
	// 补充自启状态；未加载的服务补充为 inactive
	if len(parts) > 1 {
		for _, line := range strings.Split(parts[1], "\n") {
			fields := strings.Fields(line)
			if len(fields) < 2 || !strings.HasSuffix(fields[0], ".service") {
				continue
			}
			name, state := fields[0], fields[1]
			if s, ok := services[name]; ok {
				s.Enabled = state
			} else {
				services[name] = &ServiceInfo{
					Name:        name,
					ActiveState: "inactive",
					SubState:    "dead",
					Enabled:     state,
				}
			}
		}
	}

	result := make([]ServiceInfo, 0, len(services))
	for _, s := range services {
		result = append(result, *s)
	}
	sort.Slice(result, func(i, j int) bool {
		pi, pj := serviceStatePriority(result[i].ActiveState), serviceStatePriority(result[j].ActiveState)
		if pi != pj {
			return pi < pj
		}
		return result[i].Name < result[j].Name
	})
	return result, nil
}

// serviceStatePriority 服务状态排序权重：异常状态靠前，便于快速发现故障服务。
func serviceStatePriority(state string) int {
	switch state {
	case "failed":
		return 0
	case "activating":
		return 1
	case "active":
		return 2
	default:
		return 3
	}
}

// ServiceAction 对指定服务执行操作（start/stop/restart/enable/disable）。
// 先直接执行，失败（权限不足）回退 sudo -n；失败时保留远端错误输出返回给前端。
func (rc *RemoteCollector) ServiceAction(ctx context.Context, srv *Server, name, action string) error {
	action = strings.TrimSpace(action)
	if !serviceActions[action] {
		return fmt.Errorf("不支持的操作: %s", action)
	}
	name = strings.TrimSpace(name)
	if name == "" || !unitNamePattern.MatchString(name) {
		return fmt.Errorf("非法的服务名: %s", name)
	}

	conn, err := rc.Acquire(srv)
	if err != nil {
		return err
	}
	defer rc.Release(srv.ID)

	// restart 在服务停止缓慢时可能超过默认 30s（systemd 默认停止超时 90s），放宽到 2 分钟
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	// 单元名已经正则校验，shQuote 再包一层防御纵深
	cmd := fmt.Sprintf("systemctl %s %s 2>&1", action, shQuote(name))
	out, err := conn.RunOnce(ctx, cmd)
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		// 直接执行失败（普通用户无权限操作 system 单元），回退 sudo -n
		sudoOut, sudoErr := conn.RunOnce(ctx, sudoWrap(cmd))
		if sudoErr != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			// 保留远端错误输出，便于前端提示真实原因
			if msg := strings.TrimSpace(sudoOut); msg != "" {
				return fmt.Errorf("%s", msg)
			}
			return fmt.Errorf("执行失败: %w", sudoErr)
		}
		out = sudoOut
	}
	rc.log.Infow("systemd service action", "action", action, "unit", name, "output", strings.TrimSpace(out))
	return nil
}

// unitFragmentPath 查询单元的 FragmentPath（主单元文件路径）。
// 兼容老版本 systemd：不使用 --value 选项，解析 FragmentPath= 键值对。
// 返回空字符串表示该服务无独立单元文件（如由生成器动态生成）。
func (rc *RemoteCollector) unitFragmentPath(ctx context.Context, conn commandRunner, name string) (string, error) {
	out, err := conn.RunOnce(ctx, fmt.Sprintf("systemctl show -p FragmentPath %s 2>/dev/null", shQuote(name)))
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(out, "\n") {
		if p := strings.TrimPrefix(line, "FragmentPath="); p != line {
			p = strings.TrimSpace(p)
			if p != "" {
				return p, nil
			}
		}
	}
	return "", nil
}

// GetUnitFile 读取服务的 unit 文件内容。返回 (path, content)；
// path 为空表示该服务无独立单元文件。
func (rc *RemoteCollector) GetUnitFile(ctx context.Context, srv *Server, name string) (string, string, error) {
	name = strings.TrimSpace(name)
	if name == "" || !unitNamePattern.MatchString(name) {
		return "", "", fmt.Errorf("非法的服务名: %s", name)
	}
	conn, err := rc.Acquire(srv)
	if err != nil {
		return "", "", err
	}
	defer rc.Release(srv.ID)

	ctx, cancel := withDefaultTimeout(ctx, DefaultCmdTimeout)
	defer cancel()

	frag, err := rc.unitFragmentPath(ctx, conn, name)
	if err != nil {
		return "", "", err
	}
	if frag == "" {
		return "", "", nil
	}
	// unit 文件通常 644 权限，普通用户即可读取
	out, err := conn.RunOnce(ctx, fmt.Sprintf("cat -- %s 2>/dev/null", shQuote(frag)))
	if err != nil {
		return "", "", fmt.Errorf("读取 %s 失败: %w", frag, err)
	}
	return frag, out, nil
}

// SaveUnitFile 保存 unit 文件：内容经 base64 传输避免引号注入；
// 覆盖前尽力备份为 *.bak；sudo -n 优先，失败回退直接写入；成功后执行 daemon-reload。
func (rc *RemoteCollector) SaveUnitFile(ctx context.Context, srv *Server, name, content string) error {
	name = strings.TrimSpace(name)
	if name == "" || !unitNamePattern.MatchString(name) {
		return fmt.Errorf("非法的服务名: %s", name)
	}
	conn, err := rc.Acquire(srv)
	if err != nil {
		return err
	}
	defer rc.Release(srv.ID)

	ctx, cancel := withDefaultTimeout(ctx, DefaultCmdTimeout)
	defer cancel()

	frag, err := rc.unitFragmentPath(ctx, conn, name)
	if err != nil {
		return fmt.Errorf("定位单元文件失败: %w", err)
	}
	if frag == "" {
		return fmt.Errorf("未找到 %s 的单元文件（可能由生成器动态生成）", name)
	}
	// 路径来自远端输出，拼入 shell 前校验特殊字符
	if strings.ContainsAny(frag, "'\"\n$;&|<>") {
		return fmt.Errorf("单元文件路径异常: %s", frag)
	}

	// 写临时文件 → 尽力备份 .bak → sudo 优先覆盖并 daemon-reload（回退直接执行）
	tmp := "/tmp/.opspanel-unit.tmp"
	b64 := base64.StdEncoding.EncodeToString([]byte(content))
	cmd := fmt.Sprintf("echo %s | base64 -d > %q && { [ -f %q.bak ] || cp -f %q %q.bak; } 2>/dev/null; { sudo -n sh -c 'cp -f %q %q && systemctl daemon-reload' || { cp -f %q %q && systemctl daemon-reload; }; } && echo __UNIT_SAVED__",
		b64, tmp, frag, frag, frag, tmp, frag, tmp, frag)
	out, err := conn.RunOnce(ctx, cmd)
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("保存失败: %w", err)
	}
	if !strings.Contains(out, "__UNIT_SAVED__") {
		return fmt.Errorf("保存失败: %s", strings.TrimSpace(out))
	}
	rc.log.Infow("unit file saved", "unit", name, "path", frag)
	return nil
}
