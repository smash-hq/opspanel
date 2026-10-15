// Package remote 提供通过 SSH 在远程服务器上执行命令与采集信息的能力。
// 独立于 host/storage/software 包，避免循环依赖。
package remote

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/smash-hq/opspanel/internal/logger"
	"github.com/smash-hq/opspanel/internal/system"
	"golang.org/x/crypto/ssh"
)

// Server 是 SSH 连接的配置。字段与 storage.Server 一致。
type Server struct {
	ID        string
	Name      string
	Type      string
	Host      string
	Port      int
	Username  string
	AuthType  string
	Password  string
	KeyPath   string
	CreatedAt int64
}

const (
	// DefaultCmdTimeout 单条命令的默认超时：调用方 ctx 未携带 deadline 时自动附加，
	// 避免单条命令在异常远端上无限挂起。
	DefaultCmdTimeout = 30 * time.Second

	// LongOpTimeout 安装/卸载语言环境、拉取镜像、容器操作、文件下载等长操作的默认超时。
	LongOpTimeout = 30 * time.Minute
)

// withDefaultTimeout 在 ctx 尚无 deadline 时附加 d 超时，否则原样返回。
// 长操作入口先套 LongOpTimeout 后，内部命令自然沿用长 deadline，不会被 30s 默认值覆盖。
func withDefaultTimeout(ctx context.Context, d time.Duration) (context.Context, context.CancelFunc) {
	if _, ok := ctx.Deadline(); ok {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, d)
}

// RemoteCollector 通过 SSH 在远程 Linux 服务器上采集系统信息。
type RemoteCollector struct {
	log  *logger.Logger
	pool *ConnPool
}

// NewRemoteCollector 创建远程采集器。
// 所有实例共享同一个 SSH 连接池：连接按需建立、空闲时优雅关闭。
func NewRemoteCollector(log *logger.Logger) *RemoteCollector {
	return &RemoteCollector{log: log.Named("remote"), pool: getSharedPool(log)}
}

// sshConn 封装 *ssh.Client，每次执行命令新建 Session。
// SSH 协议原生支持单连接多路复用，ssh.Client 可安全并发使用，
// 因此不做全局串行化——否则长命令（如 docker restart）会阻塞该服务器的一切操作。
type sshConn struct {
	client        *ssh.Client
	broken        atomic.Bool   // 断线标记：命令失败或保活失败时置位，池会在下次获取时重建
	stopOnce      sync.Once     // 保证保活停止通道只关闭一次
	stopKeepalive chan struct{} // 关闭时通知保活协程退出
}

// newSSHConn 建立 SSH 连接。
func newSSHConn(srv *Server) (*sshConn, error) {
	config := &ssh.ClientConfig{
		User:            srv.Username,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         10 * time.Second,
	}

	switch srv.AuthType {
	case "key":
		if srv.KeyPath == "" {
			return nil, fmt.Errorf("key path is required for key auth")
		}
		key, err := os.ReadFile(srv.KeyPath)
		if err != nil {
			return nil, fmt.Errorf("read key file: %w", err)
		}
		signer, err := ssh.ParsePrivateKey(key)
		if err != nil {
			return nil, fmt.Errorf("parse private key: %w", err)
		}
		config.Auth = []ssh.AuthMethod{ssh.PublicKeys(signer)}
	default:
		if srv.Password == "" {
			return nil, fmt.Errorf("password is required for password auth")
		}
		config.Auth = []ssh.AuthMethod{ssh.Password(srv.Password)}
	}

	addr := net.JoinHostPort(srv.Host, strconv.Itoa(srv.Port))
	client, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		return nil, fmt.Errorf("ssh dial %s: %w", addr, err)
	}

	return &sshConn{client: client, stopKeepalive: make(chan struct{})}, nil
}

// runCombined 在独立 goroutine 中执行命令并等待其完成或 ctx 取消。
// ctx 取消/超时时立即关闭 Session 终止远程命令；CombinedOutput 在 Session 关闭后返回，
// 缓冲通道保证子协程不会泄漏。
func (c *sshConn) runCombined(ctx context.Context, session *ssh.Session, cmd string) ([]byte, error) {
	type result struct {
		out []byte
		err error
	}
	done := make(chan result, 1)
	go func() {
		out, err := session.CombinedOutput(cmd)
		done <- result{out, err}
	}()

	select {
	case r := <-done:
		return r.out, r.err
	case <-ctx.Done():
		session.Close()
		return nil, ctx.Err()
	}
}

// RunOnce 执行单条命令（每次新建 Session）。
// ctx 超时或取消会关闭 Session 终止远程命令。
func (c *sshConn) RunOnce(ctx context.Context, cmd string) (string, error) {
	session, err := c.client.NewSession()
	if err != nil {
		return "", fmt.Errorf("new session: %w", err)
	}
	defer session.Close()

	out, err := c.runCombined(ctx, session, cmd)
	if err != nil {
		if ctx.Err() != nil {
			// ctx 取消/超时是主动关闭 Session，连接本身仍可用，不标记断开
			return string(out), ctx.Err()
		}
		// 命令非零退出（ExitError）是正常业务结果（如 grep 无匹配、sudo -n 免密失败），
		// 只有连接级错误（EOF/网络断开等）才标记断开，由连接池重建
		var exitErr *ssh.ExitError
		if !errors.As(err, &exitErr) {
			c.broken.Store(true)
		}
		return string(out), err
	}
	return strings.TrimSpace(string(out)), nil
}

