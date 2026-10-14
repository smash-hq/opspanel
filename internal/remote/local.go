package remote

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// localShell 在服务本机直接执行 shell 命令（os/exec），
// 使本机节点无需 SSH 凭据即可复用远程节点的全部采集逻辑。
// 执行身份即服务的运行用户；结构无状态，可安全并发使用。
type localShell struct{}

// RunOnce 执行单条命令并返回合并输出（stdout+stderr），语义与 sshConn.RunOnce 一致：
// 命令非零退出视为正常业务结果（输出照常返回），不视为通道故障。
// ctx 取消/超时会被 os/exec 转换为 kill 信号终止进程。
func (l *localShell) RunOnce(ctx context.Context, cmd string) (string, error) {
	out, err := exec.CommandContext(ctx, "/bin/sh", "-c", cmd).CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("local exec: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// RunRaw 执行单条命令并返回原始字节输出（不 TrimSpace，用于二进制传输如文件下载）。
func (l *localShell) RunRaw(ctx context.Context, cmd string) ([]byte, error) {
	return exec.CommandContext(ctx, "/bin/sh", "-c", cmd).CombinedOutput()
}

// usable 本机执行器始终可用（无连接概念）。
func (l *localShell) usable() bool { return true }

// Close 本机执行器无需清理。
func (l *localShell) Close() error { return nil }
