package gpu

import (
	"context"
	"errors"
	"testing"
)

// fakeRunner 按命令返回预置输出。
type fakeRunner struct {
	outputs map[string]string
	err     error // 连接级错误（无输出）
}

func (f *fakeRunner) Run(_ context.Context, cmd string) (string, error) {
	if f.err != nil {
		return "", f.err
	}
	if out, ok := f.outputs[cmd]; ok {
		return out, nil
	}
	return "", nil
}

func TestDetect(t *testing.T) {
	cases := []struct {
		name string
		out  string
		want string
	}{
		{"昇腾", "npu:/usr/local/bin/npu-smi\ncmon:\n", VendorAscend},
		{"寒武纪", "npu:\ncmon:/usr/bin/cnmon\n", VendorCambricon},
		{"都存在时优先昇腾", "npu:/usr/bin/npu-smi\ncmon:/usr/bin/cnmon\n", VendorAscend},
		{"都没有", "npu:\ncmon:\n", ""},
		{"空输出", "", ""},
	}
	for _, c := range cases {
		if got := Detect(c.out); got != c.want {
			t.Errorf("%s: Detect = %q, 期望 %q", c.name, got, c.want)
		}
	}
}

func TestCollectAscend(t *testing.T) {
	r := &fakeRunner{outputs: map[string]string{
		detectCmd:                        "npu:/usr/local/Ascend/npu-smi\n",
		"/usr/local/Ascend/npu-smi info": ascendSample,
	}}
	snap, err := Collect(context.Background(), r)
	if err != nil {
		t.Fatalf("Collect 失败：%v", err)
	}
	if snap.Vendor != VendorAscend || snap.VendorName != "华为昇腾" {
		t.Errorf("厂商错误：%+v", snap)
	}
	if len(snap.Devices) != 2 || snap.Error != "" {
		t.Errorf("设备/错误字段异常：%+v", snap)
	}
}

func TestCollectCambricon(t *testing.T) {
	r := &fakeRunner{outputs: map[string]string{
		detectCmd:             "cmon:/usr/bin/cnmon\n",
		"/usr/bin/cnmon info": cambriconSampleEN,
	}}
	snap, err := Collect(context.Background(), r)
	if err != nil {
		t.Fatalf("Collect 失败：%v", err)
	}
	if snap.Vendor != VendorCambricon || len(snap.Devices) != 1 {
		t.Errorf("厂商/设备异常：%+v", snap)
	}
}

func TestCollectNoGPU(t *testing.T) {
	r := &fakeRunner{outputs: map[string]string{detectCmd: "npu:\ncmon:\n"}}
	snap, err := Collect(context.Background(), r)
	if err != nil {
		t.Fatalf("Collect 失败：%v", err)
	}
	if snap.Vendor != "" || len(snap.Devices) != 0 || snap.Error != "" {
		t.Errorf("未发现 GPU 时应返回空快照：%+v", snap)
	}
}

func TestCollectToolFailure(t *testing.T) {
	// 工具存在但查询无输出：返回快照并携带 Error，而非连接级错误
	r := &fakeRunner{outputs: map[string]string{
		detectCmd: "npu:/usr/bin/npu-smi\n",
	}}
	snap, err := Collect(context.Background(), r)
	if err != nil {
		t.Fatalf("Collect 失败：%v", err)
	}
	if snap.Error == "" {
		t.Errorf("查询失败应写入 Snapshot.Error：%+v", snap)
	}
}

func TestCollectConnError(t *testing.T) {
	r := &fakeRunner{err: errors.New("ssh: connection refused")}
	if _, err := Collect(context.Background(), r); err == nil {
		t.Fatal("连接级错误应向上返回")
	}
}
