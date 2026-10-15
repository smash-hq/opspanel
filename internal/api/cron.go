package api

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/smash-hq/opspanel/internal/cron"
	"github.com/smash-hq/opspanel/internal/host"
	"github.com/smash-hq/opspanel/internal/logger"
	"github.com/smash-hq/opspanel/internal/remote"
)

// CronHandler 管理目标服务器上的用户 crontab：
// 通过 SSH 执行 crontab -l 读取、改写后经 crontab - 写回。
// 本服务不调度执行任务，任务由服务器自身的 cron 守护进程负责。
type CronHandler struct {
	log     *logger.Logger
	servers *host.Store
	rc      *remote.RemoteCollector
	mu      sync.Mutex // 读-改-写串行化，避免并发写回互相覆盖
}

// NewCronHandler 创建 crontab 管理处理器。
func NewCronHandler(log *logger.Logger, servers *host.Store) *CronHandler {
	return &CronHandler{
		log:     log.Named("api.cron"),
		servers: servers,
		rc:      remote.NewRemoteCollector(log),
	}
}

// Register 注册路由。
func (h *CronHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/cron/jobs", h.handleJobs)
	mux.HandleFunc("/api/v1/cron/jobs/", h.handleJobItem)
	mux.HandleFunc("/api/v1/cron/preview", h.handlePreview)
}

// cronJobView 任务视图：附加后续执行时间。
type cronJobView struct {
	*cron.Entry
	NextRuns []string `json:"next_runs"` // 后续 5 次执行时间；@reboot 等为空
}

// jobPayload 新增/修改任务的请求体（字段均可选，修改时空值保留原值）。
type jobPayload struct {
	Name    string `json:"name"`
	Expr    string `json:"expr"`
	Command string `json:"command"`
	Enabled *bool  `json:"enabled"`
}

// handleJobs: GET 任务列表（?server_id=），POST 新增任务。
func (h *CronHandler) handleJobs(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.handleList(w, r)
	case http.MethodPost:
		h.handleCreate(w, r)
	default:
		Fail(w, http.StatusMethodNotAllowed, 405, "method not allowed")
	}
}

// handleList 返回任务列表。
func (h *CronHandler) handleList(w http.ResponseWriter, r *http.Request) {
	srv, ok := h.server(r.Context(), r.URL.Query().Get("server_id"))
	if !ok {
		Fail(w, http.StatusBadRequest, 400, "目标服务器不存在")
		return
	}

	doc, _, err := h.readCrontab(r.Context(), srv)
	if err != nil {
		h.log.Errorw("read crontab failed", "server", srv.ID, "error", err)
		Fail(w, http.StatusBadGateway, 502, "读取 crontab 失败: "+err.Error())
		return
	}

	views := make([]cronJobView, 0, len(doc.Jobs))
	for _, e := range doc.Jobs {
		views = append(views, cronJobView{Entry: e, NextRuns: formatNextRuns(e.Expr, 5)})
	}
	OK(w, views)
}

// handleCreate 新增任务到 crontab 末尾。
func (h *CronHandler) handleCreate(w http.ResponseWriter, r *http.Request) {
	var payload jobPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		Fail(w, http.StatusBadRequest, 400, "invalid request body")
		return
	}
	if strings.TrimSpace(payload.Command) == "" {
		Fail(w, http.StatusBadRequest, 400, "执行命令不能为空")
		return
	}
	if err := cron.ValidateExpr(payload.Expr); err != nil {
		Fail(w, http.StatusBadRequest, 400, err.Error())
		return
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	srv, ok := h.server(r.Context(), r.URL.Query().Get("server_id"))
	if !ok {
		Fail(w, http.StatusBadRequest, 400, "目标服务器不存在")
		return
	}
	doc, mode, err := h.readCrontab(r.Context(), srv)
	if err != nil {
		h.log.Errorw("read crontab failed", "server", srv.ID, "error", err)
		Fail(w, http.StatusBadGateway, 502, "读取 crontab 失败: "+err.Error())
		return
	}
	if err := doc.AddJob(payload.Name, payload.Expr, payload.Command); err != nil {
		Fail(w, http.StatusBadRequest, 400, err.Error())
		return
	}
	if err := h.writeCrontab(r.Context(), srv, mode, doc.Render()); err != nil {
		h.log.Errorw("write crontab failed", "server", srv.ID, "error", err)
		Fail(w, http.StatusBadGateway, 502, "写回 crontab 失败: "+err.Error())
		return
	}
	h.log.Infow("cron job created", "server", srv.ID, "expr", payload.Expr)
	OK(w, nil)
}

