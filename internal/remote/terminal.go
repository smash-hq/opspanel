package remote

import (
	"fmt"
	"io"

	"golang.org/x/crypto/ssh"
)

// TerminalSession 封装一个交互式 SSH PTY 会话。
// 使用独立连接（不复用共享连接池）：终端会话生命周期长，
// 独立连接可避免被池的空闲巡检回收或配置变更时误杀。
type TerminalSession struct {
	client  *ssh.Client
	session *ssh.Session
	stdin   io.WriteCloser
	stdout  io.Reader
}

// OpenTerminalSession 在远程服务器上开启交互式 shell（PTY）。
// cols/rows 为初始终端尺寸。
func (rc *RemoteCollector) OpenTerminalSession(srv *Server, cols, rows int) (*TerminalSession, error) {
	conn, err := newSSHConn(srv)
	if err != nil {
		return nil, err
	}
	ts := &TerminalSession{client: conn.client}

	defer func() {
		if err != nil {
			conn.Close()
		}
	}()

	session, err := conn.client.NewSession()
	if err != nil {
		return nil, fmt.Errorf("new session: %w", err)
	}
	ts.session = session

	if rows <= 0 {
		rows = 24
	}
	if cols <= 0 {
		cols = 80
	}
	if err = session.RequestPty("xterm-256color", rows, cols, ssh.TerminalModes{
		ssh.ECHO:          1,
		ssh.TTY_OP_ISPEED: 115200,
		ssh.TTY_OP_OSPEED: 115200,
	}); err != nil {
		return nil, fmt.Errorf("request pty: %w", err)
	}

	if ts.stdin, err = session.StdinPipe(); err != nil {
		return nil, fmt.Errorf("stdin pipe: %w", err)
	}
	if ts.stdout, err = session.StdoutPipe(); err != nil {
		return nil, fmt.Errorf("stdout pipe: %w", err)
	}

	if err = session.Shell(); err != nil {
		return nil, fmt.Errorf("start shell: %w", err)
	}

	rc.log.Infow("terminal session opened", "server", srv.ID, "size", fmt.Sprintf("%dx%d", cols, rows))
	return ts, nil
}

// Read 读取远端输出（实现 io.Reader）。
func (ts *TerminalSession) Read(p []byte) (int, error) { return ts.stdout.Read(p) }

// Write 写入远端输入（实现 io.Writer）。
func (ts *TerminalSession) Write(p []byte) (int, error) { return ts.stdin.Write(p) }

// Resize 调整 PTY 窗口尺寸。
func (ts *TerminalSession) Resize(cols, rows int) error {
	return ts.session.WindowChange(rows, cols)
}

// Close 关闭会话与底层连接（优雅退出：先关闭 stdin 让 shell 收到 EOF）。
func (ts *TerminalSession) Close() {
	if ts.stdin != nil {
		_ = ts.stdin.Close()
	}
	if ts.session != nil {
		_ = ts.session.Close()
	}
	if ts.client != nil {
		_ = ts.client.Close()
	}
}
