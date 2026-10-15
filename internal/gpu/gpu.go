// Package gpu 采集国产化 GPU/NPU 设备信息。
// 无 agent：经 SSH（或本机进程内执行）实时按需拉取厂商命令行工具输出并解析。
package gpu

import (
	"context"
	"fmt"
	"strings"
)

// 厂商标识。
const (
	VendorAscend    = "ascend"    // 华为昇腾（npu-smi）
	VendorCambricon = "cambricon" // 寒武纪（cnmon）
)

// Device 单卡信息。数值字段统一用 string：解析不到即置空，不做转换失败报错。
type Device struct {
	Index      string `json:"index"`
	Name       string `json:"name"`
	Health     string `json:"health"`
	PowerW     string `json:"power_w"`
	TempC      string `json:"temp_c"`
	CoreUtil   string `json:"core_util"`
	MemUsedMB  string `json:"mem_used_mb"`
	MemTotalMB string `json:"mem_total_mb"`
}

// Snapshot 一次采集的完整结果。
type Snapshot struct {
	Vendor     string   `json:"vendor"` // ascend / cambricon / ""（未发现设备或工具）
	VendorName string   `json:"vendor_name"`
	Devices    []Device `json:"devices"`
	Error      string   `json:"error,omitempty"` // 非致命错误（工具存在但查询/解析失败）
}

// Runner 抽象命令执行通道（远程为 SSH 连接，本机为进程内执行器），便于单测注入。
type Runner interface {
	Run(ctx context.Context, cmd string) (string, error)
}

// detectCmd 一条命令同时探测两类工具（输出可执行文件完整路径，不存在则为空）。
// 除 PATH 外兜底标准安装路径：SSH 非交互 shell 不加载 /etc/profile.d，
// 昇腾 npu-smi（/usr/local/Ascend/driver/tools）与寒武纪 cnmon（/usr/local/neuware/bin）
// 常常只在交互登录 shell 的 PATH 里。
const detectCmd = `p=$(command -v npu-smi 2>/dev/null); [ -z "$p" ] && [ -x /usr/local/Ascend/driver/tools/npu-smi ] && p=/usr/local/Ascend/driver/tools/npu-smi; echo "npu:$p"` +
	`;p=$(command -v cnmon 2>/dev/null); [ -z "$p" ] && [ -x /usr/local/neuware/bin/cnmon ] && p=/usr/local/neuware/bin/cnmon; echo "cmon:$p"`

// Detect 从探测输出判定厂商。
func Detect(out string) string {
	var npu, cmon bool
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if v, ok := strings.CutPrefix(line, "npu:"); ok && strings.TrimSpace(v) != "" {
			npu = true
		}
		if v, ok := strings.CutPrefix(line, "cmon:"); ok && strings.TrimSpace(v) != "" {
			cmon = true
		}
	}
	switch {
	case npu:
		return VendorAscend
	case cmon:
		return VendorCambricon
	default:
		return ""
	}
}

func vendorName(v string) string {
	switch v {
	case VendorAscend:
		return "华为昇腾"
	case VendorCambricon:
		return "寒武纪"
	default:
		return ""
	}
}

// detectPath 从探测输出提取指定前缀（"npu:"/"cmon:"）后的工具完整路径。
func detectPath(out, prefix string) string {
	for _, line := range strings.Split(out, "\n") {
		if v, ok := strings.CutPrefix(strings.TrimSpace(line), prefix); ok {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

// toolPath 组装工具查询命令：经 bash 登录 shell 执行。
// SSH 非交互 shell 不加载 /etc/profile.d，缺少驱动库环境
// （昇腾 npu-smi 报 dcmi initialize failed ret -8005 即此因），login shell 与用户手动登录一致。
// 优先裸命令名（与用户交互环境命中一致），无探测路径时直接裸命令。
func toolPath(out, prefix, name string) string {
	if p := detectPath(out, prefix); p != "" {
		return fmt.Sprintf("bash -lc '%s info 2>/dev/null || %s info'", name, p)
	}
	return fmt.Sprintf("bash -lc '%s info'", name)
}

// Collect 探测厂商并拉取全量卡信息；解析宽容：个别字段缺失置空，不影响整体结果。
// 返回 error 仅代表连接级失败；工具查询失败等业务异常记录在 Snapshot.Error。
func Collect(ctx context.Context, r Runner) (*Snapshot, error) {
	out, err := r.Run(ctx, detectCmd)
	if err != nil && strings.TrimSpace(out) == "" {
		return nil, err
	}
	vendor := Detect(out)
	if vendor == "" {
		return &Snapshot{Devices: []Device{}}, nil
	}

	var tool string
	switch vendor {
	case VendorAscend:
		tool = toolPath(out, "npu:", "npu-smi")
	case VendorCambricon:
		tool = toolPath(out, "cmon:", "cnmon")
	}

	snap := &Snapshot{Vendor: vendor, VendorName: vendorName(vendor), Devices: []Device{}}
	out, err = r.Run(ctx, tool)
	if err != nil && strings.TrimSpace(out) == "" {
		snap.Error = err.Error()
		return snap, nil
	}

	if vendor == VendorAscend {
		snap.Devices = parseAscend(out)
	} else {
		snap.Devices = parseCambricon(out)
	}
	if len(snap.Devices) == 0 {
		snap.Error = tool + " 输出未能解析出设备信息：" + snippet(out)
	}
	return snap, nil
}

// snippet 压缩空白并截断，把原始输出片段附在错误信息里辅助排查格式问题。
func snippet(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if len(s) > 300 {
		s = s[:300] + "..."
	}
	return s
}

// normHealth 归一化健康状态：健康/正常/OK/Healthy → OK，其余原样返回。
func normHealth(s string) string {
	v := strings.TrimSpace(s)
	low := strings.ToLower(v)
	if strings.Contains(low, "健康") || strings.Contains(low, "正常") ||
		strings.Contains(low, "ok") || strings.Contains(low, "healthy") {
		return "OK"
	}
	return v
}
