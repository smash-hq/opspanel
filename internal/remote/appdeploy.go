// appdeploy.go 提供应用部署能力：分片上传（断点续传）、服务生命周期管理（启动/停止/重启）、日志查看。
package remote

import (
	"context"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// uploadTimeout 是单个上传相关操作的超时（大文件传输，取长操作上限）。
const uploadTimeout = LongOpTimeout

// appChunkDirName 是部署目录下分片上传的暂存目录名（上传完成后自动清理）。
const appChunkDirName = ".smp-upload"

// appSnippet 压缩空白并截断过长输出，用于错误信息展示。
func appSnippet(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if len(s) > 300 {
		return s[:300] + "..."
	}
	return s
}

// chunkFileName 分片文件名：8 位零填充编号保证字典序即分片顺序。
func chunkFileName(idx int) string {
	return fmt.Sprintf("%08d.chunk", idx)
}

// runSSHWithStdin 在远程节点执行命令并将 src 流式写入命令 stdin（用于分片数据落盘）。
func (rc *RemoteCollector) runSSHWithStdin(ctx context.Context, srv *Server, cmd string, src io.Reader) (string, error) {
	conn, err := rc.pool.Acquire(srv)
	if err != nil {
		return "", err
	}
	defer rc.Release(srv.ID)

	session, err := conn.client.NewSession()
	if err != nil {
		return "", fmt.Errorf("open ssh session: %w", err)
	}
	defer session.Close()

	// stdin 挂上传数据流，CombinedOutput 内部完成拷贝并捕获远端输出
	session.Stdin = src
	type result struct {
		out []byte
		err error
	}
	done := make(chan result, 1)
	go func() {
		out, err := session.CombinedOutput(cmd)
		done <- result{out, err}
	}()

	// 超时/取消时立即关闭 Session 终止传输
	select {
	case r := <-done:
		if r.err != nil {
			return string(r.out), fmt.Errorf("远端命令失败: %s", appSnippet(string(r.out)))
		}
		return string(r.out), nil
	case <-ctx.Done():
		session.Close()
		return "", fmt.Errorf("上传超时或被取消: %w", ctx.Err())
	}
}

// runAppCmd 在目标节点执行一条上传辅助命令（本机走进程内执行器，远程走 SSH），返回输出。
func (rc *RemoteCollector) runAppCmd(ctx context.Context, srv *Server, cmd string) (string, error) {
	conn, err := rc.Acquire(srv)
	if err != nil {
		return "", err
	}
	defer rc.Release(srv.ID)

	ctx, cancel := context.WithTimeout(ctx, uploadTimeout)
	defer cancel()
	out, err := rc.runLogged(ctx, conn, cmd)
	if err != nil {
		return out, fmt.Errorf("远端命令失败: %s", appSnippet(out))
	}
	return out, nil
}

// AppUploadInit 创建分片上传会话的暂存目录 <部署目录>/.smp-upload/<key>/。
func (rc *RemoteCollector) AppUploadInit(ctx context.Context, srv *Server, deployDir, key string) error {
	dir := path.Join(deployDir, appChunkDirName, key)
	if isLocal(srv) {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("创建上传目录失败（检查部署目录写权限）: %w", err)
		}
		return nil
	}
	_, err := rc.runAppCmd(ctx, srv, fmt.Sprintf("mkdir -p %q", dir))
	return err
}

// AppUploadStatus 查询已落盘的分片索引列表（断点续传依据）；目录不存在视为全新上传。
func (rc *RemoteCollector) AppUploadStatus(ctx context.Context, srv *Server, deployDir, key string) ([]int, error) {
	dir := path.Join(deployDir, appChunkDirName, key)
	var names []string

	if isLocal(srv) {
		entries, err := os.ReadDir(dir)
		if err != nil {
			if os.IsNotExist(err) {
				return []int{}, nil
			}
			return nil, fmt.Errorf("读取上传目录失败: %w", err)
		}
		for _, e := range entries {
			names = append(names, e.Name())
		}
	} else {
		out, err := rc.runAppCmd(ctx, srv, fmt.Sprintf("ls -1 %q 2>/dev/null || true", dir))
		if err != nil {
			return nil, err
		}
		names = strings.Fields(out)
	}

	received := []int{}
	for _, n := range names {
		n = strings.TrimSuffix(n, ".chunk")
		if idx, err := strconv.Atoi(n); err == nil && idx >= 0 {
			received = append(received, idx)
		}
	}
	sort.Ints(received)
	return received, nil
}

// AppUploadChunk 将单个分片流式写入暂存目录（已存在的分片直接覆盖，保证幂等）。
func (rc *RemoteCollector) AppUploadChunk(ctx context.Context, srv *Server, deployDir, key string, idx int, src io.Reader) error {
	chunkPath := path.Join(deployDir, appChunkDirName, key, chunkFileName(idx))

	ctx, cancel := context.WithTimeout(ctx, uploadTimeout)
	defer cancel()

	if isLocal(srv) {
		return writeLocalFile(chunkPath, src)
	}

	cmd := fmt.Sprintf("cat > %q", chunkPath)
	if _, err := rc.runSSHWithStdin(ctx, srv, cmd, src); err != nil {
		return err
	}
	return nil
}

