package gpu

import (
	"regexp"
	"strings"
)

// memRe 匹配显存 "x / y"（MB）模式。
var memRe = regexp.MustCompile(`(\d+)\s*/\s*(\d+)`)

// parseAscend 解析 npu-smi info 表格输出。
// 数据行以 "|" 开头且恰好 3 列、首列为数字序号，每卡固定两行交替出现：
//
//	| 0    910B2   | OK | 98.5  44  0 / 0  25344/ 65536 |  ← 第一行：序号 型号 | Health | Power Temp …
//	| 0            | 0000:C1:00.0 | 0  0 / 0  25344/ 65536 |  ← 第二行：chip/设备 | Bus-Id | AICore 显存…
//
// 不同版本首列列数不一（旧版第二行为 "NPU Device" 两列，910B 仅 chip 序号一列），
// 故按交替顺序而非首列字段数区分。其余行（版本横幅、表头、进程明细 4 列、
// "No running processes"）全部跳过。
func parseAscend(out string) []Device {
	var devices []Device
	expectChip := false // 下一有效数据行是否为每卡第二行
	for _, raw := range strings.Split(out, "\n") {
		line := strings.TrimSpace(raw)
		if !strings.HasPrefix(line, "|") {
			continue
		}
		first := strings.TrimSpace(strings.TrimPrefix(line, "|"))
		// 版本横幅与列头
		if strings.Contains(line, "npu-smi") ||
			strings.HasPrefix(first, "NPU ") || strings.HasPrefix(first, "Chip ") ||
			first == "NPU" || first == "Chip" {
			continue
		}
		parts := strings.Split(inner(line), "|")
		if len(parts) != 3 {
			continue // 进程明细 4 列 / "No running processes found" 1 列
		}
		head := strings.Fields(parts[0])
		if len(head) == 0 || !isNum(head[0]) {
			continue // 首列非数字序号（表头兜底）
		}

		if !expectChip {
			// 每卡第一行：Index Name | Health | Power Temp …
			d := Device{
				Index:  head[0],
				Name:   strings.Join(head[1:], " "),
				Health: strings.TrimSpace(parts[1]),
			}
			fields := strings.Fields(parts[2])
			if len(fields) > 0 {
				d.PowerW = fields[0]
			}
			if len(fields) > 1 {
				d.TempC = fields[1]
			}
			devices = append(devices, d)
		} else if len(devices) > 0 {
			// 每卡第二行：更新上一张卡的 AICore 利用率与显存。
			d := &devices[len(devices)-1]
			fields := strings.Fields(parts[2])
			if len(fields) > 0 {
				d.CoreUtil = fields[0]
			}
			// 显存取最后一个 "x / y"：910B 输出含 Memory-Usage 与 HBM-Usage 两组，
			// 前者常为 0/0，卡显存以 HBM 为准；旧版输出仅一组时同样命中。
			if ms := memRe.FindAllStringSubmatch(strings.Join(fields, " "), -1); len(ms) > 0 {
				m := ms[len(ms)-1]
				d.MemUsedMB, d.MemTotalMB = m[1], m[2]
			}
		}
		expectChip = !expectChip
	}
	return devices
}

// inner 去掉行首尾的 "|" 分隔符，返回中间内容。
func inner(line string) string {
	s := strings.TrimSpace(strings.TrimPrefix(line, "|"))
	return strings.TrimSuffix(s, "|")
}

// isNum 判断字符串是否为非负整数（用于识别数据行首列序号，排除表头兜底）。
func isNum(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}
