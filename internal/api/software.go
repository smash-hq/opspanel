package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/smash-hq/server-management/internal/host"
	"github.com/smash-hq/server-management/internal/logger"
	"github.com/smash-hq/server-management/internal/remote"
	"github.com/smash-hq/server-management/internal/software"
	"github.com/smash-hq/server-management/internal/storage"
)

// SoftwareHandler 提供软件管理 API。
type SoftwareHandler struct {
	log            *logger.Logger
	store          *host.Store
	swStore        *storage.SoftwareStore
	manager        *software.Manager
	remoteCollector *remote.RemoteCollector
	db             *storage.DB
}

// NewSoftwareHandler 创建软件管理处理器。
func NewSoftwareHandler(log *logger.Logger, store *host.Store, swStore *storage.SoftwareStore, db *storage.DB) *SoftwareHandler {
	return &SoftwareHandler{
		log:            log.Named("api.software"),
		store:          store,
		swStore:        swStore,
		manager:        software.NewManager(log),
		remoteCollector: remote.NewRemoteCollector(log),
		db:             db,
	}
}

// Register 注册路由。
func (h *SoftwareHandler) Register(mux *http.ServeMux) {
	// 软件目录与安装
	mux.HandleFunc("/api/v1/software/catalog", h.handleCatalog)
	mux.HandleFunc("/api/v1/software/status", h.handleStatus)
	mux.HandleFunc("/api/v1/software/install", h.handleInstall)
	mux.HandleFunc("/api/v1/software/uninstall", h.handleUninstall)
	mux.HandleFunc("/api/v1/software/service", h.handleService)

	// 软件包管理
	mux.HandleFunc("/api/v1/software/packages", h.handlePackages)          // GET 列表 / POST 新增
	mux.HandleFunc("/api/v1/software/packages/", h.handlePackageDetail)    // PUT 更新 / DELETE 删除

	// 日志查看
	mux.HandleFunc("/api/v1/software/logs", h.handleLogs)                  // GET ?server=&package=&lines=

	// 进程管理
	mux.HandleFunc("/api/v1/processes", h.handleProcesses)                 // GET ?server=&lines= 进程列表
	mux.HandleFunc("/api/v1/processes/kill", h.handleKillProcess)          // POST 终止进程
	mux.HandleFunc("/api/v1/processes/log", h.handleProcessLog)            // GET ?server=&pid=&lines= 进程日志

	// 编程语言环境
	mux.HandleFunc("/api/v1/languages", h.handleLanguages)                 // GET 列表 / POST 新增
	mux.HandleFunc("/api/v1/languages/", h.handleLanguageDetail)           // PUT 更新 / DELETE 删除
	mux.HandleFunc("/api/v1/languages/detect", h.handleDetectLanguages)    // GET ?server= 检测已安装
	mux.HandleFunc("/api/v1/languages/install", h.handleInstallLanguage)   // POST 安装语言

	// 镜像源管理
	mux.HandleFunc("/api/v1/software/sources", h.handleSources)            // GET 列表 / POST 新增
	mux.HandleFunc("/api/v1/software/sources/", h.handleSourceDetail)     // PUT 更新 / DELETE 删除
}

// --- 软件目录与安装 ---

// handleCatalog: GET 软件目录列表（从数据库读取）。
func (h *SoftwareHandler) handleCatalog(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		Fail(w, http.StatusMethodNotAllowed, 405, "method not allowed")
		return
	}
	OK(w, h.swStore.ListPackages())
}

// handleStatus: GET ?server=<id> 查询所有软件包的安装状态。
func (h *SoftwareHandler) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		Fail(w, http.StatusMethodNotAllowed, 405, "method not allowed")
		return
	}

	serverID := r.URL.Query().Get("server")
	if serverID == "" || serverID == host.LocalServerID {
		Fail(w, http.StatusBadRequest, 400, "remote server required")
		return
	}

	srv, ok := h.store.Get(serverID)
	if !ok {
		Fail(w, http.StatusNotFound, 404, "server not found")
		return
	}

	var statuses []*software.SoftwareStatus
	for _, pkg := range h.swStore.ListPackages() {
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
		Action    string `json:"action"`
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

// --- 软件包管理 ---

// handlePackages: GET 列表 / POST 新增。
func (h *SoftwareHandler) handlePackages(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		OK(w, h.swStore.ListPackages())

	case http.MethodPost:
		var pkg software.Package
		if err := decodeBody(r, &pkg); err != nil {
			Fail(w, http.StatusBadRequest, 400, "invalid request body")
			return
		}
		if pkg.ID == "" || pkg.Name == "" {
			Fail(w, http.StatusBadRequest, 400, "id and name are required")
			return
		}
		if err := h.swStore.AddPackage(&pkg); err != nil {
			h.log.Errorw("add package failed", "error", err)
			Fail(w, http.StatusInternalServerError, 500, "failed to add package")
			return
		}
		// 刷新内存索引
		software.LoadFromStore(h.swStore)
		OK(w, pkg)

	default:
		Fail(w, http.StatusMethodNotAllowed, 405, "method not allowed")
	}
}