// AppUploadComplete 校验分片齐全后合并为最终文件（先写 .part 再原子改名），
// 清理分片目录，返回最终文件字节数。expectSize <= 0 时跳过大小校验。
func (rc *RemoteCollector) AppUploadComplete(ctx context.Context, srv *Server, deployDir, key, relPath string, total int, expectSize int64) (int64, error) {
	chunkDir := path.Join(deployDir, appChunkDirName, key)
	targetPath := path.Join(deployDir, relPath)
	targetDir := path.Dir(targetPath)

	if isLocal(srv) {
		return completeLocalUpload(chunkDir, targetPath, total, expectSize)
	}

	// 远程命令串联：分片数校验 → 合并 → 大小校验 → 建目标目录 → 原子改名 → 清理分片 → 输出最终大小
	// （零填充分片文件名保证 shell glob 展开即分片顺序）
	var sb strings.Builder
	fmt.Fprintf(&sb, `cd %q && n=$(ls -1 *.chunk 2>/dev/null | wc -l) && [ "$n" -eq %d ] && cat *.chunk > merged.part`, chunkDir, total)
	if expectSize > 0 {
		fmt.Fprintf(&sb, ` && [ $(wc -c < merged.part) -eq %d ]`, expectSize)
	}
	fmt.Fprintf(&sb, ` && mkdir -p %q && mv -f merged.part %q`, targetDir, targetPath)
	fmt.Fprintf(&sb, ` && cd %q && rm -rf %q && echo FINAL $(wc -c < %q)`, deployDir, path.Join(appChunkDirName, key), targetPath)

	out, err := rc.runAppCmd(ctx, srv, sb.String())
	if err != nil {
		return 0, fmt.Errorf("合并分片失败（分片不完整或已损坏）: %s", appSnippet(out))
	}
	// 输出最后一行取最终大小
	fields := strings.Fields(strings.TrimSpace(out))
	if len(fields) == 0 {
		return 0, fmt.Errorf("合并完成但未能确认文件大小: %s", appSnippet(out))
	}
	size, err := strconv.ParseInt(fields[len(fields)-1], 10, 64)
	if err != nil || size < 0 {
		return 0, fmt.Errorf("解析最终文件大小失败: %s", appSnippet(out))
	}
	return size, nil
}

// completeLocalUpload 本机节点的分片合并（与远端 AppUploadComplete 同逻辑，直接操作文件系统）。
func completeLocalUpload(chunkDir, targetPath string, total int, expectSize int64) (int64, error) {
	entries, err := os.ReadDir(chunkDir)
	if err != nil {
		return 0, fmt.Errorf("读取分片目录失败: %w", err)
	}
	var chunks []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".chunk") {
			chunks = append(chunks, e.Name())
		}
	}
	if len(chunks) != total {
		return 0, fmt.Errorf("分片不完整: 已有 %d 个，需要 %d 个", len(chunks), total)
	}
	sort.Strings(chunks) // 零填充文件名，字典序即分片顺序

	if err := os.MkdirAll(path.Dir(targetPath), 0o755); err != nil {
		return 0, fmt.Errorf("创建目标目录失败: %w", err)
	}
	partPath := filepath.Join(chunkDir, "merged.part")
	part, err := os.Create(partPath)
	if err != nil {
		return 0, fmt.Errorf("创建合并临时文件失败: %w", err)
	}
	var size int64
	for _, name := range chunks {
		src, err := os.Open(filepath.Join(chunkDir, name))
		if err != nil {
			part.Close()
			return 0, fmt.Errorf("读取分片 %s 失败: %w", name, err)
		}
		n, copyErr := io.Copy(part, src)
		src.Close()
		if copyErr != nil {
			part.Close()
			return 0, fmt.Errorf("合并分片 %s 失败: %w", name, copyErr)
		}
		size += n
	}
	if err := part.Close(); err != nil {
		return 0, fmt.Errorf("写入合并文件失败: %w", err)
	}
	if expectSize > 0 && size != expectSize {
		_ = os.Remove(partPath)
		return 0, fmt.Errorf("合并后大小 %d 字节与预期 %d 不一致，文件可能损坏", size, expectSize)
	}
	if err := os.Rename(partPath, targetPath); err != nil {
		_ = os.Remove(partPath)
		return 0, fmt.Errorf("落盘失败: %w", err)
	}
	_ = os.RemoveAll(chunkDir)
	return size, nil
}

