// Package remote 提供通过 SSH 在远程服务器上执行命令与采集信息的能力。
// 独立于 host/storage/software 包，避免循环依赖。
package remote

import (
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

	"github.com/smash-hq/server-management/internal/logger"
	"github.com/smash-hq/server-management/internal/system"
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
type sshConn struct {
	client        *ssh.Client
	mu            sync.Mutex
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

// RunOnce 执行单条命令（每次新建 Session）。
func (c *sshConn) RunOnce(cmd string) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	session, err := c.client.NewSession()
	if err != nil {
		return "", fmt.Errorf("new session: %w", err)
	}
	defer session.Close()

	out, err := session.CombinedOutput(cmd)
	if err != nil {
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

// usable 返回连接是否仍可使用（未被标记断开）。
func (c *sshConn) usable() bool { return !c.broken.Load() }

// Close 优雅关闭 SSH 连接：停止保活协程，并向对端发送 disconnect。
func (c *sshConn) Close() error {
	c.stopOnce.Do(func() { close(c.stopKeepalive) })
	return c.client.Close()
}

// Acquire 从连接池获取服务器的可用连接；池中不存在或已断线时自动重建。
func (rc *RemoteCollector) Acquire(srv *Server) (*sshConn, error) {
	return rc.pool.Acquire(srv)
}

// Release 归还连接到池（不关闭，由连接池统一管理生命周期）。
func (rc *RemoteCollector) Release(serverID string) {
	rc.pool.Release(serverID)
}

// CloseConn 优雅关闭指定服务器的池化连接（更新/删除服务器配置时调用）。
func (rc *RemoteCollector) CloseConn(serverID string) {
	rc.pool.CloseConn(serverID)
}

// RunOne 在远程服务器上执行单条命令并返回输出。所有命令均记录日志。
func (rc *RemoteCollector) RunOne(client *sshConn, cmd string) string {
	out, err := client.RunOnce(cmd)
	if err != nil {
		rc.log.Infow("remote cmd error", "cmd", cmd, "error", err.Error())
		return ""
	}
	// 截断过长的输出用于日志
	logOut := out
	if len(logOut) > 200 {
		logOut = logOut[:200] + "..."
	}
	rc.log.Infow("remote cmd", "cmd", cmd, "output", logOut)
	return out
}

// RunWithSudo 先尝试直接执行命令，失败则使用 sudo -n 回退。所有命令均记录日志。
func (rc *RemoteCollector) RunWithSudo(client *sshConn, cmd string) string {
	// 先尝试直接执行
	out := rc.RunOne(client, cmd)
	if out != "" {
		return out
	}
	// 失败则尝试 sudo -n（免密 sudo）
	sudoCmd := "sudo -n " + cmd
	rc.log.Infow("remote cmd sudo fallback", "original_cmd", cmd, "sudo_cmd", sudoCmd)
	out = rc.RunOne(client, sudoCmd)
	return out
}

// Snapshot 采集远程服务器的完整系统快照。
func (rc *RemoteCollector) Snapshot(srv *Server) (*system.Info, error) {
	conn, err := rc.Acquire(srv)
	if err != nil {
		return nil, err
	}
	defer rc.Release(srv.ID)

	info := &system.Info{
		Timestamp: time.Now().UnixMilli(),
	}

	// 主机信息
	info.Host.Hostname = rc.RunOne(conn, "hostname")
	info.Host.OS = rc.RunOne(conn, "uname -s")
	info.Host.Kernel = rc.RunOne(conn, "uname -r")
	info.Host.Arch = rc.RunOne(conn, "uname -m")
	info.Host.Platform = rc.RunOne(conn, "uname -v")
	// 系统语言环境：优先 LANG，回退 LC_ALL
	if lang := rc.RunOne(conn, "echo $LANG"); lang != "" {
		info.Host.Locale = lang
	} else if lc := rc.RunOne(conn, "echo $LC_ALL"); lc != "" {
		info.Host.Locale = lc
	}

	// CPU 信息
	info.CPU.Model = rc.RunOne(conn, "grep -m1 'model name' /proc/cpuinfo 2>/dev/null | cut -d: -f2")
	info.CPU.Model = strings.TrimSpace(info.CPU.Model)
	// 物理核心数
	if cores := rc.RunOne(conn, "grep -c ^processor /proc/cpuinfo 2>/dev/null"); cores != "" {
		fmt.Sscanf(cores, "%d", &info.CPU.Threads)
	}
	// 物理核心（cores）
	if physCores := rc.RunOne(conn, "grep 'cpu cores' /proc/cpuinfo 2>/dev/null | head -1 | awk '{print $4}'"); physCores != "" {
		fmt.Sscanf(physCores, "%d", &info.CPU.Cores)
	}
	// CPU 使用率
	if pct := rc.RunOne(conn, "top -bn1 2>/dev/null | grep 'Cpu(s)' | awk '{print $2}'"); pct != "" {
		fmt.Sscanf(pct, "%f", &info.CPU.Percent)
	}

	// 内存信息（使用 free 命令，无需 root）
	if mem := rc.RunOne(conn, "free -b 2>/dev/null | awk '/Mem:/{print $2, $3, $7}'"); mem != "" {
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
	if procs := rc.RunWithSudo(conn, "ps -e --no-headers 2>/dev/null | wc -l"); procs != "" {
		fmt.Sscanf(strings.TrimSpace(procs), "%d", &info.Processes)
	}

	// 磁盘信息（runWithSudo 自动回退到 sudo -n）
	if disks := rc.RunWithSudo(conn, "df -B1 --output=source,size,used,target -x tmpfs -x devtmpfs -x overlay 2>/dev/null | tail -n +2"); disks != "" {
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
	if uptime := rc.RunOne(conn, "cat /proc/uptime 2>/dev/null | awk '{print $1}'"); uptime != "" {
		var secs float64
		fmt.Sscanf(uptime, "%f", &secs)
		info.Uptime = uint64(secs)
	}

	// 系统负载（无需 root）
	if load := rc.RunOne(conn, "cat /proc/loadavg 2>/dev/null | awk '{print $1, $2, $3}'"); load != "" {
		fmt.Sscanf(load, "%f %f %f", &info.Load.Load1, &info.Load.Load5, &info.Load.Load15)
	}

	// 检测是否使用了 sudo（权限不足标记）
	// 如果进程数异常低，可能是权限不足
	if info.Processes > 0 && info.Processes < 10 {
		if procs := rc.RunOne(conn, "sudo -n ps -e --no-headers 2>/dev/null | wc -l"); procs != "" {
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
func (rc *RemoteCollector) GetLog(srv *Server, service string, lines int) (string, error) {
	if lines <= 0 {
		lines = 100
	}
	conn, err := rc.Acquire(srv)
	if err != nil {
		return "", err
	}
	defer rc.Release(srv.ID)

	// 优先尝试 journalctl（systemd 服务）
	cmd := fmt.Sprintf("journalctl -u %s --no-pager -n %d 2>/dev/null", service, lines)
	out := rc.RunOne(conn, cmd)
	if out != "" {
		return out, nil
	}

	// 回退：尝试 docker logs
	cmd = fmt.Sprintf("docker logs --tail %d %s 2>/dev/null", lines, service)
	out = rc.RunOne(conn, cmd)
	if out != "" {
		return out, nil
	}

	// 回退：尝试从 /var/log 读取
	cmd = fmt.Sprintf("tail -n %d /var/log/%s*.log 2>/dev/null || tail -n %d /var/log/%s/%s.log 2>/dev/null", lines, service, lines, service, service)
	out = rc.RunOne(conn, cmd)
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
func (rc *RemoteCollector) ListProcesses(srv *Server, limit int) ([]ProcessInfo, error) {
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
	out := rc.RunOne(conn, cmd)
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
func (rc *RemoteCollector) DetectLanguage(srv *Server, versionCmd string) string {
	conn, err := rc.Acquire(srv)
	if err != nil {
		return ""
	}
	defer rc.Release(srv.ID)
	return rc.RunOne(conn, versionCmd)
}

// InstallLanguage 执行语言环境安装命令。version 为空则不替换版本占位符。
func (rc *RemoteCollector) InstallLanguage(srv *Server, installCmd, version string) error {
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
	out := rc.RunWithSudo(conn, cmd)
	rc.log.Infow("install language", "cmd", cmd, "version", version, "output", out)
	return nil
}

// --- 文件管理 ---

// FileInfo 表示文件或目录的信息。
type FileInfo struct {
	Name     string `json:"name"`
	Path     string `json:"path"`
	IsDir    bool   `json:"is_dir"`
	Size     int64  `json:"size"`
	Mode     string `json:"mode"`
	ModTime  int64  `json:"mod_time"`
	IsHidden bool   `json:"is_hidden"`
}

// ListFilesResult 包含文件列表和 sudo 使用状态。
type ListFilesResult struct {
	Files    []FileInfo `json:"files"`
	SudoUsed bool       `json:"sudo_used"`
}

// ListFiles 列出指定目录的内容（类似 ls -la）。
// 先尝试 sudo ls，失败则回退到普通 ls，并标记 sudo 是否可用。
func (rc *RemoteCollector) ListFiles(srv *Server, path string) (*ListFilesResult, error) {
	conn, err := rc.Acquire(srv)
	if err != nil {
		return nil, err
	}
	defer rc.Release(srv.ID)

	if path == "" {
		path = "/"
	}

	sudoUsed := false
	// 先尝试 sudo ls -la
	cmd := fmt.Sprintf("ls -la --time-style=long-iso %q 2>/dev/null", path)
	out := rc.RunOne(conn, "sudo -n "+cmd)
	if out != "" {
		sudoUsed = true
	} else {
		// sudo 失败，回退到普通 ls
		out = rc.RunOne(conn, cmd)
	}
	if out == "" {
		return nil, fmt.Errorf("cannot list directory: %s", path)
	}

	var files []FileInfo
	lines := strings.Split(out, "\n")
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) < 9 {
			continue
		}
		// 跳过总计行和当前目录/父目录
		name := strings.Join(fields[8:], " ")
		if name == "" || name == "." || name == ".." || strings.HasPrefix(name, "total") {
			continue
		}

		mode := fields[0]
		isDir := strings.HasPrefix(mode, "d")
		isHidden := strings.HasPrefix(name, ".")

		var size int64
		fmt.Sscanf(fields[4], "%d", &size)

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
			Name:     name,
			Path:     fullPath,
			IsDir:    isDir,
			Size:     size,
			Mode:     mode,
			ModTime:  0, // ls 输出解析时间较复杂，暂不解析
			IsHidden: isHidden,
		})
	}

	return &ListFilesResult{Files: files, SudoUsed: sudoUsed}, nil
}

// GetFileInfo 获取单个文件的详细信息。
func (rc *RemoteCollector) GetFileInfo(srv *Server, path string) (*FileInfo, error) {
	conn, err := rc.Acquire(srv)
	if err != nil {
		return nil, err
	}
	defer rc.Release(srv.ID)

	cmd := fmt.Sprintf("stat -c '%%n|%%F|%%s|%%a|%%Y|%%U|%%G' %q 2>/dev/null", path)
	out := rc.RunWithSudo(conn, cmd)
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

	return &FileInfo{
		Name:    parts[0],
		Path:    path,
		IsDir:   isDir,
		Size:    size,
		Mode:    mode,
		ModTime: modTime,
	}, nil
}

// ReadFileContent 读取文件内容（用于预览）。
func (rc *RemoteCollector) ReadFileContent(srv *Server, path string, lines int) (string, error) {
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
	out := rc.RunOne(conn, cmd)
	var size int64
	fmt.Sscanf(out, "%d", &size)
	if size > 1<<20 { // > 1MB
		return "", fmt.Errorf("file too large (%d bytes), max 1MB", size)
	}

	cmd = fmt.Sprintf("cat %q 2>/dev/null | head -n %d", path, lines)
	return rc.RunWithSudo(conn, cmd), nil
}

// DownloadFile 读取文件内容用于下载。
func (rc *RemoteCollector) DownloadFile(srv *Server, path string) ([]byte, string, error) {
	conn, err := rc.Acquire(srv)
	if err != nil {
		return nil, "", err
	}
	defer rc.Release(srv.ID)

	cmd := fmt.Sprintf("cat %q 2>/dev/null", path)
	out := rc.RunWithSudo(conn, cmd)

	// 提取文件名
	parts := strings.Split(path, "/")
	filename := parts[len(parts)-1]

	return []byte(out), filename, nil
}

// KillProcess 终止指定 PID 的进程。
func (rc *RemoteCollector) KillProcess(srv *Server, pid int, force bool) error {
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
	out := rc.RunOne(conn, cmd)
	if out != "" && strings.Contains(out, "Operation not permitted") {
		return fmt.Errorf("permission denied: try using sudo or running as root")
	}
	return nil
}

// GetProcessLog 获取指定进程的日志（通过 journalctl 或 /proc/{pid}/fd）。
func (rc *RemoteCollector) GetProcessLog(srv *Server, pid int, lines int) (string, error) {
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
	out := rc.RunOne(conn, cmd)
	if out != "" {
		return out, nil
	}

	// 回退：尝试读取 /proc/{pid}/fd/1 和 fd/2（stdout/stderr）
	cmd = fmt.Sprintf("sudo -n tail -n %d /proc/%d/fd/1 2>/dev/null; sudo -n tail -n %d /proc/%d/fd/2 2>/dev/null", lines, pid, lines, pid)
	out = rc.RunOne(conn, cmd)
	if out != "" {
		return out, nil
	}

	return "", fmt.Errorf("no log found for PID: %d", pid)
}

// TestConnection 测试 SSH 连接并返回远程主机基本信息。
// 被测服务器可能尚未保存到数据库，因此使用临时连接（不入池），测试完即优雅关闭。
func (rc *RemoteCollector) TestConnection(srv *Server) (*system.HostInfo, error) {
	conn, err := newSSHConn(srv)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	info := &system.HostInfo{}
	info.Hostname = rc.RunOne(conn, "hostname")
	info.OS = rc.RunOne(conn, "uname -s")
	info.Kernel = rc.RunOne(conn, "uname -r")
	info.Arch = rc.RunOne(conn, "uname -m")
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
func (rc *RemoteCollector) CheckDocker(srv *Server) (bool, string) {
	conn, err := rc.Acquire(srv)
	if err != nil {
		return false, ""
	}
	defer rc.Release(srv.ID)

	// bash -lc 加载登录 shell 的 PATH（兼容 snap 安装在 /snap/bin 等非标准路径），
	// 失败再由 RunWithSudo 回退 sudo -n 尝试
	out := rc.RunWithSudo(conn, "bash -lc 'docker --version' 2>/dev/null")
	if out == "" {
		return false, ""
	}
	return true, out
}

// ListImages 列出 Docker 镜像。
func (rc *RemoteCollector) ListImages(srv *Server) ([]DockerImage, error) {
	conn, err := rc.Acquire(srv)
	if err != nil {
		return nil, err
	}
	defer rc.Release(srv.ID)

	// 使用 docker images --format 获取结构化输出（需要 sudo）
	// 使用 @@ 作为分隔符避免 shell 解析问题
	cmd := "docker images --format '{{.ID}}@@{{.Repository}}@@{{.Tag}}@@{{.Size}}@@{{.CreatedAt}}' 2>/dev/null"
	out := rc.RunWithSudo(conn, cmd)
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
func (rc *RemoteCollector) ListContainers(srv *Server, all bool) ([]DockerContainer, error) {
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
	out := rc.RunWithSudo(conn, psCmd)
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

// PullImage 拉取 Docker 镜像（异步）。
func (rc *RemoteCollector) PullImage(srv *Server, image string) error {
	conn, err := rc.Acquire(srv)
	if err != nil {
		return err
	}
	defer rc.Release(srv.ID)

	cmd := fmt.Sprintf("docker pull %q 2>&1", image)
	out := rc.RunWithSudo(conn, cmd)
	rc.log.Infow("docker pull", "image", image, "output", out)
	return nil
}

// RemoveImage 删除 Docker 镜像。
func (rc *RemoteCollector) RemoveImage(srv *Server, id string) error {
	conn, err := rc.Acquire(srv)
	if err != nil {
		return err
	}
	defer rc.Release(srv.ID)

	cmd := fmt.Sprintf("docker rmi %q 2>&1", id)
	out := rc.RunWithSudo(conn, cmd)
	if strings.Contains(out, "No such image") {
		return fmt.Errorf("image not found: %s", id)
	}
	return nil
}

// ContainerAction 执行容器操作（start/stop/restart/kill）。
func (rc *RemoteCollector) ContainerAction(srv *Server, id, action string) error {
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
	out := rc.RunWithSudo(conn, cmd)
	rc.log.Infow("docker container action", "action", action, "id", id, "output", out)
	return nil
}

// ExecInContainer 在容器内执行命令。
func (rc *RemoteCollector) ExecInContainer(srv *Server, id, cmd string) (string, error) {
	conn, err := rc.Acquire(srv)
	if err != nil {
		return "", err
	}
	defer rc.Release(srv.ID)

	fullCmd := fmt.Sprintf("docker exec %q sh -c %q 2>&1", id, cmd)
	out := rc.RunWithSudo(conn, fullCmd)
	return out, nil
}

// ContainerLogs 获取容器日志。
func (rc *RemoteCollector) ContainerLogs(srv *Server, id string, lines int) (string, error) {
	if lines <= 0 {
		lines = 100
	}
	conn, err := rc.Acquire(srv)
	if err != nil {
		return "", err
	}
	defer rc.Release(srv.ID)

	cmd := fmt.Sprintf("docker logs --tail %d %q 2>&1", lines, id)
	return rc.RunWithSudo(conn, cmd), nil
}

// RemoveContainer 删除容器。
func (rc *RemoteCollector) RemoveContainer(srv *Server, id string) error {
	conn, err := rc.Acquire(srv)
	if err != nil {
		return err
	}
	defer rc.Release(srv.ID)

	cmd := fmt.Sprintf("docker rm -f %q 2>&1", id)
	out := rc.RunWithSudo(conn, cmd)
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
func (rc *RemoteCollector) GetMirrorSources(srv *Server) (*MirrorSourceConfig, error) {
	conn, err := rc.Acquire(srv)
	if err != nil {
		return nil, err
	}
	defer rc.Release(srv.ID)

	// 读取 daemon.json
	out := rc.RunWithSudo(conn, "cat /etc/docker/daemon.json 2>/dev/null || echo '{}'")

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
func (rc *RemoteCollector) ApplyMirrorSource(srv *Server, mirrorURL string) error {
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

	out := rc.RunWithSudo(conn, cmd)
	rc.log.Infow("apply mirror source", "mirror", mirrorURL, "output", out)
	return nil
}