// handlePackageDetail: PUT 更新 / DELETE 删除。
func (h *SoftwareHandler) handlePackageDetail(w http.ResponseWriter, r *http.Request) {
	id := trimPrefix(r.URL.Path, "/api/v1/software/packages/")
	if id == "" {
		Fail(w, http.StatusBadRequest, 400, "package id required")
		return
	}

	switch r.Method {
	case http.MethodPut:
		var pkg software.Package
		if err := decodeBody(r, &pkg); err != nil {
			Fail(w, http.StatusBadRequest, 400, "invalid request body")
			return
		}
		pkg.ID = id
		if err := h.swStore.UpdatePackage(&pkg); err != nil {
			h.log.Errorw("update package failed", "error", err)
			Fail(w, http.StatusInternalServerError, 500, "failed to update package")
			return
		}
		software.LoadFromStore(h.swStore)
		OK(w, pkg)

	case http.MethodDelete:
		if err := h.swStore.DeletePackage(id); err != nil {
			h.log.Errorw("delete package failed", "error", err)
			Fail(w, http.StatusInternalServerError, 500, "failed to delete package")
			return
		}
		software.LoadFromStore(h.swStore)
		OK(w, map[string]string{"deleted": id})

	default:
		Fail(w, http.StatusMethodNotAllowed, 405, "method not allowed")
	}
}

// --- 镜像源管理 ---

// handleSources: GET 列表 / POST 新增。
func (h *SoftwareHandler) handleSources(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		OK(w, h.swStore.ListSources())

	case http.MethodPost:
		var src storage.SourceRecord
		if err := decodeBody(r, &src); err != nil {
			Fail(w, http.StatusBadRequest, 400, "invalid request body")
			return
		}
		if src.ID == "" || src.Name == "" || src.URL == "" {
			Fail(w, http.StatusBadRequest, 400, "id, name and url are required")
			return
		}
		if err := h.swStore.AddSource(&src); err != nil {
			h.log.Errorw("add source failed", "error", err)
			Fail(w, http.StatusInternalServerError, 500, "failed to add source")
			return
		}
		OK(w, src)

	default:
		Fail(w, http.StatusMethodNotAllowed, 405, "method not allowed")
	}
}

// handleSourceDetail: PUT 更新 / DELETE 删除。
func (h *SoftwareHandler) handleSourceDetail(w http.ResponseWriter, r *http.Request) {
	id := trimPrefix(r.URL.Path, "/api/v1/software/sources/")
	if id == "" {
		Fail(w, http.StatusBadRequest, 400, "source id required")
		return
	}

	switch r.Method {
	case http.MethodPut:
		var src storage.SourceRecord
		if err := decodeBody(r, &src); err != nil {
			Fail(w, http.StatusBadRequest, 400, "invalid request body")
			return
		}
		src.ID = id
		if err := h.swStore.UpdateSource(&src); err != nil {
			h.log.Errorw("update source failed", "error", err)
			Fail(w, http.StatusInternalServerError, 500, "failed to update source")
			return
		}
		OK(w, src)

	case http.MethodDelete:
		if err := h.swStore.DeleteSource(id); err != nil {
			h.log.Errorw("delete source failed", "error", err)
			Fail(w, http.StatusInternalServerError, 500, "failed to delete source")
			return
		}
		OK(w, map[string]string{"deleted": id})

	default:
		Fail(w, http.StatusMethodNotAllowed, 405, "method not allowed")
	}
}

// --- 日志查看 ---

// handleLogs: GET ?server=<id>&package=<id>&lines=<n> 获取软件运行日志。
func (h *SoftwareHandler) handleLogs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		Fail(w, http.StatusMethodNotAllowed, 405, "method not allowed")
		return
	}

	serverID := r.URL.Query().Get("server")
	pkgID := r.URL.Query().Get("package")
	linesStr := r.URL.Query().Get("lines")
	lines := 100
	if linesStr != "" {
		fmt.Sscanf(linesStr, "%d", &lines)
	}
	if lines <= 0 || lines > 1000 {
		lines = 100
	}

	srv, ok := h.store.Get(serverID)
	if !ok {
		Fail(w, http.StatusNotFound, 404, "server not found")
		return
	}

	pkg, ok := software.Get(pkgID)
	if !ok {
		Fail(w, http.StatusNotFound, 404, "package not found")
		return
	}

	log, err := h.manager.GetLog(srv, pkg.ID, lines)
	if err != nil {
		h.log.Errorw("get log failed", "server", serverID, "pkg", pkgID, "error", err)
		Fail(w, http.StatusInternalServerError, 500, "failed to get log: "+err.Error())
		return
	}

	OK(w, map[string]string{
		"package": pkgID,
		"server":  serverID,
		"log":     log,
	})
}