// RunRaw 执行单条命令并返回原始字节输出（不 TrimSpace，用于二进制传输如文件下载）。
func (c *sshConn) RunRaw(ctx context.Context, cmd string) ([]byte, error) {
	session, err := c.client.NewSession()
	if err != nil {
		return nil, fmt.Errorf("new session: %w", err)
	}
	defer session.Close()

	out, err := c.runCombined(ctx, session, cmd)
	if err != nil {
		if ctx.Err() != nil {
			return out, ctx.Err()
		}
		var exitErr *ssh.ExitError
		if !errors.As(err, &exitErr) {
			c.broken.Store(true)
		}
	}
	return out, err
}

// usable 返回连接是否仍可使用（未被标记断开）。
func (c *sshConn) usable() bool { return !c.broken.Load() }

// Close 优雅关闭 SSH 连接：停止保活协程，并向对端发送 disconnect。
func (c *sshConn) Close() error {
	c.stopOnce.Do(func() { close(c.stopKeepalive) })
	return c.client.Close()
}

// LocalServerID 本机节点固定 ID（与 storage/host 包一致；remote 不依赖 host 包，故此处重复定义）。
const LocalServerID = "srv_local"

// commandRunner 抽象命令执行通道：远程节点为池化 SSH 连接，本机节点为进程内执行器。
type commandRunner interface {
	RunOnce(ctx context.Context, cmd string) (string, error)
	RunRaw(ctx context.Context, cmd string) ([]byte, error)
	usable() bool
	Close() error
}

// sharedLocal 本机执行器共享实例（无状态，可并发）。
var sharedLocal commandRunner = &localShell{}

// isLocal 判断是否本机节点（本机走进程内执行，无需 SSH）。
func isLocal(srv *Server) bool {
	return srv == nil || srv.ID == LocalServerID || srv.Type == "local"
}

// Acquire 获取服务器的命令执行通道：本机节点返回进程内执行器（无需 SSH 凭据与 root 权限），
// 远程节点从连接池获取 SSH 连接（池中不存在或已断线时自动重建）。
func (rc *RemoteCollector) Acquire(srv *Server) (commandRunner, error) {
	if isLocal(srv) {
		return sharedLocal, nil
	}
	return rc.pool.Acquire(srv)
}

// Release 归还连接到池（本机执行器为共享实例，无需归还）。
func (rc *RemoteCollector) Release(serverID string) {
	if serverID == LocalServerID {
		return
	}
	rc.pool.Release(serverID)
}

// CloseConn 优雅关闭指定服务器的池化连接（更新/删除服务器配置时调用；本机执行器无需处理）。
func (rc *RemoteCollector) CloseConn(serverID string) {
	if serverID == LocalServerID {
		return
	}
	rc.pool.CloseConn(serverID)
}

// runLogged 执行命令并记录调试日志，返回输出与错误（错误时输出置空）。
// ctx 无 deadline 时附加 30s 默认超时。
func (rc *RemoteCollector) runLogged(ctx context.Context, client commandRunner, cmd string) (string, error) {
	ctx, cancel := withDefaultTimeout(ctx, DefaultCmdTimeout)
	defer cancel()

	out, err := client.RunOnce(ctx, cmd)
	if err != nil {
		rc.log.Debugw("remote cmd error", "cmd", cmd, "error", err.Error())
		return "", err
	}
	// 截断过长的输出用于日志
	logOut := out
	if len(logOut) > 200 {
		logOut = logOut[:200] + "..."
	}
	rc.log.Debugw("remote cmd", "cmd", cmd, "output", logOut)
	return out, nil
}

// RunOne 在目标服务器上执行单条命令并返回输出。
// 命令执行日志为 Debug 级：周期性采集（pushLoop）会高频调用，Info 级会刷屏。
// 排查问题时将日志级别调至 debug 即可看到每条命令及输出。
func (rc *RemoteCollector) RunOne(ctx context.Context, client commandRunner, cmd string) string {
	out, _ := rc.runLogged(ctx, client, cmd)
	return out
}

// RunWithSudo 先尝试直接执行命令，按退出码判断失败，失败则使用 sudo -n 回退。
// 不能以输出为空判定失败：部分命令成功时输出为空。
func (rc *RemoteCollector) RunWithSudo(ctx context.Context, client commandRunner, cmd string) string {
	// 先尝试直接执行
	out, err := rc.runLogged(ctx, client, cmd)
	if err == nil {
		return out
	}
	// 失败则尝试 sudo -n（免密 sudo）
	sudoCmd := "sudo -n " + cmd
	rc.log.Debugw("remote cmd sudo fallback", "original_cmd", cmd, "sudo_cmd", sudoCmd)
	sudoOut, _ := rc.runLogged(ctx, client, sudoCmd)
	return sudoOut
}

