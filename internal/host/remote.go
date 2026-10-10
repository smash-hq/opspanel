package host

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

// RemoteCollector 通过 SSH 在远程 Linux 服务器上采集系统信息。
type RemoteCollector struct {
	log *logger.Logger
}

// NewRemoteCollector 创建远程采集器。
func NewRemoteCollector(log *logger.Logger) *RemoteCollector {
	return &RemoteCollector{log: log.Named("remote")}
}

// TestConnection 测试 SSH 连接并返回远程主机基本信息。
// 连接失败时返回错误，成功时返回主机名、OS、内核、架构。
func (rc *RemoteCollector) TestConnection(srv *Server) (*system.HostInfo, error) {
	client, err := rc.dial(srv)
	if err != nil {
		return nil, fmt.Errorf("ssh 连接失败: %w", err)
	}
	defer client.Close()

	hostInfo := rc.collectHost(client)
	return &hostInfo, nil
}

// Snapshot 连接远程服务器并采集系统快照。
func (rc *RemoteCollector) Snapshot(srv *Server) (*system.Info, error) {
	client, err := rc.dial(srv)
	if err != nil {
		return nil, fmt.Errorf("ssh dial %s: %w", srv.Host, err)
	}
	defer client.Close()

	// 跟踪各采集项是否使用了 sudo
	sudoFlags := make(map[string]bool)

	info := &system.Info{
		Timestamp: time.Now().UnixMilli(),
		GoVersion: "remote",
	}

	// 依次执行采集命令，单项失败不影响整体
	info.Host = rc.collectHost(client)
	info.CPU = rc.collectCPU(client, sudoFlags)
	info.Memory = rc.collectMemory(client, sudoFlags)
	info.Disks = rc.collectDisks(client, sudoFlags)
	info.Network = rc.collectNetwork(client, sudoFlags)
	info.Uptime = rc.collectUptime(client, sudoFlags)

	// 如果采集过程中使用了 sudo，标记权限警告
	if rc.usedSudo(sudoFlags) {
		info.SudoUsed = true
	}

	return info, nil
}

// dial 建立 SSH 连接。
func (rc *RemoteCollector) dial(srv *Server) (*ssh.Client, error) {
	config := &ssh.ClientConfig{
		User:            srv.Username,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // TODO: 生产环境应验证主机密钥
		Timeout:         10 * time.Second,
	}

	switch srv.AuthType {
	case "key":
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
		// 同时支持多种认证方式，keyboard-interactive 优先于 password。
		// 部分服务器（如启用 PAM/ChallengeResponse 的 Linux）只接受 keyboard-interactive。
		config.Auth = []ssh.AuthMethod{
			ssh.KeyboardInteractive(func(user, instruction string, questions []string, echos []bool) ([]string, error) {
				answers := make([]string, len(questions))
				for i := range questions {
					answers[i] = srv.Password
				}
				return answers, nil
			}),
			ssh.Password(srv.Password),
		}
	}

	addr := net.JoinHostPort(srv.Host, strconv.Itoa(srv.Port))
	return ssh.Dial("tcp", addr, config)
}

// Dial 建立 SSH 连接（导出供其他包使用）。
func (rc *RemoteCollector) Dial(srv *Server) (*ssh.Client, error) {
	return rc.dial(srv)
}

// RunOne 在远程执行单条命令并返回标准输出（导出供其他包使用）。
// 自动尝试 sudo -n 回退。
func (rc *RemoteCollector) RunOne(client *ssh.Client, cmd string) string {
	out, _, _ := rc.run(client, cmd)
	return out
}

// run 在远程执行单条命令并返回标准输出。
// 命令失败时自动尝试 sudo -n 回退（非交互式 sudo）。
// 返回值: (输出, 是否使用了 sudo, 最终是否成功)。
func (rc *RemoteCollector) run(client *ssh.Client, cmd string) (string, bool, bool) {
	out := rc.exec(client, cmd)
	if out != "" {
		return out, false, true
	}

	// 普通命令失败，尝试 sudo -n（非交互式，需要 NOPASSWD 配置）
	sudoCmd := "sudo -n " + cmd
	sudoOut := rc.exec(client, sudoCmd)
	if sudoOut != "" {
		rc.log.Info("remote cmd succeeded with sudo", "cmd", cmd)
		return sudoOut, true, true
	}

	return "", false, false
}

// exec 在远程执行单条命令并返回标准输出（原始执行）。
func (rc *RemoteCollector) exec(client *ssh.Client, cmd string) string {
	session, err := client.NewSession()
	if err != nil {
		return ""
	}
	defer session.Close()

	out, err := session.CombinedOutput(cmd)
	if err != nil {
		rc.log.Warn("remote cmd failed", "cmd", cmd, "err", err)
		return ""
	}
	return string(out)
}

