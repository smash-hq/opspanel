// Package remote 提供通过 SSH 在远程服务器上执行命令与采集信息的能力。
// 独立于 host/storage/software 包，避免循环依赖。
package remote

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
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
	log *logger.Logger
}

// NewRemoteCollector 创建远程采集器。
func NewRemoteCollector(log *logger.Logger) *RemoteCollector {
	return &RemoteCollector{log: log.Named("remote")}
}

// sshConn 封装 *ssh.Client，每次执行命令新建 Session。
type sshConn struct {
	client *ssh.Client
	mu     sync.Mutex
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

	return &sshConn{client: client}, nil
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
		return string(out), err
	}
	return strings.TrimSpace(string(out)), nil
}

// Close 关闭 SSH 连接。
func (c *sshConn) Close() error {
	return c.client.Close()
}

// Dial 建立 SSH 连接（返回可执行多次命令的连接）。
func (rc *RemoteCollector) Dial(srv *Server) (*sshConn, error) {
	return newSSHConn(srv)
}

// RunOne 在远程服务器上执行单条命令并返回输出。
func (rc *RemoteCollector) RunOne(client *sshConn, cmd string) string {
	out, err := client.RunOnce(cmd)
	if err != nil {
		rc.log.Debugw("remote cmd failed", "cmd", cmd, "error", err)
		return ""
	}
	return out
}

// Snapshot 采集远程服务器的完整系统快照。
func (rc *RemoteCollector) Snapshot(srv *Server) (*system.Info, error) {
	conn, err := rc.Dial(srv)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	info := &system.Info{
		Timestamp: time.Now().UnixMilli(),
	}

	// 主机信息
	info.Host.Hostname = rc.RunOne(conn, "hostname")
	info.Host.OS = rc.RunOne(conn, "uname -s")
	info.Host.Kernel = rc.RunOne(conn, "uname -r")
	info.Host.Arch = rc.RunOne(conn, "uname -m")
	info.Host.Platform = rc.RunOne(conn, "uname -v")

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
	if procs := rc.RunOne(conn, "ps -e --no-headers 2>/dev/null | wc -l"); procs != "" {
		fmt.Sscanf(strings.TrimSpace(procs), "%d", &info.Processes)
	}

	// 磁盘信息
	sudoNeeded := false
	if disks := rc.RunOne(conn, "df -B1 --output=source,size,used,target -x tmpfs -x devtmpfs -x overlay 2>/dev/null | tail -n +2"); disks != "" {
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
	// 检查是否有权限获取完整磁盘信息（部分挂载点可能需要 root）
	if len(info.Disks) == 0 {
		sudoNeeded = true
		// 尝试用 sudo 获取
		if disks := rc.RunOne(conn, "sudo -n df -B1 --output=source,size,used,target -x tmpfs -x devtmpfs -x overlay 2>/dev/null | tail -n +2"); disks != "" {
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
	if sudoNeeded {
		info.SudoUsed = true
	}
	// 额外检查：如果进程数异常低，也可能是权限不足
	if info.Processes > 0 && info.Processes < 10 {
		// 尝试用 sudo 获取更准确的进程数
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
	conn, err := rc.Dial(srv)
	if err != nil {
		return "", err
	}
	defer conn.Close()

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
    PID      int    `json:"pid"`
    User     string `json:"user"`
    CPU      string `json:"cpu"`
    Mem      string `json:"mem"`
    Command  string `json:"command"`
}

// ListProcesses 获取远程服务器进程列表（类似 top/ps）。
func (rc *RemoteCollector) ListProcesses(srv *Server, limit int) ([]ProcessInfo, error) {
    if limit <= 0 {
        limit = 50
    }
    conn, err := rc.Dial(srv)
    if err != nil {
        return nil, err
    }
    defer conn.Close()

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
	conn, err := rc.Dial(srv)
	if err != nil {
		return ""
	}
	defer conn.Close()
	return rc.RunOne(conn, versionCmd)
}

// InstallLanguage 执行语言环境安装命令。
func (rc *RemoteCollector) InstallLanguage(srv *Server, installCmd string) error {
	conn, err := rc.Dial(srv)
	if err != nil {
		return err
	}
	defer conn.Close()

	// 执行安装命令
	out := rc.RunOne(conn, installCmd)
	rc.log.Infow("install language", "cmd", installCmd, "output", out)
	return nil
}

// KillProcess 终止指定 PID 的进程。
func (rc *RemoteCollector) KillProcess(srv *Server, pid int, force bool) error {
    conn, err := rc.Dial(srv)
    if err != nil {
        return err
    }
    defer conn.Close()

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
    conn, err := rc.Dial(srv)
    if err != nil {
        return "", err
    }
    defer conn.Close()

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
func (rc *RemoteCollector) TestConnection(srv *Server) (*system.HostInfo, error) {
	conn, err := rc.Dial(srv)
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
