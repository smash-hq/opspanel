// Package remote 提供通过 SSH 在远程服务器上执行命令与采集信息的能力。
// 独立于 host/storage/software 包，避免循环依赖。
package remote

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/smash-hq/server-management/internal/logger"
	"github.com/smash-hq/server-management/internal/system"
	"golang.org/x/crypto/ssh"
)

// 确保 system 包被使用
var _ system.Info

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

// SSHClient 是 SSH 会话接口。
type SSHClient interface {
	Close() error
	Run(cmd string) (string, error)
}

type sshClient struct {
	session *ssh.Session
}

func (c *sshClient) Close() error {
	return c.session.Close()
}

func (c *sshClient) Run(cmd string) (string, error) {
	out, err := c.session.CombinedOutput(cmd)
	return string(out), err
}

// Dial 建立 SSH 连接。
func (rc *RemoteCollector) Dial(srv *Server) (SSHClient, error) {
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
	conn, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		return nil, fmt.Errorf("ssh dial %s: %w", addr, err)
	}

	session, err := conn.NewSession()
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("ssh new session: %w", err)
	}

	return &sshClient{session: session}, nil
}

// RunOne 在远程服务器上执行单条命令并返回输出。
func (rc *RemoteCollector) RunOne(client SSHClient, cmd string) string {
	out, err := client.Run(cmd)
	if err != nil {
		rc.log.Debugw("remote cmd failed", "cmd", cmd, "error", err)
		return ""
	}
	return strings.TrimSpace(out)
}

// Snapshot 采集远程服务器的完整系统快照。
func (rc *RemoteCollector) Snapshot(srv *Server) (*system.Info, error) {
	client, err := rc.Dial(srv)
	if err != nil {
		return nil, err
	}
	defer client.Close()

	info := &system.Info{
		Timestamp: time.Now().UnixMilli(),
	}

	// 主机信息
	if h := rc.RunOne(client, "hostname"); h != "" {
		info.Host.Hostname = h
	}
	if o := rc.RunOne(client, "uname -s"); o != "" {
		info.Host.OS = o
	}
	if k := rc.RunOne(client, "uname -r"); k != "" {
		info.Host.Kernel = k
	}
	if a := rc.RunOne(client, "uname -m"); a != "" {
		info.Host.Arch = a
	}

	// CPU 信息
	if model := rc.RunOne(client, "grep -m1 'model name' /proc/cpuinfo | cut -d: -f2"); model != "" {
		info.CPU.Model = strings.TrimSpace(model)
	}
	if cores := rc.RunOne(client, "nproc"); cores != "" {
		fmt.Sscanf(cores, "%d", &info.CPU.Threads)
	}
	if pct := rc.RunOne(client, "top -bn1 | grep 'Cpu(s)' | awk '{print $2}'"); pct != "" {
		fmt.Sscanf(pct, "%f", &info.CPU.Percent)
	}

	// 内存信息
	if mem := rc.RunOne(client, "free -b | awk '/Mem:/{print $2, $3, $4, $7}'"); mem != "" {
		var total, used, free, available uint64
		fmt.Sscanf(mem, "%d %d %d %d", &total, &used, &free, &available)
		info.Memory.Total = total
		info.Memory.Used = used
		info.Memory.Free = free
		if total > 0 {
			info.Memory.Percent = float64(used) / float64(total) * 100
		}
	}

	// 磁盘信息
	if disks := rc.RunOne(client, "df -B1 --output=source,size,used,target -x tmpfs -x devtmpfs -x overlay 2>/dev/null | tail -n +2"); disks != "" {
		for _, line := range strings.Split(disks, "\n") {
			fields := strings.Fields(line)
			if len(fields) >= 4 {
				var total, used uint64
				fmt.Sscanf(fields[1], "%d", &total)
				fmt.Sscanf(fields[2], "%d", &used)
				info.Disks = append(info.Disks, system.DiskInfo{
					Device:  fields[0],
					Total:   total,
					Used:    used,
					Mount:   fields[3],
					Percent: float64(used) / float64(total) * 100,
				})
			}
		}
	}

	// 运行时间
	if uptime := rc.RunOne(client, "cat /proc/uptime | awk '{print $1}'"); uptime != "" {
		var secs float64
		fmt.Sscanf(uptime, "%f", &secs)
		info.Uptime = uint64(secs)
	}

	// 负载
	if load := rc.RunOne(client, "cat /proc/loadavg | awk '{print $1, $2, $3}'"); load != "" {
		fmt.Sscanf(load, "%f %f %f", &info.Load.Load1, &info.Load.Load5, &info.Load.Load15)
	}

	return info, nil
}

// TestConnection 测试 SSH 连接并返回远程主机基本信息。
func (rc *RemoteCollector) TestConnection(srv *Server) (*system.HostInfo, error) {
	client, err := rc.Dial(srv)
	if err != nil {
		return nil, err
	}
	defer client.Close()

	info := &system.HostInfo{}
	if h := rc.RunOne(client, "hostname"); h != "" {
		info.Hostname = h
	}
	if o := rc.RunOne(client, "uname -s"); o != "" {
		info.OS = o
	}
	if k := rc.RunOne(client, "uname -r"); k != "" {
		info.Kernel = k
	}
	if a := rc.RunOne(client, "uname -m"); a != "" {
		info.Arch = a
	}
	return info, nil
}
