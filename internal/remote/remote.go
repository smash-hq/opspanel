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

	// CPU 信息
	info.CPU.Model = rc.RunOne(conn, "grep -m1 'model name' /proc/cpuinfo | cut -d: -f2")
	info.CPU.Model = strings.TrimSpace(info.CPU.Model)
	if cores := rc.RunOne(conn, "nproc"); cores != "" {
		fmt.Sscanf(cores, "%d", &info.CPU.Threads)
	}
	// CPU 使用率：使用 top 非阻塞模式
	if pct := rc.RunOne(conn, "top -bn1 2>/dev/null | grep 'Cpu(s)' | awk '{print $2}'"); pct != "" {
		fmt.Sscanf(pct, "%f", &info.CPU.Percent)
	}

	// 内存信息（使用 free 命令）
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

	// 磁盘信息
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

	// 运行时间
	if uptime := rc.RunOne(conn, "cat /proc/uptime | awk '{print $1}'"); uptime != "" {
		var secs float64
		fmt.Sscanf(uptime, "%f", &secs)
		info.Uptime = uint64(secs)
	}

	// 系统负载
	if load := rc.RunOne(conn, "cat /proc/loadavg | awk '{print $1, $2, $3}'"); load != "" {
		fmt.Sscanf(load, "%f %f %f", &info.Load.Load1, &info.Load.Load5, &info.Load.Load15)
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