// Snapshot 采集远程服务器的完整系统快照。
func (rc *RemoteCollector) Snapshot(ctx context.Context, srv *Server) (*system.Info, error) {
	conn, err := rc.Acquire(srv)
	if err != nil {
		return nil, err
	}
	defer rc.Release(srv.ID)

	info := &system.Info{
		Timestamp: time.Now().UnixMilli(),
	}

	// 主机信息
	info.Host.Hostname = rc.RunOne(ctx, conn, "hostname")
	info.Host.OS = rc.RunOne(ctx, conn, "uname -s")
	info.Host.Kernel = rc.RunOne(ctx, conn, "uname -r")
	info.Host.Arch = rc.RunOne(ctx, conn, "uname -m")
	info.Host.Platform = rc.RunOne(ctx, conn, "uname -v")
	// 系统语言环境：优先 LANG，回退 LC_ALL
	if lang := rc.RunOne(ctx, conn, "echo $LANG"); lang != "" {
		info.Host.Locale = lang
	} else if lc := rc.RunOne(ctx, conn, "echo $LC_ALL"); lc != "" {
		info.Host.Locale = lc
	}

	// CPU 信息
	info.CPU.Model = rc.RunOne(ctx, conn, "grep -m1 'model name' /proc/cpuinfo 2>/dev/null | cut -d: -f2")
	info.CPU.Model = strings.TrimSpace(info.CPU.Model)
	// 物理核心数
	if cores := rc.RunOne(ctx, conn, "grep -c ^processor /proc/cpuinfo 2>/dev/null"); cores != "" {
		fmt.Sscanf(cores, "%d", &info.CPU.Threads)
	}
	// 物理核心（cores）
	if physCores := rc.RunOne(ctx, conn, "grep 'cpu cores' /proc/cpuinfo 2>/dev/null | head -1 | awk '{print $4}'"); physCores != "" {
		fmt.Sscanf(physCores, "%d", &info.CPU.Cores)
	}
	// CPU 使用率
	if pct := rc.RunOne(ctx, conn, "top -bn1 2>/dev/null | grep 'Cpu(s)' | awk '{print $2}'"); pct != "" {
		fmt.Sscanf(pct, "%f", &info.CPU.Percent)
	}

	// 内存信息（使用 free 命令，无需 root）
	if mem := rc.RunOne(ctx, conn, "free -b 2>/dev/null | awk '/Mem:/{print $2, $3, $7}'"); mem != "" {
		var total, used, available uint64
		fmt.Sscanf(mem, "%d %d %d", &total, &used, &available)
		info.Memory.Total = total
		info.Memory.Used = used
		info.Memory.Free = available
		if total > 0 {
			info.Memory.Percent = float64(used) / float64(total) * 100
		}
	}

	// 进程数（无需 root）
	if procs := rc.RunWithSudo(ctx, conn, "ps -e --no-headers 2>/dev/null | wc -l"); procs != "" {
		fmt.Sscanf(strings.TrimSpace(procs), "%d", &info.Processes)
	}

	// 磁盘信息（runWithSudo 自动回退到 sudo -n）
	if disks := rc.RunWithSudo(ctx, conn, "df -B1 --output=source,size,used,target -x tmpfs -x devtmpfs -x overlay 2>/dev/null | tail -n +2"); disks != "" {
		for _, line := range strings.Split(disks, "\n") {
			fields := strings.Fields(line)
			if len(fields) >= 4 {
				var total, used uint64
				fmt.Sscanf(fields[1], "%d", &total)
				fmt.Sscanf(fields[2], "%d", &used)
				pct := 0.0
				if total > 0 {
					pct = float64(used) / float64(total) * 100
				}
				info.Disks = append(info.Disks, system.DiskInfo{
					Device:  fields[0],
					Total:   total,
					Used:    used,
					Mount:   fields[3],
					Percent: pct,
				})
			}
		}
	}

	// 运行时间（无需 root）
	if uptime := rc.RunOne(ctx, conn, "cat /proc/uptime 2>/dev/null | awk '{print $1}'"); uptime != "" {
		var secs float64
		fmt.Sscanf(uptime, "%f", &secs)
		info.Uptime = uint64(secs)
	}

	// 系统负载（无需 root）
	if load := rc.RunOne(ctx, conn, "cat /proc/loadavg 2>/dev/null | awk '{print $1, $2, $3}'"); load != "" {
		fmt.Sscanf(load, "%f %f %f", &info.Load.Load1, &info.Load.Load5, &info.Load.Load15)
	}

	// 检测是否使用了 sudo（权限不足标记）
	// 如果进程数异常低，可能是权限不足
	if info.Processes > 0 && info.Processes < 10 {
		if procs := rc.RunOne(ctx, conn, "sudo -n ps -e --no-headers 2>/dev/null | wc -l"); procs != "" {
			var sudoProcs int
			fmt.Sscanf(strings.TrimSpace(procs), "%d", &sudoProcs)
			if sudoProcs > info.Processes {
				info.Processes = sudoProcs
				info.SudoUsed = true
			}
		}
	}

	return info, nil
}

// GetLog 获取软件运行日志（systemd 服务或 Docker 容器）。
// lines 指定返回最后多少行，默认 100。
func (rc *RemoteCollector) GetLog(ctx context.Context, srv *Server, service string, lines int) (string, error) {
	if lines <= 0 {
		lines = 100
	}
	conn, err := rc.Acquire(srv)
	if err != nil {
		return "", err
	}
	defer rc.Release(srv.ID)

	// 优先尝试 journalctl（systemd 服务）；service 名来自用户输入，须引号包裹防命令注入
	cmd := fmt.Sprintf("journalctl -u %q --no-pager -n %d 2>/dev/null", service, lines)
	out := rc.RunOne(ctx, conn, cmd)
	if out != "" {
		return out, nil
	}

	// 回退：尝试 docker logs
	cmd = fmt.Sprintf("docker logs --tail %d %q 2>/dev/null", lines, service)
	out = rc.RunOne(ctx, conn, cmd)
	if out != "" {
		return out, nil
	}

	// 回退：尝试从 /var/log 读取（%q 拼接后引号外的 * 仍可 glob）
	cmd = fmt.Sprintf("tail -n %[1]d /var/log/%[2]q*.log 2>/dev/null || tail -n %[1]d /var/log/%[2]q/%[2]q.log 2>/dev/null", lines, service)
	out = rc.RunOne(ctx, conn, cmd)
	if out != "" {
		return out, nil
	}

	return "", fmt.Errorf("no log found for service: %s", service)
}

// ProcessInfo 表示单个进程的信息。
type ProcessInfo struct {
	PID     int    `json:"pid"`
	User    string `json:"user"`
	CPU     string `json:"cpu"`
	Mem     string `json:"mem"`
	Command string `json:"command"`
}