// handleJobItem: /api/v1/cron/jobs/{id}（?server_id=），PUT 修改，DELETE 删除。
func (h *CronHandler) handleJobItem(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/v1/cron/jobs/")
	if id == "" {
		Fail(w, http.StatusNotFound, 404, "job not found")
		return
	}

	var handle func(w http.ResponseWriter, r *http.Request, serverID, id string)
	switch r.Method {
	case http.MethodPut:
		handle = h.handleUpdate
	case http.MethodDelete:
		handle = h.handleDelete
	default:
		Fail(w, http.StatusMethodNotAllowed, 405, "method not allowed")
		return
	}
	handle(w, r, r.URL.Query().Get("server_id"), id)
}

// handleUpdate 修改任务（名称/表达式/命令/启停，空值保留原值）。
func (h *CronHandler) handleUpdate(w http.ResponseWriter, r *http.Request, serverID, id string) {
	var payload jobPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		Fail(w, http.StatusBadRequest, 400, "invalid request body")
		return
	}
	if strings.TrimSpace(payload.Expr) != "" {
		if err := cron.ValidateExpr(payload.Expr); err != nil {
			Fail(w, http.StatusBadRequest, 400, err.Error())
			return
		}
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	srv, ok := h.server(r.Context(), serverID)
	if !ok {
		Fail(w, http.StatusBadRequest, 400, "目标服务器不存在")
		return
	}
	doc, mode, err := h.readCrontab(r.Context(), srv)
	if err != nil {
		h.log.Errorw("read crontab failed", "server", srv.ID, "error", err)
		Fail(w, http.StatusBadGateway, 502, "读取 crontab 失败: "+err.Error())
		return
	}
	if err := doc.UpdateJob(id, payload.Name, payload.Expr, payload.Command); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			Fail(w, http.StatusNotFound, 404, "定时任务不存在")
			return
		}
		Fail(w, http.StatusBadRequest, 400, err.Error())
		return
	}
	if payload.Enabled != nil {
		if err := doc.SetJobEnabled(id, *payload.Enabled); err != nil {
			Fail(w, http.StatusNotFound, 404, "定时任务不存在")
			return
		}
	}
	if err := h.writeCrontab(r.Context(), srv, mode, doc.Render()); err != nil {
		h.log.Errorw("write crontab failed", "server", srv.ID, "error", err)
		Fail(w, http.StatusBadGateway, 502, "写回 crontab 失败: "+err.Error())
		return
	}
	h.log.Infow("cron job updated", "server", srv.ID, "id", id)
	OK(w, nil)
}

// handleDelete 删除任务。
func (h *CronHandler) handleDelete(w http.ResponseWriter, r *http.Request, serverID, id string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	srv, ok := h.server(r.Context(), serverID)
	if !ok {
		Fail(w, http.StatusBadRequest, 400, "目标服务器不存在")
		return
	}
	doc, mode, err := h.readCrontab(r.Context(), srv)
	if err != nil {
		h.log.Errorw("read crontab failed", "server", srv.ID, "error", err)
		Fail(w, http.StatusBadGateway, 502, "读取 crontab 失败: "+err.Error())
		return
	}
	if err := doc.RemoveJob(id); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			Fail(w, http.StatusNotFound, 404, "定时任务不存在")
			return
		}
		Fail(w, http.StatusBadRequest, 400, err.Error())
		return
	}
	if err := h.writeCrontab(r.Context(), srv, mode, doc.Render()); err != nil {
		h.log.Errorw("write crontab failed", "server", srv.ID, "error", err)
		Fail(w, http.StatusBadGateway, 502, "写回 crontab 失败: "+err.Error())
		return
	}
	h.log.Infow("cron job deleted", "server", srv.ID, "id", id)
	OK(w, nil)
}

