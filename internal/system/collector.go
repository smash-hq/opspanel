// Package system 通过 gopsutil 采集主机硬件与运行时信息。
// gopsutil 支持 Linux x86_64 / arm64 / loong64，满足国产 CPU 平台需求。
package system

import (
	"runtime"
	"time"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/host"
	"github.com/shirou/gopsutil/v4/mem"
	"github.com/shirou/gopsutil/v4/net"
	"github.com/shirou/gopsutil/v4/process"
)

// Info 是单次采集得到的完整系统快照。
type Info struct {
	Timestamp   int64       `json:"timestamp"`    // Unix 毫秒时间戳
	Host        HostInfo    `json:"host"`         // 主机基本信息
	CPU         CPUInfo     `json:"cpu"`          // CPU 信息
	Memory      MemoryInfo  `json:"memory"`       // 内存信息
	Disks       []DiskInfo  `json:"disks"`        // 磁盘信息
	Network     NetworkInfo `json:"network"`      // 网络信息
	Load        LoadInfo    `json:"load"`         // 负载信息
	Processes   int         `json:"processes"`    // 进程数
	Uptime      uint64      `json:"uptime"`       // 运行秒数
	GoVersion   string      `json:"go_version"`   // Go 运行时版本
	Goroutines  int         `json:"goroutines"`   // 当前 goroutine 数
	SudoUsed    bool        `json:"sudo_used"`    // 采集过程使用了 sudo（权限不足标记）
}

// HostInfo 主机基本信息。
type HostInfo struct {
	Hostname string `json:"hostname"`
	OS       string `json:"os"`
	Platform string `json:"platform"`
	Kernel   string `json:"kernel"`
	Arch     string `json:"arch"`
}

// CPUInfo CPU 信息。
type CPUInfo struct {
	Model     string    `json:"model"`
	Cores     int       `json:"cores"`      // 物理核心
	Threads   int       `json:"threads"`    // 逻辑核心
	Percent   float64   `json:"percent"`    // 总使用率
	PerCore   []float64 `json:"per_core"`   // 每核使用率
}

// MemoryInfo 内存信息（字节）。
type MemoryInfo struct {
	Total     uint64  `json:"total"`
	Used      uint64  `json:"used"`
	Free      uint64  `json:"free"`
	Percent   float64 `json:"percent"`
	SwapTotal uint64  `json:"swap_total"`
	SwapUsed  uint64  `json:"swap_used"`
}

// DiskInfo 单个挂载点信息。
type DiskInfo struct {
	Device    string  `json:"device"`
	Mount     string  `json:"mount"`
	Total     uint64  `json:"total"`
	Used      uint64  `json:"used"`
	Percent   float64 `json:"percent"`
	FSType    string  `json:"fs_type"`
}

// NetworkInfo 网络信息。
type NetworkInfo struct {
	Interfaces []string       `json:"interfaces"`
	IOCounters []NetIOCounter `json:"io_counters"`
}

// NetIOCounter 单网卡 IO 统计。
type NetIOCounter struct {
	Name     string `json:"name"`
	BytesSent uint64 `json:"bytes_sent"`
	BytesRecv uint64 `json:"bytes_recv"`
}

// LoadInfo 系统负载。
type LoadInfo struct {
	Load1  float64 `json:"load1"`
	Load5  float64 `json:"load5"`
	Load15 float64 `json:"load15"`
}

// Collector 是系统信息采集器。
type Collector struct{}

// NewCollector 创建采集器。
func NewCollector() *Collector {
	return &Collector{}
}

// Snapshot 采集一次完整系统快照。
// 任何子项采集失败时返回零值而非中断整体流程。
func (c *Collector) Snapshot() (*Info, error) {
	info := &Info{
		Timestamp:  time.Now().UnixMilli(),
		GoVersion:  runtime.Version(),
		Goroutines: runtime.NumGoroutine(),
	}

	// 主机信息
	if h, err := host.Info(); err == nil {
		info.Host = HostInfo{
			Hostname: h.Hostname,
			OS:       h.OS,
			Platform: h.Platform,
			Kernel:   h.KernelVersion,
			Arch:     h.KernelArch,
		}
		info.Uptime = h.Uptime
	}

	// CPU
	info.CPU = c.collectCPU()

	// 内存
	info.Memory = c.collectMemory()

	// 磁盘
	info.Disks = c.collectDisks()

	// 网络
	info.Network = c.collectNetwork()

	// 进程数
	if procs, err := process.Pids(); err == nil {
		info.Processes = len(procs)
	}

	return info, nil
}

func (c *Collector) collectCPU() CPUInfo {
	ci := CPUInfo{}

	// CPU 使用率（1 秒采样间隔，非阻塞取上次值）
	if pct, err := cpu.Percent(0, false); err == nil && len(pct) > 0 {
		ci.Percent = pct[0]
	}
	if pct, err := cpu.Percent(0, true); err == nil {
		ci.PerCore = pct
		ci.Threads = len(pct)
	}

	// CPU 详细信息
	if infos, err := cpu.Info(); err == nil && len(infos) > 0 {
		ci.Model = infos[0].ModelName
		ci.Cores = int(infos[0].Cores)
	}

	return ci
}

func (c *Collector) collectMemory() MemoryInfo {
	mi := MemoryInfo{}

	if v, err := mem.VirtualMemory(); err == nil {
		mi.Total = v.Total
		mi.Used = v.Used
		mi.Free = v.Free
		mi.Percent = v.UsedPercent
	}

	if s, err := mem.SwapMemory(); err == nil {
		mi.SwapTotal = s.Total
		mi.SwapUsed = s.Used
	}

	return mi
}

func (c *Collector) collectDisks() []DiskInfo {
	var disks []DiskInfo

	parts, err := disk.Partitions(false)
	if err != nil {
		return disks
	}

	for _, p := range parts {
		// 跳过虚拟文件系统
		if isVirtualFS(p.Fstype) {
			continue
		}
		if usage, err := disk.Usage(p.Mountpoint); err == nil {
			disks = append(disks, DiskInfo{
				Device:  p.Device,
				Mount:   p.Mountpoint,
				Total:   usage.Total,
				Used:    usage.Used,
				Percent: usage.UsedPercent,
				FSType:  p.Fstype,
			})
		}
	}
	return disks
}

func (c *Collector) collectNetwork() NetworkInfo {
	ni := NetworkInfo{}

	// 网卡列表
	if ifaces, err := net.Interfaces(); err == nil {
		for _, iface := range ifaces {
			ni.Interfaces = append(ni.Interfaces, iface.Name)
		}
	}

	// IO 统计
	if counters, err := net.IOCounters(true); err == nil {
		for _, c := range counters {
			ni.IOCounters = append(ni.IOCounters, NetIOCounter{
				Name:      c.Name,
				BytesSent: c.BytesSent,
				BytesRecv: c.BytesRecv,
			})
		}
	}

	return ni
}

// isVirtualFS 判断是否为虚拟/伪文件系统，避免采集 tmpfs、devtmpfs 等。
func isVirtualFS(fs string) bool {
	virtual := []string{"tmpfs", "devtmpfs", "proc", "sysfs", "cgroup", "cgroup2"}
	for _, v := range virtual {
		if fs == v {
			return true
		}
	}
	return false
}