// AppStatus 查询远端进程存活状态；pid<=0 视为 stopped。
func (rc *RemoteCollector) AppStatus(ctx context.Context, srv *Server, pid int) string {
	if pid <= 0 {
		return "stopped"
	}
	conn, err := rc.Acquire(srv)
	if err != nil {
		return "unknown"
	}
	defer rc.Release(srv.ID)

	out := rc.RunOne(ctx, conn, fmt.Sprintf("kill -0 %d 2>/dev/null && echo running || echo stopped", pid))
	if strings.Contains(out, "running") {
		return "running"
	}
	return "stopped"
}

// StartApp 在远端部署目录内启动服务：nohup 后台执行启动命令，返回记录到的 PID。
// 启动命令经 sh -c 执行，支持管道、重定向等 shell 语法；stdout/stderr 追加到日志文件。
func (rc *RemoteCollector) StartApp(ctx context.Context, srv *Server, deployDir, startCmd, logFile string) (int, error) {
	deployDir = strings.TrimSpace(deployDir)
	startCmd = strings.TrimSpace(startCmd)
	if deployDir == "" || startCmd == "" {
		return 0, fmt.Errorf("部署目录与启动命令不能为空")
	}
	if logFile == "" {
		logFile = path.Join(deployDir, "app.log")
	}
	// 相对路径锚定到部署目录
	if !strings.HasPrefix(logFile, "/") {
		logFile = path.Join(deployDir, logFile)
	}

	// 启动属长操作（Java 进程初始化可能较慢），套 10 分钟超时
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()

	conn, err := rc.Acquire(srv)
	if err != nil {
		return 0, err
	}
	defer rc.Release(srv.ID)

	// nohup 脱离会话；</dev/null 防止读 stdin 挂起；echo $! 输出新进程 PID
	cmd := fmt.Sprintf("cd %q && nohup sh -c %q >> %q 2>&1 < /dev/null & echo $!", deployDir, startCmd, logFile)
	out := rc.RunOne(ctx, conn, cmd)
	pidStr := strings.TrimSpace(out)
	// 输出可能带告警行，取最后一行
	if lines := strings.Fields(pidStr); len(lines) > 0 {
		pidStr = lines[len(lines)-1]
	}
	pid, err := strconv.Atoi(pidStr)
	if err != nil || pid <= 0 {
		return 0, fmt.Errorf("启动失败，未能获取进程 PID: %s", appSnippet(out))
	}

	// 给进程一点初始化时间后确认存活，让用户尽早发现秒退（如端口占用、配置缺失）
	time.Sleep(1500 * time.Millisecond)
	if rc.AppStatus(ctx, srv, pid) == "stopped" {
		logTail := rc.TailAppLog(ctx, srv, logFile, 30)
		return 0, fmt.Errorf("进程启动后立即退出（PID %d），日志末尾：\n%s", pid, appSnippet(logTail))
	}
	return pid, nil
}

// StopApp 停止远端服务进程：先 SIGTERM 优雅退出，2 秒后仍存活则 SIGKILL。
func (rc *RemoteCollector) StopApp(ctx context.Context, srv *Server, pid int) error {
	if pid <= 0 {
		return nil
	}
	conn, err := rc.Acquire(srv)
	if err != nil {
		return err
	}
	defer rc.Release(srv.ID)

	cmd := fmt.Sprintf("kill -TERM %d 2>/dev/null; sleep 2; if kill -0 %d 2>/dev/null; then kill -9 %d 2>/dev/null; fi; echo ok", pid, pid, pid)
	out := rc.RunOne(ctx, conn, cmd)
	if !strings.Contains(out, "ok") {
		return fmt.Errorf("停止进程失败: %s", appSnippet(out))
	}
	return nil
}

// TailAppLog 读取日志文件末尾 lines 行；日志不存在时返回提示。
func (rc *RemoteCollector) TailAppLog(ctx context.Context, srv *Server, logFile string, lines int) string {
	if logFile == "" {
		return "未配置日志文件路径"
	}
	conn, err := rc.Acquire(srv)
	if err != nil {
		return "连接失败: " + err.Error()
	}
	defer rc.Release(srv.ID)

	out := rc.RunOne(ctx, conn, fmt.Sprintf("tail -n %d %q 2>/dev/null || echo '（日志文件不存在或暂无内容）'", lines, logFile))
	if strings.TrimSpace(out) == "" {
		return "（日志文件为空）"
	}
	return out
}

// writeLocalFile 本机节点直接写文件系统（无 SSH），.part 原子改名策略防半截文件。
func writeLocalFile(remotePath string, src io.Reader) error {
	if err := os.MkdirAll(path.Dir(remotePath), 0o755); err != nil {
		return fmt.Errorf("创建目录失败: %w", err)
	}
	tmp := remotePath + ".part"
	f, err := os.Create(tmp)
	if err != nil {
		return fmt.Errorf("创建临时文件失败: %w", err)
	}
	_, copyErr := io.Copy(f, src)
	closeErr := f.Close()
	if copyErr != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("写入文件失败: %w", copyErr)
	}
	if closeErr != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("关闭文件失败: %w", closeErr)
	}
	if err := os.Rename(tmp, remotePath); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("落盘失败: %w", err)
	}
	return nil
}