// ListProcesses 获取远程服务器进程列表（类似 top/ps）。
func (rc *RemoteCollector) ListProcesses(ctx context.Context, srv *Server, limit int) ([]ProcessInfo, error) {
	if limit <= 0 {
		limit = 50
	}
	conn, err := rc.Acquire(srv)
	if err != nil {
		return nil, err
	}
	defer rc.Release(srv.ID)

	// 使用 ps 命令获取进程列表，按 CPU 使用率排序
	cmd := fmt.Sprintf("ps aux --sort=-%%cpu 2>/dev/null | head -n %d", limit+1)
	out := rc.RunOne(ctx, conn, cmd)
	if out == "" {
		return nil, fmt.Errorf("failed to list processes")
	}

	var processes []ProcessInfo
	lines := strings.Split(out, "\n")
	// 跳过标题行
	for i, line := range lines {
		if i == 0 {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 11 {
			continue
		}
		var pid int
		fmt.Sscanf(fields[1], "%d", &pid)
		cmdStr := strings.Join(fields[10:], " ")
		// 截断过长的命令行
		if len(cmdStr) > 80 {
			cmdStr = cmdStr[:77] + "..."
		}
		processes = append(processes, ProcessInfo{
			PID:     pid,
			User:    fields[0],
			CPU:     fields[2],
			Mem:     fields[3],
			Command: cmdStr,
		})
	}
	return processes, nil
}

// DetectLanguage 检测指定编程语言的版本。返回空字符串表示未安装。
func (rc *RemoteCollector) DetectLanguage(ctx context.Context, srv *Server, versionCmd string) string {
	conn, err := rc.Acquire(srv)
	if err != nil {
		return ""
	}
	defer rc.Release(srv.ID)
	return rc.RunOne(ctx, conn, versionCmd)
}

// InstallLanguage 执行语言环境安装命令。version 为空则不替换版本占位符。
func (rc *RemoteCollector) InstallLanguage(ctx context.Context, srv *Server, installCmd, version string) error {
	// 安装是长操作，套 30 分钟超时（调用方已有更紧的 deadline 时以调用方为准）
	ctx, cancel := context.WithTimeout(ctx, LongOpTimeout)
	defer cancel()

	conn, err := rc.Acquire(srv)
	if err != nil {
		return err
	}
	defer rc.Release(srv.ID)

	// 替换版本占位符
	cmd := installCmd
	if version != "" {
		cmd = strings.ReplaceAll(cmd, "{{VERSION}}", version)
	}

	// 执行安装命令
	out := rc.RunWithSudo(ctx, conn, cmd)
	rc.log.Infow("install language", "cmd", cmd, "version", version, "output", out)
	return nil
}

// --- 文件管理 ---

// FileInfo 表示文件或目录的信息。
type FileInfo struct {
	Name       string `json:"name"`
	Path       string `json:"path"`
	IsDir      bool   `json:"is_dir"`
	Size       int64  `json:"size"`
	Mode       string `json:"mode"`
	ModTime    int64  `json:"mod_time"`
	IsHidden   bool   `json:"is_hidden"`
	LinkTarget string `json:"link_target,omitempty"` // 软链接目标（仅软链接条目非空）
}

// ListFilesResult 包含文件列表和 sudo 使用状态。
type ListFilesResult struct {
	Files    []FileInfo `json:"files"`
	SudoUsed bool       `json:"sudo_used"`
}

// ListFiles 列出指定目录的内容（类似 ls -la）。
// 先尝试 sudo ls，失败则回退到普通 ls，并标记 sudo 是否可用。
func (rc *RemoteCollector) ListFiles(ctx context.Context, srv *Server, path string) (*ListFilesResult, error) {
	conn, err := rc.Acquire(srv)
	if err != nil {
		return nil, err
	}
	defer rc.Release(srv.ID)

	if path == "" {
		path = "/"
	}

	sudoUsed := false
	// TZ=UTC + 自定义 time-style：时间输出为单字段 RFC3339，保证列数固定、时区无歧义
	lsCmd := "env TZ=UTC ls -la --time-style=+%Y-%m-%dT%H:%M:%SZ"
	cmd := fmt.Sprintf("%s %q 2>/dev/null", lsCmd, path)
	out := rc.RunOne(ctx, conn, "sudo -n "+cmd)
	if out != "" {
		sudoUsed = true
	} else {
		// sudo 失败，回退到普通 ls
		out = rc.RunOne(ctx, conn, cmd)
	}
	if out == "" {
		return nil, fmt.Errorf("cannot list directory: %s", path)
	}

	var files []FileInfo
	for _, line := range strings.Split(out, "\n") {
		fields := strings.Fields(line)
		// 固定 7 列：mode links owner group size time name...（name 可能含空格）
		if len(fields) < 7 {
			continue
		}
		mode := fields[0]
		if strings.HasPrefix(mode, "total") {
			continue
		}

		name := strings.Join(fields[6:], " ")
		var linkTarget string
		// 软链接行尾部为 "name... -> target"，按箭头拆出名字与目标
		for i := 6; i < len(fields)-1; i++ {
			if fields[i] == "->" {
				name = strings.Join(fields[6:i], " ")
				linkTarget = strings.Join(fields[i+1:], " ")
				break
			}
		}
		if name == "" || name == "." || name == ".." {
			continue
		}

		isDir := strings.HasPrefix(mode, "d")
		isHidden := strings.HasPrefix(name, ".")

		var size int64
		fmt.Sscanf(fields[4], "%d", &size)

		// 解析 UTC 修改时间为 Unix 时间戳
		var modTime int64
		if t, err := time.Parse(time.RFC3339, fields[5]); err == nil {
			modTime = t.Unix()
		}

		// 拼接完整路径
		fullPath := path
		if !strings.HasSuffix(fullPath, "/") {
			fullPath += "/"
		}
		fullPath += name

		// 如果是目录，追加 /
		if isDir && !strings.HasSuffix(fullPath, "/") {
			fullPath += "/"
		}

		files = append(files, FileInfo{
			Name:       name,
			Path:       fullPath,
			IsDir:      isDir,
			Size:       size,
			Mode:       mode,
			ModTime:    modTime,
			IsHidden:   isHidden,
			LinkTarget: linkTarget,
		})
	}

	return &ListFilesResult{Files: files, SudoUsed: sudoUsed}, nil
}

// GetFileInfo 获取单个文件的详细信息。
func (rc *RemoteCollector) GetFileInfo(ctx context.Context, srv *Server, path string) (*FileInfo, error) {
	conn, err := rc.Acquire(srv)
	if err != nil {
		return nil, err
	}
	defer rc.Release(srv.ID)

	cmd := fmt.Sprintf("stat -c '%%n|%%F|%%s|%%a|%%Y|%%U|%%G' %q 2>/dev/null", path)
	out := rc.RunWithSudo(ctx, conn, cmd)
	if out == "" {
		return nil, fmt.Errorf("file not found: %s", path)
	}

	parts := strings.Split(out, "|")
	if len(parts) < 5 {
		return nil, fmt.Errorf("failed to stat file: %s", path)
	}

	var size, modTime int64
	fmt.Sscanf(parts[2], "%d", &size)
	fmt.Sscanf(parts[4], "%d", &modTime)

	isDir := strings.Contains(parts[1], "directory")
	mode := parts[3]

	// %n 输出完整路径，名称取最后一段
	name := path
	if idx := strings.LastIndex(strings.TrimSuffix(path, "/"), "/"); idx >= 0 {
		name = strings.TrimSuffix(path, "/")[idx+1:]
	}

	return &FileInfo{
		Name:    name,
		Path:    path,
		IsDir:   isDir,
		Size:    size,
		Mode:    mode,
		ModTime: modTime,
	}, nil
}

// ReadFileContent 读取文件内容（用于预览）。
func (rc *RemoteCollector) ReadFileContent(ctx context.Context, srv *Server, path string, lines int) (string, error) {
	if lines <= 0 {
		lines = 100
	}
	conn, err := rc.Acquire(srv)
	if err != nil {
		return "", err
	}
	defer rc.Release(srv.ID)

	// 先检查文件大小，避免读取过大的文件
	cmd := fmt.Sprintf("stat -c %%s %q 2>/dev/null", path)
	out := rc.RunOne(ctx, conn, cmd)
	var size int64
	fmt.Sscanf(out, "%d", &size)
	if size > 1<<20 { // > 1MB
		return "", fmt.Errorf("file too large (%d bytes), max 1MB", size)
	}

	cmd = fmt.Sprintf("cat %q 2>/dev/null | head -n %d", path, lines)
	return rc.RunWithSudo(ctx, conn, cmd), nil
}

// DownloadFile 读取文件内容用于下载；目录会打包为 tar.gz 返回。
func (rc *RemoteCollector) DownloadFile(ctx context.Context, srv *Server, path string) ([]byte, string, error) {
	// 下载可能传输大文件/目录打包，属于长操作，套 30 分钟超时
	ctx, cancel := context.WithTimeout(ctx, LongOpTimeout)
	defer cancel()

	conn, err := rc.Acquire(srv)
	if err != nil {
		return nil, "", err
	}
	defer rc.Release(srv.ID)

	// 去掉尾部分隔符，拆出父目录与名称
	clean := strings.TrimSuffix(path, "/")
	if clean == "" {
		clean = "/"
	}
	parent, name := "/", clean
	if idx := strings.LastIndex(clean, "/"); idx >= 0 {
		parent = clean[:idx]
		name = clean[idx+1:]
		if parent == "" {
			parent = "/"
		}
	}
	if name == "" {
		return nil, "", fmt.Errorf("invalid path: %s", path)
	}

	// 目录打包为 tar.gz，普通文件直接 cat
	cmd := fmt.Sprintf("if [ -d %q ]; then tar -czf - -C %q %q 2>/dev/null; else cat %q 2>/dev/null; fi", clean, parent, name, clean)

	data, err := conn.RunRaw(ctx, cmd)
	if len(data) == 0 {
		// 权限不足时 sudo 回退
		data, _ = conn.RunRaw(ctx, "sudo -n "+cmd)
	}
	if len(data) == 0 {
		return nil, "", fmt.Errorf("cannot download: %s", path)
	}

	// gzip 魔数（1f 8b）说明走的是 tar 打包分支
	filename := name
	if len(data) > 2 && data[0] == 0x1f && data[1] == 0x8b {
		filename = name + ".tar.gz"
	}

	return data, filename, nil
}

// KillProcess 终止指定 PID 的进程。
func (rc *RemoteCollector) KillProcess(ctx context.Context, srv *Server, pid int, force bool) error {
	conn, err := rc.Acquire(srv)
	if err != nil {
		return err
	}
	defer rc.Release(srv.ID)

	signal := "TERM"
	if force {
		signal = "KILL"
	}
	cmd := fmt.Sprintf("kill -%s %d 2>&1", signal, pid)
	out := rc.RunOne(ctx, conn, cmd)
	if out != "" && strings.Contains(out, "Operation not permitted") {
		return fmt.Errorf("permission denied: try using sudo or running as root")
	}
	return nil
}

// GetProcessLog 获取指定进程的日志（通过 journalctl 或 /proc/{pid}/fd）。
func (rc *RemoteCollector) GetProcessLog(ctx context.Context, srv *Server, pid int, lines int) (string, error) {
	if lines <= 0 {
		lines = 50
	}
	conn, err := rc.Acquire(srv)
	if err != nil {
		return "", err
	}
	defer rc.Release(srv.ID)

	// 尝试通过 journalctl 获取进程日志
	cmd := fmt.Sprintf("journalctl _PID=%d --no-pager -n %d 2>/dev/null", pid, lines)
	out := rc.RunOne(ctx, conn, cmd)
	if out != "" {
		return out, nil
	}

	// 回退：尝试读取 /proc/{pid}/fd/1 和 fd/2（stdout/stderr）
	cmd = fmt.Sprintf("sudo -n tail -n %d /proc/%d/fd/1 2>/dev/null; sudo -n tail -n %d /proc/%d/fd/2 2>/dev/null", lines, pid, lines, pid)
	out = rc.RunOne(ctx, conn, cmd)
	if out != "" {
		return out, nil
	}

	return "", fmt.Errorf("no log found for PID: %d", pid)
}

// TestConnection 测试 SSH 连接并返回远程主机基本信息。
// 被测服务器可能尚未保存到数据库，因此使用临时连接（不入池），测试完即优雅关闭。
func (rc *RemoteCollector) TestConnection(ctx context.Context, srv *Server) (*system.HostInfo, error) {
	conn, err := newSSHConn(srv)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	info := &system.HostInfo{}
	info.Hostname = rc.RunOne(ctx, conn, "hostname")
	info.OS = rc.RunOne(ctx, conn, "uname -s")
	info.Kernel = rc.RunOne(ctx, conn, "uname -r")
	info.Arch = rc.RunOne(ctx, conn, "uname -m")
	return info, nil
}

// ============================================================
// Docker 管理
// ============================================================

// DockerImage 表示 Docker 镜像信息。
type DockerImage struct {
	ID         string `json:"id"`
	Repository string `json:"repository"`
	Tag        string `json:"tag"`
	Size       string `json:"size"`
	Created    string `json:"created"`
}

// DockerContainer 表示 Docker 容器信息。
type DockerContainer struct {
	ID         string `json:"id"`
	Image      string `json:"image"`
	Command    string `json:"command"`
	State      string `json:"state"`
	Ports      string `json:"ports"`
	Names      string `json:"names"`
	RunningFor string `json:"running_for"`
}

// CheckDocker 检测服务器是否安装了 Docker。
func (rc *RemoteCollector) CheckDocker(ctx context.Context, srv *Server) (bool, string) {
	conn, err := rc.Acquire(srv)
	if err != nil {
		return false, ""
	}
	defer rc.Release(srv.ID)

	// bash -lc 加载登录 shell 的 PATH（兼容 snap 安装在 /snap/bin 等非标准路径），
	// 失败再由 RunWithSudo 回退 sudo -n 尝试
	out := rc.RunWithSudo(ctx, conn, "bash -lc 'docker --version' 2>/dev/null")
	if out == "" {
		return false, ""
	}
	return true, out
}

// ListImages 列出 Docker 镜像。
func (rc *RemoteCollector) ListImages(ctx context.Context, srv *Server) ([]DockerImage, error) {
	conn, err := rc.Acquire(srv)
	if err != nil {
		return nil, err
	}
	defer rc.Release(srv.ID)

	// 使用 docker images --format 获取结构化输出（需要 sudo）
	// 使用 @@ 作为分隔符避免 shell 解析问题
	cmd := "docker images --format '{{.ID}}@@{{.Repository}}@@{{.Tag}}@@{{.Size}}@@{{.CreatedAt}}' 2>/dev/null"
	out := rc.RunWithSudo(ctx, conn, cmd)
	if out == "" {
		return []DockerImage{}, nil
	}

	var images []DockerImage
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Split(line, "@@")
		if len(parts) >= 5 {
			images = append(images, DockerImage{
				ID:         parts[0],
				Repository: parts[1],
				Tag:        parts[2],
				Size:       parts[3],
				Created:    parts[4],
			})
		}
	}
	return images, nil
}