// handlePreview: POST /api/v1/cron/preview {"expr"} 计算后续 5 次执行时间。
func (h *CronHandler) handlePreview(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		Fail(w, http.StatusMethodNotAllowed, 405, "method not allowed")
		return
	}
	var req struct {
		Expr string `json:"expr"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Fail(w, http.StatusBadRequest, 400, "invalid request body")
		return
	}
	if err := cron.ValidateExpr(req.Expr); err != nil {
		OK(w, map[string]any{"valid": false, "error": err.Error()})
		return
	}
	OK(w, map[string]any{"valid": true, "next": formatNextRuns(req.Expr, 5)})
}

// server 校验 server_id 并返回服务器；本机节点直接放行。
func (h *CronHandler) server(ctx context.Context, serverID string) (*host.Server, bool) {
	if serverID == "" {
		serverID = host.LocalServerID
	}
	return h.servers.Get(ctx, serverID)
}

// readCrontab 在目标服务器读取并解析 crontab。
// 默认经 sudo -n 读取（即 root 的 crontab，需免密 sudo），sudo 不可用时回退当前用户 crontab。
// 返回实际生效的命令前缀 mode（"sudo -n " 或 ""），写回时必须使用相同前缀，
// 避免读写落到不同用户的 crontab。服务器尚无 crontab（"no crontab for ..."）时返回空文档。
func (h *CronHandler) readCrontab(ctx context.Context, srv *host.Server) (*cron.Document, string, error) {
	client, err := h.rc.Acquire(toRemoteServer(srv))
	if err != nil {
		return nil, "", err
	}
	defer h.rc.Release(srv.ID)

	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	// 先试 sudo -n（免密），失败回退当前用户；连接级错误（无输出）不回退
	out, err := client.RunOnce(ctx, "sudo -n crontab -l")
	if err != nil && out == "" {
		return nil, "", err
	}
	if err != nil {
		if strings.Contains(out, "no crontab for") {
			return cron.ParseDocument(""), "sudo -n ", nil
		}
		if ctx.Err() != nil {
			return nil, "", ctx.Err()
		}
		sudoErr := crontabError(out)

		out, err = client.RunOnce(ctx, "crontab -l")
		if err == nil {
			return cron.ParseDocument(out), "", nil
		}
		if strings.Contains(out, "no crontab for") {
			return cron.ParseDocument(""), "", nil
		}
		if ctx.Err() != nil {
			return nil, "", ctx.Err()
		}
		return nil, "", fmt.Errorf("%v（sudo 免密 crontab 亦不可用: %v）", crontabError(out), sudoErr)
	}
	return cron.ParseDocument(out), "sudo -n ", nil
}

// crontabError 将 crontab 命令的常见失败输出映射为可操作的提示，未知内容原样返回。
func crontabError(out string) error {
	msg := strings.TrimSpace(out)
	switch {
	case strings.Contains(msg, "a password is required"):
		return errors.New("需要免密 sudo（visudo 配置 NOPASSWD）才能以 root 管理 crontab")
	case strings.Contains(msg, "not allowed to use this program"):
		return errors.New("服务器禁止当前用户使用 crontab（cron.allow/cron.deny 限制），" +
			"可为其配置免密 sudo，或执行: echo \"$USER\" | sudo tee -a /usr/lib/cron/cron.allow")
	case strings.Contains(msg, "Operation not permitted"):
		return errors.New("crontab 访问被系统拒绝（macOS 需为本服务宿主程序授予完全磁盘访问权限）")
	}
	return errors.New(msg)
}

// writeCrontab 将内容经 base64 管道写回目标服务器 crontab（避免引号/换行转义问题）。
// mode 为 readCrontab 返回的前缀，sudo 模式下须作用于管道末端的 crontab -。
func (h *CronHandler) writeCrontab(ctx context.Context, srv *host.Server, mode, content string) error {
	if !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	client, err := h.rc.Acquire(toRemoteServer(srv))
	if err != nil {
		return err
	}
	defer h.rc.Release(srv.ID)

	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	encoded := base64.StdEncoding.EncodeToString([]byte(content))
	cmd := "echo " + encoded + " | base64 -d | " + mode + "crontab -"
	out, err := client.RunOnce(ctx, cmd)
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return crontabError(out)
	}
	return nil
}

// formatNextRuns 计算后续 n 次执行时间并格式化。
func formatNextRuns(expr string, n int) []string {
	times := cron.NextRuns(expr, n)
	out := make([]string, 0, len(times))
	for _, t := range times {
		out = append(out, t.Format("2006-01-02 15:04"))
	}
	return out
}
