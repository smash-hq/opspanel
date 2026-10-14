package gpu

import "testing"

const cambriconSampleZH = `
cnmon Ver v3.4.2
chiplist
        card0
                卡号                  ：0
                卡名称                ：MLU370-X8
                卡健康状况            ：健康
                卡功耗(W)             ：65.4 / 150.0
                卡温度(C)             ：41
                卡显存使用量(MB)      ：1024 / 24576
                核心平均使用率(%)     ：12
        card1
                卡号                  ：1
                卡名称                ：MLU370-X8
                卡健康状况            ：故障
                卡功耗(W)             ：70.0 / 150.0
                卡温度(C)             ：52
                卡显存使用量(MB)      ：4096 / 24576
                核心平均使用率(%)     ：35
`

const cambriconSampleEN = `
Card ID               : 0
Card Name             : MLU370-X8
Card Health           : Healthy
Card Power(W)         : 65.4 / 150.0
Card Temp(C)          : 41
Card Memory Usage(MB) : 1024 / 24576
Average Core Usage(%) : 12
`

func TestParseCambriconZH(t *testing.T) {
	devices := parseCambricon(cambriconSampleZH)
	if len(devices) != 2 {
		t.Fatalf("期望 2 张卡，实际 %d：%+v", len(devices), devices)
	}

	d := devices[0]
	if d.Index != "0" || d.Name != "MLU370-X8" || d.Health != "OK" {
		t.Errorf("卡0 基础字段错误：%+v", d)
	}
	if d.PowerW != "65.4" || d.TempC != "41" || d.CoreUtil != "12" {
		t.Errorf("卡0 功耗/温度/利用率错误：%+v", d)
	}
	if d.MemUsedMB != "1024" || d.MemTotalMB != "24576" {
		t.Errorf("卡0 显存错误：%+v", d)
	}

	d = devices[1]
	if d.Index != "1" || d.Health != "故障" {
		t.Errorf("卡1 基础字段错误：%+v", d)
	}
	if d.PowerW != "70.0" || d.TempC != "52" || d.CoreUtil != "35" {
		t.Errorf("卡1 功耗/温度/利用率错误：%+v", d)
	}
	if d.MemUsedMB != "4096" || d.MemTotalMB != "24576" {
		t.Errorf("卡1 显存错误：%+v", d)
	}
}

func TestParseCambriconEN(t *testing.T) {
	devices := parseCambricon(cambriconSampleEN)
	if len(devices) != 1 {
		t.Fatalf("期望 1 张卡，实际 %d：%+v", len(devices), devices)
	}
	d := devices[0]
	if d.Index != "0" || d.Name != "MLU370-X8" || d.Health != "OK" {
		t.Errorf("基础字段错误：%+v", d)
	}
	if d.PowerW != "65.4" || d.TempC != "41" || d.CoreUtil != "12" {
		t.Errorf("功耗/温度/利用率错误：%+v", d)
	}
	if d.MemUsedMB != "1024" || d.MemTotalMB != "24576" {
		t.Errorf("显存错误：%+v", d)
	}
}