// --- 进程管理 ---

// handleProcesses: GET ?server=<id>&lines=<n> 获取进程列表。
func (h *SoftwareHandler) handleProcesses(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		Fail(w, http.StatusMethodNotAllowed, 405, "method not allowed")
		return
	}

	serverID := r.URL.Query().Get("server")
	linesStr := r.URL.Query().Get("lines")
	lines := 50
	if linesStr != "" {
		fmt.Sscanf(linesStr, "%d", &lines)
	}

	srv, ok := h.store.Get(serverID)
	if !ok {
		Fail(w, http.StatusNotFound, 404, "server not found")
		return
	}

	processes, err := h.remoteCollector.ListProcesses(toRemoteServer(srv), lines)
	if err != nil {
		h.log.Errorw("list processes failed", "server", serverID, "error", err)
		Fail(w, http.StatusInternalServerError, 500, "failed to list processes: "+err.Error())
		return
	}

	OK(w, processes)
}

// handleKillProcess: POST {"server_id": "...", "pid": 123, "force": false} 终止进程。
func (h *SoftwareHandler) handleKillProcess(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		Fail(w, http.StatusMethodNotAllowed, 405, "method not allowed")
		return
	}

	var req struct {
		ServerID string `json:"server_id"`
		PID      int    `json:"pid"`
		Force    bool   `json:"force"`
	}
	if err := decodeBody(r, &req); err != nil {
		Fail(w, http.StatusBadRequest, 400, "invalid request body")
		return
	}

	srv, ok := h.store.Get(req.ServerID)
	if !ok {
		Fail(w, http.StatusNotFound, 404, "server not found")
		return
	}

	if err := h.remoteCollector.KillProcess(toRemoteServer(srv), req.PID, req.Force); err != nil {
		h.log.Errorw("kill process failed", "server", req.ServerID, "pid", req.PID, "error", err)
		Fail(w, http.StatusInternalServerError, 500, "failed to kill process: "+err.Error())
		return
	}

	OK(w, map[string]string{"status": "ok", "pid": fmt.Sprintf("%d", req.PID)})
}

// handleProcessLog: GET ?server=<id>&pid=<pid>&lines=<n> 获取进程日志。
func (h *SoftwareHandler) handleProcessLog(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		Fail(w, http.StatusMethodNotAllowed, 405, "method not allowed")
		return
	}

	serverID := r.URL.Query().Get("server")
	pidStr := r.URL.Query().Get("pid")
	linesStr := r.URL.Query().Get("lines")
	pid := 0
	lines := 50
	fmt.Sscanf(pidStr, "%d", &pid)
	if linesStr != "" {
		fmt.Sscanf(linesStr, "%d", &lines)
	}

	srv, ok := h.store.Get(serverID)
	if !ok {
		Fail(w, http.StatusNotFound, 404, "server not found")
		return
	}

	log, err := h.remoteCollector.GetProcessLog(toRemoteServer(srv), pid, lines)
	if err != nil {
		h.log.Errorw("get process log failed", "server", serverID, "pid", pid, "error", err)
		Fail(w, http.StatusInternalServerError, 500, "failed to get process log: "+err.Error())
		return
	}

	OK(w, map[string]string{
		"pid":     fmt.Sprintf("%d", pid),
		"server":  serverID,
		"log":     log,
	})
}

// --- 编程语言环境 ---

// handleLanguages: GET 列表 / POST 新增。
func (h *SoftwareHandler) handleLanguages(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		langStore := storage.NewLanguageStore(h.db)
		OK(w, langStore.ListLanguages())

	case http.MethodPost:
		var lang storage.LanguageRecord
		if err := decodeBody(r, &lang); err != nil {
			Fail(w, http.StatusBadRequest, 400, "invalid request body")
			return
		}
		if lang.ID == "" || lang.Name == "" {
			Fail(w, http.StatusBadRequest, 400, "id and name are required")
			return
		}
		langStore := storage.NewLanguageStore(h.db)
		if err := langStore.AddLanguage(&lang); err != nil {
			h.log.Errorw("add language failed", "error", err)
			Fail(w, http.StatusInternalServerError, 500, "failed to add language")
			return
		}
		OK(w, lang)

	default:
		Fail(w, http.StatusMethodNotAllowed, 405, "method not allowed")
	}
}

