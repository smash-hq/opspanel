package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/smash-hq/server-management/internal/host"
	"github.com/smash-hq/server-management/internal/logger"
	"github.com/smash-hq/server-management/internal/software"
)

// SoftwareHandler 提供软件管理 API。
type SoftwareHandler struct {
	log     *logger.Logger
	store   *host.Store
	manager *software.Manager
}

// NewSoftwareHandler 创建软件管理处理器。
func NewSoftwareHandler(log *logger.Logger, store *host.Store) *SoftwareHandler {
	return &SoftwareHandler{
		log:     log.Named("api.software"),
		store:   store,
		manager: software.NewManager(log),
	}
}

// Register 注册路由。
func (h *SoftwareHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/software/catalog", h.handleCatalog)           // GET 软件目录
	mux.HandleFunc("/api/v1/software/status", h.handleStatus)             // GET ?server=<id> 批量查状态
	mux.HandleFunc("/api/v1/software/install", h.handleInstall)           // POST 安装
	mux.HandleFunc("/api/v1/software/uninstall", h.handleUninstall)       // POST 卸载
	mux.HandleFunc("/api/v1/software/service", h.handleService)           // POST 服务启停
}

// handleCatalog: GET 软件目录列表。
func (h *SoftwareHandler) handleCatalog(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		Fail(w, http.StatusMethodNotAllowed, 405, "method not allowed")
		return
	}
	OK(w, software.Catalog)
}

// handleStatus: GET ?server=<id> 查询所有软件包的安装状态。
func (h *SoftwareHandler) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		Fail(w, http.StatusMethodNotAllowed, 405, "method not allowed")
		return
	}

	serverID := r.URL.Query().Get("server")
	if serverID == "" || serverID == "local" {
		Fail(w, http.StatusBadRequest, 400, "remote server required")
		return
	}

	srv, ok := h.store.Get(serverID)
	if !ok {
		Fail(w, http.StatusNotFound, 404, "server not found")
		return
	}

	var statuses []*software.SoftwareStatus
	for _, pkg := range software.Catalog {
		status := h.manager.CheckStatus(srv, pkg)
		statuses = append(statuses, status)
	}

	OK(w, statuses)
}

// handleInstall: POST 安装软件。
func (h *SoftwareHandler) handleInstall(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		Fail(w, http.StatusMethodNotAllowed, 405, "method not allowed")
		return
	}

	var req struct {
		ServerID  string `json:"server_id"`
		PackageID string `json:"package_id"`
	}
	if err := decodeBody(r, &req); err != nil {
		Fail(w, http.StatusBadRequest, 400, "invalid request body")
		return
	}

	srv, pkg, err := h.getServerAndPackage(req.ServerID, req.PackageID)
	if err != nil {
		Fail(w, http.StatusBadRequest, 400, err.Error())
		return
	}

	go func() {
		if err := h.manager.Install(srv, pkg); err != nil {
			h.log.Errorw("install failed", "pkg", pkg.ID, "error", err)
		}
	}()

	OK(w, map[string]string{"status": "installing", "package": pkg.ID})
}

// handleUninstall: POST 卸载软件。
func (h *SoftwareHandler) handleUninstall(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		Fail(w, http.StatusMethodNotAllowed, 405, "method not allowed")
		return
	}

	var req struct {
		ServerID  string `json:"server_id"`
		PackageID string `json:"package_id"`
	}
	if err := decodeBody(r, &req); err != nil {
		Fail(w, http.StatusBadRequest, 400, "invalid request body")
		return
	}

	srv, pkg, err := h.getServerAndPackage(req.ServerID, req.PackageID)
	if err != nil {
		Fail(w, http.StatusBadRequest, 400, err.Error())
		return
	}

	go func() {
		if err := h.manager.Uninstall(srv, pkg); err != nil {
			h.log.Errorw("uninstall failed", "pkg", pkg.ID, "error", err)
		}
	}()

	OK(w, map[string]string{"status": "uninstalling", "package": pkg.ID})
}

// handleService: POST 服务启停。
func (h *SoftwareHandler) handleService(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		Fail(w, http.StatusMethodNotAllowed, 405, "method not allowed")
		return
	}

	var req struct {
		ServerID  string `json:"server_id"`
		PackageID string `json:"package_id"`
		Action    string `json:"action"` // start / stop / restart
	}
	if err := decodeBody(r, &req); err != nil {
		Fail(w, http.StatusBadRequest, 400, "invalid request body")
		return
	}

	if req.Action != "start" && req.Action != "stop" && req.Action != "restart" {
		Fail(w, http.StatusBadRequest, 400, "invalid action")
		return
	}

	srv, pkg, err := h.getServerAndPackage(req.ServerID, req.PackageID)
	if err != nil {
		Fail(w, http.StatusBadRequest, 400, err.Error())
		return
	}

	if err := h.manager.ServiceControl(srv, pkg, req.Action); err != nil {
		h.log.Errorw("service control failed", "pkg", pkg.ID, "action", req.Action, "error", err)
		Fail(w, http.StatusInternalServerError, 500, err.Error())
		return
	}

	OK(w, map[string]string{"status": "ok", "action": req.Action, "package": pkg.ID})
}

// --- 内部方法 ---

func (h *SoftwareHandler) getServerAndPackage(serverID, pkgID string) (*host.Server, *software.Package, error) {
	if serverID == "" || serverID == "local" {
		return nil, nil, fmt.Errorf("software management requires a remote server")
	}
	srv, ok := h.store.Get(serverID)
	if !ok {
		return nil, nil, fmt.Errorf("server not found: %s", serverID)
	}
	pkg, ok := software.Get(pkgID)
	if !ok {
		return nil, nil, fmt.Errorf("unknown package: %s", pkgID)
	}
	return srv, pkg, nil
}

// decodeBody 解析 JSON 请求体。
func decodeBody(r *http.Request, v any) error {
	return json.NewDecoder(r.Body).Decode(v)
}