// ListContainers 列出 Docker 容器。
func (rc *RemoteCollector) ListContainers(ctx context.Context, srv *Server, all bool) ([]DockerContainer, error) {
	conn, err := rc.Acquire(srv)
	if err != nil {
		return nil, err
	}
	defer rc.Release(srv.ID)

	// 使用 docker ps 获取容器列表（需要 sudo）
	// 使用 @@ 作为分隔符避免 shell 解析问题
	psCmd := "docker ps --format '{{.ID}}@@{{.Image}}@@{{.Command}}@@{{.Status}}@@{{.Ports}}@@{{.Names}}@@{{.RunningFor}}' 2>/dev/null"
	if all {
		psCmd = strings.Replace(psCmd, "docker ps ", "docker ps -a ", 1)
	}
	out := rc.RunWithSudo(ctx, conn, psCmd)
	if out == "" {
		return []DockerContainer{}, nil
	}

	var containers []DockerContainer
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Split(line, "@@")
		if len(parts) >= 6 {
			state := parts[3]
			// 标准化状态
			stateLower := strings.ToLower(state)
			if strings.Contains(stateLower, "up") {
				state = "running"
			} else if strings.Contains(stateLower, "restarting") {
				state = "restarting"
			} else if strings.Contains(stateLower, "exited") {
				state = "exited"
			} else if strings.Contains(stateLower, "paused") {
				state = "paused"
			} else {
				state = stateLower
			}
			runningFor := ""
			if len(parts) >= 7 {
				runningFor = parts[6]
			}
			containers = append(containers, DockerContainer{
				ID:         parts[0],
				Image:      parts[1],
				Command:    strings.Trim(parts[2], "\""),
				State:      state,
				Ports:      parts[4],
				Names:      parts[5],
				RunningFor: runningFor,
			})
		}
	}
	return containers, nil
}