// handleLanguageDetail: PUT 更新 / DELETE 删除。
func (h *SoftwareHandler) handleLanguageDetail(w http.ResponseWriter, r *http.Request) {
	id := trimPrefix(r.URL.Path, "/api/v1/languages/")
	if id == "" {
		Fail(w, http.StatusBadRequest, 400, "language id required")
		return
	}

	switch r.Method {
	case http.MethodPut:
		var lang storage.LanguageRecord
		if err := decodeBody(r, &lang); err != nil {
			Fail(w, http.StatusBadRequest, 400, "invalid request body")
			return
		}
		lang.ID = id
		langStore := storage.NewLanguageStore(h.db)
		if err := langStore.UpdateLanguage(&lang); err != nil {
			h.log.Errorw("update language failed", "error", err)
			Fail(w, http.StatusInternalServerError, 500, "failed to update language")
			return
		}
		OK(w, lang)

	case http.MethodDelete:
		langStore := storage.NewLanguageStore(h.db)
		if err := langStore.DeleteLanguage(id); err != nil {
			h.log.Errorw("delete language failed", "error", err)
			Fail(w, http.StatusInternalServerError, 500, "failed to delete language")
			return
		}
		OK(w, map[string]string{"deleted": id})

	default:
		Fail(w, http.StatusMethodNotAllowed, 405, "method not allowed")
	}
}

// handleDetectLanguages: GET ?server=<id> 检测服务器已安装的语言。
func (h *SoftwareHandler) handleDetectLanguages(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		Fail(w, http.StatusMethodNotAllowed, 405, "method not allowed")
		return
	}

	serverID := r.URL.Query().Get("server")
	srv, ok := h.store.Get(serverID)
	if !ok {
		Fail(w, http.StatusNotFound, 404, "server not found")
		return
	}

	langStore := storage.NewLanguageStore(h.db)
	languages := langStore.ListLanguages()

	var detected []map[string]string
	for _, lang := range languages {
		if lang.VersionCmd == "" {
			continue
		}
		version := h.remoteCollector.DetectLanguage(toRemoteServer(srv), lang.VersionCmd)
		status := "not_installed"
		if version != "" {
			status = "installed"
			// 截断过长的版本字符串
			if len(version) > 100 {
				version = version[:97] + "..."
			}
		}
		detected = append(detected, map[string]string{
			"id":      lang.ID,
			"name":    lang.Name,
			"icon":    lang.Icon,
			"version": version,
			"status":  status,
		})
	}

	OK(w, detected)
}

// handleInstallLanguage: POST {"server_id": "...", "language_id": "..."} 安装语言环境。
func (h *SoftwareHandler) handleInstallLanguage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		Fail(w, http.StatusMethodNotAllowed, 405, "method not allowed")
		return
	}

	var req struct {
		ServerID   string `json:"server_id"`
		LanguageID string `json:"language_id"`
	}
	if err := decodeBody(r, &req); err != nil {
		Fail(w, http.StatusBadRequest, 400, "invalid request body")
		return
	}

	srv, ok := h.store.Get(req.ServerID)
	if !ok {
		Fail(w, http.StatusNotFound, 404, "server not found")
		return
	}

	langStore := storage.NewLanguageStore(h.db)
	lang, ok := langStore.GetLanguage(req.LanguageID)
	if !ok {
		Fail(w, http.StatusNotFound, 404, "language not found")
		return
	}
	if lang.InstallCmd == "" {
		Fail(w, http.StatusBadRequest, 400, "no install command defined")
		return
	}

	// 异步执行安装
	go func() {
		if err := h.remoteCollector.InstallLanguage(toRemoteServer(srv), lang.InstallCmd); err != nil {
			h.log.Errorw("install language failed", "lang", lang.ID, "error", err)
		}
	}()

	OK(w, map[string]string{"status": "installing", "language": lang.ID})
}

// --- 内部方法 ---

func toRemoteServer(srv *host.Server) *remote.Server {
	if srv == nil {
		return nil
	}
	return &remote.Server{
		ID:       srv.ID,
		Name:     srv.Name,
		Type:     srv.Type,
		Host:     srv.Host,
		Port:     srv.Port,
		Username: srv.Username,
		AuthType: srv.AuthType,
		Password: srv.Password,
		KeyPath:  srv.KeyPath,
	}
}

func (h *SoftwareHandler) getServerAndPackage(serverID, pkgID string) (*host.Server, *software.Package, error) {
	if serverID == "" || serverID == host.LocalServerID {
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

func decodeBody(r *http.Request, v any) error {
	return json.NewDecoder(r.Body).Decode(v)
}

func trimPrefix(s, prefix string) string {
	if len(s) > len(prefix) && s[:len(prefix)] == prefix {
		return s[len(prefix):]
	}
	return ""
}
