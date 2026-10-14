package gpu

import "strings"

// parseCambricon 解析 cnmon info 输出（中英文双语 label，按 Contains 匹配）。
//
//	卡号                  ：0
//	卡名称                ：MLU370-X8
//	卡健康状况            ：健康
//	卡功耗(W)             ：65.4 / 150.0
//	卡温度(C)             ：41
//	卡显存使用量(MB)      ：1024 / 24576
//	核心平均使用率(%)     ：12
func parseCambricon(out string) []Device {
	var devices []Device
	for _, raw := range strings.Split(out, "\n") {
		label, value := splitLabel(strings.TrimSpace(raw))
		if label == "" {
			continue
		}
		low := strings.ToLower(label)
		switch {
		case strings.Contains(low, "卡号") || strings.Contains(low, "card id"):
			devices = append(devices, Device{Index: strings.TrimSpace(value)})
		case strings.Contains(low, "卡名称") || strings.Contains(low, "card name"):
			setLast(&devices, func(d *Device) { d.Name = strings.TrimSpace(value) })
		case strings.Contains(low, "健康状况") || strings.Contains(low, "card health"):
			setLast(&devices, func(d *Device) { d.Health = normHealth(value) })
		case strings.Contains(low, "功耗") || strings.Contains(low, "power"):
			setLast(&devices, func(d *Device) { d.PowerW = firstField(value) })
		case strings.Contains(low, "温度") || strings.Contains(low, "temp"):
			setLast(&devices, func(d *Device) { d.TempC = firstField(value) })
		case strings.Contains(low, "显存") || strings.Contains(low, "memory"):
			// 注意须在 "usage" 分支之前：英文 "Card Memory Usage" 同时含 usage
			setLast(&devices, func(d *Device) {
				if m := memRe.FindStringSubmatch(value); m != nil {
					d.MemUsedMB, d.MemTotalMB = m[1], m[2]
				}
			})
		case strings.Contains(low, "使用率") || strings.Contains(low, "usage"):
			setLast(&devices, func(d *Device) { d.CoreUtil = firstField(value) })
		}
	}
	return devices
}

// setLast 对最后一张卡的字段赋值（无卡时忽略）。
func setLast(devices *[]Device, set func(*Device)) {
	if len(*devices) == 0 {
		return
	}
	set(&(*devices)[len(*devices)-1])
}

// splitLabel 按 label 分隔符（全角"："或半角":"）拆出行标签与值；无分隔符返回空。
func splitLabel(line string) (label, value string) {
	if i := strings.Index(line, "："); i >= 0 {
		return strings.TrimSpace(line[:i]), strings.TrimSpace(line[i+len("："):])
	}
	if i := strings.Index(line, ":"); i >= 0 {
		return strings.TrimSpace(line[:i]), strings.TrimSpace(line[i+1:])
	}
	return "", ""
}

// firstField 取值的首个字段（如 "65.4 / 150.0" → "65.4"）。
func firstField(s string) string {
	f := strings.Fields(s)
	if len(f) == 0 {
		return ""
	}
	return f[0]
}