// PullImage 拉取 Docker 镜像。
func (rc *RemoteCollector) PullImage(ctx context.Context, srv *Server, image string) error {
	// 拉取镜像是长操作，套 30 分钟超时
	ctx, cancel := context.WithTimeout(ctx, LongOpTimeout)
	defer cancel()

	conn, err := rc.Acquire(srv)
	if err != nil {
		return err
	}
	defer rc.Release(srv.ID)

	cmd := fmt.Sprintf("docker pull %q 2>&1", image)
	out := rc.RunWithSudo(ctx, conn, cmd)
	rc.log.Infow("docker pull", "image", image, "output", out)
	return nil
}

// RemoveImage 删除 Docker 镜像。
func (rc *RemoteCollector) RemoveImage(ctx context.Context, srv *Server, id string) error {
	conn, err := rc.Acquire(srv)
	if err != nil {
		return err
	}
	defer rc.Release(srv.ID)

	cmd := fmt.Sprintf("docker rmi %q 2>&1", id)
	out := rc.RunWithSudo(ctx, conn, cmd)
	if strings.Contains(out, "No such image") {
		return fmt.Errorf("image not found: %s", id)
	}
	return nil
}

// DockerRunOptions 是基于已有镜像一键启动容器的配置项。
type DockerRunOptions struct {
	Name       string   `json:"name"`        // 容器名（可选）
	Ports      []string `json:"ports"`       // 端口映射，格式 宿主机端口:容器端口
	Volumes    []string `json:"volumes"`     // 卷挂载，格式 宿主机路径:容器路径
	Envs       []string `json:"envs"`        // 环境变量，格式 KEY=VALUE
	Network    string   `json:"network"`     // 网络模式（bridge/host/none 或自定义网络，可选）
	Restart    string   `json:"restart"`     // 重启策略（no/always/on-failure/unless-stopped，可选）
	Cmd        string   `json:"cmd"`         // 覆盖镜像默认启动命令（可选）
	Privileged bool     `json:"privileged"`  // 特权模式（--privileged）
	AutoRemove bool     `json:"auto_remove"` // 退出后自动删除（--rm）
	Tty        bool     `json:"tty"`         // 分配 TTY（-t）
}

