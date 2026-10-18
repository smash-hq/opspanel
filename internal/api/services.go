package api

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/smash-hq/opspanel/internal/host"
	"github.com/smash-hq/opspanel/internal/logger"
	"github.com/smash-hq/opspanel/internal/remote"
)

// ServicesHandler 服务器 systemd 服务管理接口：无 agent，经 SSH 执行 systemctl。
type ServicesHandler struct {
	log     *logger.Logger
	servers *host.Store
	rc      *remote.RemoteCollector
}

// NewServicesHandler 创建服务管理处理器。
func NewServicesHandler(log *logger.Logger, servers *host.Store) *ServicesHandler {
	return &ServicesHandler{
		log:     log.Named("api.services"),
		servers: servers,
		rc:      remote.NewRemoteCollector(log),
	}
}

// Register 注册路由。
func (h *ServicesHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/services", h.handleList)          // GET ?server= 服务列表
	mux.HandleFunc("/api/v1/services/action", h.handleAction) // POST 启停/重启/自启
	mux.HandleFunc("/api/v1/services/logs", h.handleLogs)     // GET ?server=&name=&lines= 服务日志
	mux.HandleFunc("/api/v1/services/unit", h.handleUnit)     // GET 读取 / POST 保存 unit 文件
}

// handleList: GET /api/v1/services?server=<id> 返回 systemd 服务列表。
func (h *ServicesHandler) handleList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		Fail(w, http.StatusMethodNotAllowed, 405, "method not allowed")
		return
	}
	srv, ok := h.server(r.Context(), r.URL.Query().Get("server"))
	if !ok {
		Fail(w, http.StatusBadRequest, 400, "目标服务器不存在")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	list, err := h.rc.ListServices(ctx, toRemoteServer(srv))
	if err != nil {
		if errors.Is(err, remote.ErrNoSystemd) {
			Fail(w, http.StatusBadRequest, 400, err.Error())
			return
		}
		h.log.Errorw("list services failed", "server", srv.ID, "error", err)
		Fail(w, http.StatusBadGateway, 502, "服务列表获取失败: "+err.Error())
		return
	}
	OK(w, list)
}

// handleAction: POST /api/v1/services/action 执行服务操作（start/stop/restart/enable/disable）。
func (h *ServicesHandler) handleAction(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		Fail(w, http.StatusMethodNotAllowed, 405, "method not allowed")
		return
	}
	var req struct {
		ServerID string `json:"server_id"`
		Name     string `json:"name"`
		Action   string `json:"action"`
	}
	if err := decodeBody(r, &req); err != nil {
		Fail(w, http.StatusBadRequest, 400, "invalid request body")
		return
	}
	srv, ok := h.server(r.Context(), req.ServerID)
	if !ok {
		Fail(w, http.StatusBadRequest, 400, "目标服务器不存在")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Minute)
	defer cancel()

	if err := h.rc.ServiceAction(ctx, toRemoteServer(srv), req.Name, req.Action); err != nil {
		h.log.Errorw("service action failed", "server", srv.ID, "name", req.Name, "action", req.Action, "error", err)
		Fail(w, http.StatusBadGateway, 502, "操作失败: "+err.Error())
		return
	}
	OK(w, map[string]string{"status": "ok"})
}

// handleLogs: GET /api/v1/services/logs?server=<id>&name=<unit>&lines=<n> 查看服务日志（journalctl 优先）。
func (h *ServicesHandler) handleLogs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		Fail(w, http.StatusMethodNotAllowed, 405, "method not allowed")
		return
	}
	srv, ok := h.server(r.Context(), r.URL.Query().Get("server"))
	if !ok {
		Fail(w, http.StatusBadRequest, 400, "目标服务器不存在")
		return
	}
	name := strings.TrimSpace(r.URL.Query().Get("name"))
	if name == "" {
		Fail(w, http.StatusBadRequest, 400, "缺少服务名")
		return
	}
	lines := 100
	if n, err := strconv.Atoi(r.URL.Query().Get("lines")); err == nil && n > 0 && n <= 2000 {
		lines = n
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	logText, err := h.rc.GetLog(ctx, toRemoteServer(srv), name, lines)
	if err != nil {
		h.log.Errorw("service logs failed", "server", srv.ID, "name", name, "error", err)
		Fail(w, http.StatusBadGateway, 502, "日志获取失败: "+err.Error())
		return
	}
	OK(w, map[string]string{"log": logText})
}

// handleUnit: GET /api/v1/services/unit?server=<id>&name=<unit> 读取 unit 文件；
// POST 保存 unit 文件（body: {server_id, name, content}）。
func (h *ServicesHandler) handleUnit(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.readUnit(w, r)
	case http.MethodPost:
		h.saveUnit(w, r)
	default:
		Fail(w, http.StatusMethodNotAllowed, 405, "method not allowed")
	}
}

// readUnit: 读取 unit 文件内容与路径。
func (h *ServicesHandler) readUnit(w http.ResponseWriter, r *http.Request) {
	srv, ok := h.server(r.Context(), r.URL.Query().Get("server"))
	if !ok {
		Fail(w, http.StatusBadRequest, 400, "目标服务器不存在")
		return
	}
	name := strings.TrimSpace(r.URL.Query().Get("name"))
	if name == "" {
		Fail(w, http.StatusBadRequest, 400, "缺少服务名")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	path, content, err := h.rc.GetUnitFile(ctx, toRemoteServer(srv), name)
	if err != nil {
		if errors.Is(err, remote.ErrNoSystemd) {
			Fail(w, http.StatusBadRequest, 400, err.Error())
			return
		}
		h.log.Errorw("read unit file failed", "server", srv.ID, "name", name, "error", err)
		Fail(w, http.StatusBadGateway, 502, "unit 文件读取失败: "+err.Error())
		return
	}
	OK(w, map[string]string{"path": path, "content": content})
}

// saveUnit: 保存 unit 文件内容。
func (h *ServicesHandler) saveUnit(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ServerID string `json:"server_id"`
		Name     string `json:"name"`
		Content  string `json:"content"`
	}
	if err := decodeBody(r, &req); err != nil {
		Fail(w, http.StatusBadRequest, 400, "invalid request body")
		return
	}
	srv, ok := h.server(r.Context(), req.ServerID)
	if !ok {
		Fail(w, http.StatusBadRequest, 400, "目标服务器不存在")
		return
	}
	if strings.TrimSpace(req.Content) == "" {
		Fail(w, http.StatusBadRequest, 400, "unit 文件内容不能为空")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()

	if err := h.rc.SaveUnitFile(ctx, toRemoteServer(srv), req.Name, req.Content); err != nil {
		if errors.Is(err, remote.ErrNoSystemd) {
			Fail(w, http.StatusBadRequest, 400, err.Error())
			return
		}
		h.log.Errorw("save unit file failed", "server", srv.ID, "name", req.Name, "error", err)
		Fail(w, http.StatusBadGateway, 502, "unit 文件保存失败: "+err.Error())
		return
	}
	OK(w, map[string]string{"status": "ok"})
}

// server 解析目标服务器；server 为空时取本机节点。
func (h *ServicesHandler) server(ctx context.Context, serverID string) (*host.Server, bool) {
	if strings.TrimSpace(serverID) == "" {
		serverID = host.LocalServerID
	}
	return h.servers.Get(ctx, serverID)
}