// 采集过程中是否使用了 sudo（需要 root 权限的命令）。
func (rc *RemoteCollector) usedSudo(flags map[string]bool) bool {
	for _, v := range flags {
		if v {
			return true
		}
	}
	return false
}

// collectHost 采集主机基本信息。
func (rc *RemoteCollector) collectHost(client *ssh.Client) system.HostInfo {
	var h system.HostInfo

	// hostname
	h.Hostname = strings.TrimSpace(rc.runGet(client, "hostname"))

	// os-release
	if osRelease := rc.runGet(client, "cat /etc/os-release 2>/dev/null | head -5"); osRelease != "" {
		for _, line := range strings.Split(osRelease, "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "PRETTY_NAME=") {
				h.OS = strings.Trim(strings.TrimPrefix(line, "PRETTY_NAME="), "\"")
				break
			}
		}
	}

	// uname
	if uname := rc.runGet(client, "uname -srm"); uname != "" {
		parts := strings.Fields(uname)
		if len(parts) >= 3 {
			h.Platform = parts[0]
			h.Kernel = parts[1]
			h.Arch = parts[2]
		}
	}

	return h
}

// runGet 是 run 的便捷包装，只返回输出字符串。
func (rc *RemoteCollector) runGet(client *ssh.Client, cmd string) string {
	out, _, _ := rc.run(client, cmd)
	return out
}

// collectCPU 采集 CPU 信息。
// 优先使用 lscpu，失败时回退到 /proc/cpuinfo（兼容国产 CPU 平台）。
func (rc *RemoteCollector) collectCPU(client *ssh.Client, sudoFlags map[string]bool) system.CPUInfo {
	var ci system.CPUInfo

	// 尝试 lscpu
	if cpuinfo, sudoUsed, _ := rc.run(client, "lscpu 2>/dev/null | grep -E 'Model name|Core\\(s\\)|Thread\\(s\\)'"); cpuinfo != "" {
		if sudoUsed {
			sudoFlags["cpu"] = true
		}
		for _, line := range strings.Split(cpuinfo, "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "Model name:") {
				ci.Model = strings.TrimSpace(strings.TrimPrefix(line, "Model name:"))
			} else if strings.HasPrefix(line, "Core(s) per socket:") {
				cores, _ := strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(line, "Core(s) per socket:")))
				ci.Cores = cores
			} else if strings.HasPrefix(line, "Thread(s) per core:") {
				threads, _ := strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(line, "Thread(s) per core:")))
				ci.Threads = threads
				if ci.Cores > 0 {
					ci.Threads = ci.Cores * threads
				}
			}
		}
	}

	// lscpu 无结果时，回退到 /proc/cpuinfo
	if ci.Cores == 0 || ci.Model == "" {
		if procCpuinfo := rc.runGet(client, "cat /proc/cpuinfo 2>/dev/null"); procCpuinfo != "" {
			var physicalIDs = make(map[string]bool)
			var coreIDs = make(map[string]bool)
			var processors = 0
			for _, line := range strings.Split(procCpuinfo, "\n") {
				line = strings.TrimSpace(line)
				if strings.HasPrefix(line, "model name") && ci.Model == "" {
					ci.Model = strings.TrimSpace(strings.SplitN(line, ":", 2)[1])
				} else if strings.HasPrefix(line, "physical id") {
					id := strings.TrimSpace(strings.SplitN(line, ":", 2)[1])
					physicalIDs[id] = true
				} else if strings.HasPrefix(line, "core id") {
					id := strings.TrimSpace(strings.SplitN(line, ":", 2)[1])
					coreIDs[id] = true
				} else if strings.HasPrefix(line, "processor") {
					processors++
				}
			}
			if ci.Cores == 0 {
				if len(coreIDs) > 0 {
					ci.Cores = len(coreIDs)
				} else if len(physicalIDs) > 0 {
					ci.Cores = len(physicalIDs)
				}
			}
			if ci.Threads == 0 {
				ci.Threads = processors
			}
			// 如果 cores 有值但 threads 没有正确计算
			if ci.Cores > 0 && ci.Threads == 0 {
				ci.Threads = ci.Cores
			}
		}
	}

	// CPU 使用率：优先 top，回退到 /proc/stat
	if top, sudoUsed, _ := rc.run(client, "top -bn1 2>/dev/null | grep 'Cpu(s)' | head -1"); top != "" {
		if sudoUsed {
			sudoFlags["cpu"] = true
		}
		if idx := strings.Index(top, "id"); idx > 0 {
			segment := top[:idx]
			fields := strings.Fields(segment)
			if len(fields) > 0 {
				idleStr := strings.TrimRight(fields[len(fields)-1], ",")
				if idle, err := strconv.ParseFloat(idleStr, 64); err == nil {
					ci.Percent = 100.0 - idle
				}
			}
		}
	} else if stat := rc.runGet(client, "head -1 /proc/stat 2>/dev/null"); stat != "" {
		// 通过两次采样计算 CPU 使用率
		fields := strings.Fields(stat)
		if len(fields) >= 5 && fields[0] == "cpu" {
			user, _ := strconv.ParseUint(fields[1], 10, 64)
			systemVal, _ := strconv.ParseUint(fields[3], 10, 64)
			idle, _ := strconv.ParseUint(fields[4], 10, 64)
			total := user + systemVal + idle
			if total > 0 {
				ci.Percent = float64(total-idle) / float64(total) * 100
			}
		}
	}

	return ci
}