// isContainerID 判断输出是否为 docker run -d 成功时打印的 64 位容器 ID。
func isContainerID(s string) bool {
	if len(s) != 64 {
		return false
	}
	for _, c := range s {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return false
		}
	}
	return true
}

// extractContainerID 从 docker run -d 的输出中提取容器 ID。
// 成功时最后一行必为 64 位容器 ID，但前面可能带 WARNING 等前导行；
// 失败时错误信息出现在 ID 之后（若创建了容器），最后一行不是 ID。
func extractContainerID(out string) string {
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) == 0 {
		return ""
	}
	last := strings.TrimSpace(lines[len(lines)-1])
	if isContainerID(last) {
		return last
	}
	return ""
}

// runSudoKeepOut 与 RunWithSudo 类似（先直接执行、失败回退 sudo -n），
// 但失败时保留输出 —— docker run 的报错信息在合并输出中，不能因非零退出码丢弃。
func (rc *RemoteCollector) runSudoKeepOut(ctx context.Context, client commandRunner, cmd string) string {
	out, _ := client.RunOnce(ctx, cmd)
	out = strings.TrimSpace(out)
	if extractContainerID(out) != "" {
		return out
	}
	// 直接执行失败（目标用户无 docker 权限等），回退 sudo -n
	sudoOut, _ := client.RunOnce(ctx, "sudo -n "+cmd)
	return strings.TrimSpace(sudoOut)
}

// RunContainer 基于已有镜像一键启动容器（docker run -d），返回容器 ID。
func (rc *RemoteCollector) RunContainer(ctx context.Context, srv *Server, image string, opts DockerRunOptions) (string, error) {
	image = strings.TrimSpace(image)
	if image == "" {
		return "", fmt.Errorf("image is required")
	}

	// 启动容器可能耗时较长，套 30 分钟超时
	ctx, cancel := context.WithTimeout(ctx, LongOpTimeout)
	defer cancel()

	conn, err := rc.Acquire(srv)
	if err != nil {
		return "", err
	}
	defer rc.Release(srv.ID)

	// 组装 docker run -d 命令；各参数经 %q 引用防止注入
	var b strings.Builder
	b.WriteString("docker run -d")
	if v := strings.TrimSpace(opts.Name); v != "" {
		b.WriteString(fmt.Sprintf(" --name %q", v))
	}
	for _, p := range opts.Ports {
		if p = strings.TrimSpace(p); p != "" {
			b.WriteString(fmt.Sprintf(" -p %q", p))
		}
	}
	for _, v := range opts.Volumes {
		if v = strings.TrimSpace(v); v != "" {
			b.WriteString(fmt.Sprintf(" -v %q", v))
		}
	}
	for _, e := range opts.Envs {
		if e = strings.TrimSpace(e); e != "" {
			b.WriteString(fmt.Sprintf(" -e %q", e))
		}
	}
	if n := strings.TrimSpace(opts.Network); n != "" && n != "bridge" {
		b.WriteString(fmt.Sprintf(" --network %q", n))
	}
	if r := strings.TrimSpace(opts.Restart); r != "" && r != "no" {
		b.WriteString(fmt.Sprintf(" --restart %q", r))
	}
	if opts.Privileged {
		b.WriteString(" --privileged")
	}
	if opts.AutoRemove {
		b.WriteString(" --rm")
	}
	if opts.Tty {
		b.WriteString(" -t")
	}
	b.WriteString(" " + image)
	if c := strings.TrimSpace(opts.Cmd); c != "" {
		b.WriteString(" " + c)
	}
	b.WriteString(" 2>&1")

	out := rc.runSudoKeepOut(ctx, conn, b.String())
	rc.log.Infow("docker run", "image", image, "name", opts.Name, "output", out)
	// 成功时输出最后一行为 64 位容器 ID，否则为错误信息
	if id := extractContainerID(out); id != "" {
		return id, nil
	}
	if out == "" {
		return "", fmt.Errorf("启动容器失败：远端无输出，请确认目标机 Docker 正常运行")
	}
	if len(out) > 300 {
		out = out[:300] + "..."
	}
	return "", fmt.Errorf("启动容器失败：%s", out)
}

// ContainerAction 执行容器操作（start/stop/restart/kill）。
func (rc *RemoteCollector) ContainerAction(ctx context.Context, srv *Server, id, action string) error {
	// 容器启停可能耗时较长，套 30 分钟超时
	ctx, cancel := context.WithTimeout(ctx, LongOpTimeout)
	defer cancel()

	conn, err := rc.Acquire(srv)
	if err != nil {
		return err
	}
	defer rc.Release(srv.ID)

	validActions := map[string]bool{"start": true, "stop": true, "restart": true, "kill": true}
	if !validActions[action] {
		return fmt.Errorf("invalid action: %s", action)
	}

	cmd := fmt.Sprintf("docker %s %q 2>&1", action, id)
	out := rc.RunWithSudo(ctx, conn, cmd)
	rc.log.Infow("docker container action", "action", action, "id", id, "output", out)
	return nil
}

// ExecInContainer 在容器内执行命令。
func (rc *RemoteCollector) ExecInContainer(ctx context.Context, srv *Server, id, cmd string) (string, error) {
	// 容器内命令可能长时间运行，套 30 分钟超时
	ctx, cancel := context.WithTimeout(ctx, LongOpTimeout)
	defer cancel()

	conn, err := rc.Acquire(srv)
	if err != nil {
		return "", err
	}
	defer rc.Release(srv.ID)

	fullCmd := fmt.Sprintf("docker exec %q sh -c %q 2>&1", id, cmd)
	out := rc.RunWithSudo(ctx, conn, fullCmd)
	return out, nil
}

// ContainerLogs 获取容器日志。
func (rc *RemoteCollector) ContainerLogs(ctx context.Context, srv *Server, id string, lines int) (string, error) {
	if lines <= 0 {
		lines = 100
	}
	conn, err := rc.Acquire(srv)
	if err != nil {
		return "", err
	}
	defer rc.Release(srv.ID)

	cmd := fmt.Sprintf("docker logs --tail %d %q 2>&1", lines, id)
	return rc.RunWithSudo(ctx, conn, cmd), nil
}

// RemoveContainer 删除容器。
func (rc *RemoteCollector) RemoveContainer(ctx context.Context, srv *Server, id string) error {
	conn, err := rc.Acquire(srv)
	if err != nil {
		return err
	}
	defer rc.Release(srv.ID)

	cmd := fmt.Sprintf("docker rm -f %q 2>&1", id)
	out := rc.RunWithSudo(ctx, conn, cmd)
	rc.log.Infow("docker rm", "id", id, "output", out)
	return nil
}

// MirrorSourceConfig 表示服务器上的镜像源配置。
type MirrorSourceConfig struct {
	Mirror   string `json:"mirror"`   // registry-mirrors 第一个地址
	Registry string `json:"registry"` // insecure-registries 第一个地址
	RawJSON  string `json:"raw_json"` // daemon.json 原始内容
}

// GetMirrorSources 获取服务器当前的 Docker 镜像源配置。
// 读取 /etc/docker/daemon.json 并解析 registry-mirrors 与 insecure-registries。
func (rc *RemoteCollector) GetMirrorSources(ctx context.Context, srv *Server) (*MirrorSourceConfig, error) {
	conn, err := rc.Acquire(srv)
	if err != nil {
		return nil, err
	}
	defer rc.Release(srv.ID)

	// 读取 daemon.json
	out := rc.RunWithSudo(ctx, conn, "cat /etc/docker/daemon.json 2>/dev/null || echo '{}'")

	config := &MirrorSourceConfig{}
	if out == "" {
		return config, nil
	}
	config.RawJSON = out

	if out == "{}" {
		return config, nil
	}

	// 解析 JSON 中的 registry-mirrors 和 insecure-registries 字段
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(out), &data); err == nil {
		if mirrors, ok := data["registry-mirrors"].([]interface{}); ok && len(mirrors) > 0 {
			if m, ok := mirrors[0].(string); ok {
				config.Mirror = m
			}
		}
		if registries, ok := data["insecure-registries"].([]interface{}); ok && len(registries) > 0 {
			if r, ok := registries[0].(string); ok {
				config.Registry = r
			}
		}
	}
	return config, nil
}

// ApplyMirrorSource 应用镜像源配置到服务器。
func (rc *RemoteCollector) ApplyMirrorSource(ctx context.Context, srv *Server, mirrorURL string) error {
	// 写配置后需重启 Docker，属于长操作，套 30 分钟超时
	ctx, cancel := context.WithTimeout(ctx, LongOpTimeout)
	defer cancel()

	conn, err := rc.Acquire(srv)
	if err != nil {
		return err
	}
	defer rc.Release(srv.ID)

	// 构建新的 daemon.json
	daemonJSON := fmt.Sprintf(`{
	"registry-mirrors": [%q]
}`, mirrorURL)

	// 写入配置文件并重启 Docker
	cmd := fmt.Sprintf(`sudo tee /etc/docker/daemon.json > /dev/null << 'EOF'
%s
EOF
sudo systemctl restart docker 2>/dev/null || sudo service docker restart 2>/dev/null`, daemonJSON)

	out := rc.RunWithSudo(ctx, conn, cmd)
	rc.log.Infow("apply mirror source", "mirror", mirrorURL, "output", out)
	return nil
}