// collectMemory 采集内存信息。
func (rc *RemoteCollector) collectMemory(client *ssh.Client, sudoFlags map[string]bool) system.MemoryInfo {
	mi := system.MemoryInfo{}

	if free, sudoUsed, _ := rc.run(client, "free -b 2>/dev/null"); free != "" {
		if sudoUsed {
			sudoFlags["memory"] = true
		}
		for _, line := range strings.Split(free, "\n") {
			fields := strings.Fields(line)
			if len(fields) < 4 {
				continue
			}
			switch fields[0] {
			case "Mem:":
				mi.Total, _ = strconv.ParseUint(fields[1], 10, 64)
				mi.Used, _ = strconv.ParseUint(fields[2], 10, 64)
				mi.Free, _ = strconv.ParseUint(fields[3], 10, 64)
				if mi.Total > 0 {
					mi.Percent = float64(mi.Used) / float64(mi.Total) * 100
				}
			case "Swap:":
				mi.SwapTotal, _ = strconv.ParseUint(fields[1], 10, 64)
				mi.SwapUsed, _ = strconv.ParseUint(fields[2], 10, 64)
			}
		}
	}

	return mi
}

// collectDisks 采集磁盘信息。
func (rc *RemoteCollector) collectDisks(client *ssh.Client, sudoFlags map[string]bool) []system.DiskInfo {
	var disks []system.DiskInfo

	// df -B1 输出: Filesystem 1K-blocks Used Available Use% Mounted
	output, sudoUsed, _ := rc.run(client, "df -B1 --output=source,target,fstype,size,used,avail 2>/dev/null | tail -n +2")
	if sudoUsed {
		sudoFlags["disks"] = true
	}
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 6 {
			continue
		}
		device := fields[0]
		mount := fields[1]
		fstype := fields[2]

		// 跳过伪文件系统
		if isPseudoFS(fstype) {
			continue
		}

		total, _ := strconv.ParseUint(fields[3], 10, 64)
		used, _ := strconv.ParseUint(fields[4], 10, 64)

		disks = append(disks, system.DiskInfo{
			Device:  device,
			Mount:   mount,
			FSType:  fstype,
			Total:   total,
			Used:    used,
			Percent: float64(used) / float64(total) * 100,
		})
	}

	return disks
}

// isPseudoFS 判断是否为伪文件系统。
func isPseudoFS(fs string) bool {
	for _, v := range []string{"tmpfs", "devtmpfs", "proc", "sysfs", "cgroup", "cgroup2", "autofs", "devfs"} {
		if fs == v {
			return true
		}
	}
	return false
}

// collectNetwork 采集网络信息。
func (rc *RemoteCollector) collectNetwork(client *ssh.Client, sudoFlags map[string]bool) system.NetworkInfo {
	ni := system.NetworkInfo{}

	// /proc/net/dev 格式
	output, sudoUsed, _ := rc.run(client, "cat /proc/net/dev")
	if sudoUsed {
		sudoFlags["network"] = true
	}
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if !strings.Contains(line, ":") {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		name := strings.TrimSpace(parts[0])
		if name == "lo" {
			continue
		}
		fields := strings.Fields(parts[1])
		if len(fields) < 9 {
			continue
		}

		ni.Interfaces = append(ni.Interfaces, name)
		recv, _ := strconv.ParseUint(fields[0], 10, 64)
		sent, _ := strconv.ParseUint(fields[8], 10, 64)
		ni.IOCounters = append(ni.IOCounters, system.NetIOCounter{
			Name:      name,
			BytesSent: sent,
			BytesRecv: recv,
		})
	}

	return ni
}

// collectUptime 获取系统运行时间。
func (rc *RemoteCollector) collectUptime(client *ssh.Client, sudoFlags map[string]bool) uint64 {
	output, sudoUsed, _ := rc.run(client, "cat /proc/uptime")
	if sudoUsed {
		sudoFlags["uptime"] = true
	}
	fields := strings.Fields(output)
	if len(fields) > 0 {
		if secs, err := strconv.ParseFloat(fields[0], 64); err == nil {
			return uint64(secs)
		}
	}
	return 0
}
