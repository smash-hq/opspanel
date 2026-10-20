package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/smash-hq/opspanel/internal/host"
	"github.com/smash-hq/opspanel/internal/logger"
	"github.com/smash-hq/opspanel/internal/notify"
	"github.com/smash-hq/opspanel/internal/remote"
	"github.com/smash-hq/opspanel/internal/software"
	"github.com/smash-hq/opspanel/internal/storage"
)

// pkgIDPattern 限制自定义软件包 ID 字符集，防止其拼入远程 shell 命令造成注入。
var pkgIDPattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._+-]*$`)

// SoftwareHandler 提供软件管理 API。
type SoftwareHandler struct {
	log             *logger.Logger
	store           *host.Store
	swStore         *storage.SoftwareStore
	manager         *software.Manager
	remoteCollector *remote.RemoteCollector
	db              *storage.DB
	notifications   *notify.Notifier
}

// NewSoftwareHandler 创建软件管理处理器。
func NewSoftwareHandler(log *logger.Logger, store *host.Store, swStore *storage.SoftwareStore, db *storage.DB) *SoftwareHandler {
	return &SoftwareHandler{
		log:             log.Named("api.software"),
		store:           store,
		swStore:         swStore,
		manager:         software.NewManager(log),
		remoteCollector: remote.NewRemoteCollector(log),
		db:              db,
		notifications:   notify.New(db, log),
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
	mux.HandleFunc("/api/v1/software/packages", h.handlePackages)       // GET 列表 / POST 新增
	mux.HandleFunc("/api/v1/software/packages/", h.handlePackageDetail) // PUT 更新 / DELETE 删除

	// 日志查看
	mux.HandleFunc("/api/v1/software/logs", h.handleLogs) // GET ?server=&package=&lines=

	// 进程管理
	mux.HandleFunc("/api/v1/processes", h.handleProcesses)        // GET ?server=&lines= 进程列表
	mux.HandleFunc("/api/v1/processes/kill", h.handleKillProcess) // POST 终止进程
	mux.HandleFunc("/api/v1/processes/log", h.handleProcessLog)   // GET ?server=&pid=&lines= 进程日志

	// 编程语言环境
	mux.HandleFunc("/api/v1/languages", h.handleLanguages)                   // GET 列表 / POST 新增
	mux.HandleFunc("/api/v1/languages/", h.handleLanguageDetail)             // PUT 更新 / DELETE 删除
	mux.HandleFunc("/api/v1/languages/detect", h.handleDetectLanguages)      // GET ?server= 检测已安装
	mux.HandleFunc("/api/v1/languages/install", h.handleInstallLanguage)     // POST 安装语言
	mux.HandleFunc("/api/v1/languages/uninstall", h.handleUninstallLanguage) // POST 卸载语言
	mux.HandleFunc("/api/v1/languages/job", h.handleLanguageJob)             // GET ?server_id=&language_id=&action= 任务进度

	// 镜像源管理
	mux.HandleFunc("/api/v1/software/sources", h.handleSources)       // GET 列表 / POST 新增
	mux.HandleFunc("/api/v1/software/sources/", h.handleSourceDetail) // PUT 更新 / DELETE 删除

	// 文件管理
	mux.HandleFunc("/api/v1/files", h.handleFiles)                 // GET ?server=&path= 列出目录
	mux.HandleFunc("/api/v1/files/info", h.handleFileInfo)         // GET ?server=&path= 文件详情
	mux.HandleFunc("/api/v1/files/preview", h.handleFilePreview)   // GET ?server=&path=&lines= 文件预览
	mux.HandleFunc("/api/v1/files/download", h.handleFileDownload) // GET ?server=&path= 下载文件

	// Docker 管理
	mux.HandleFunc("/api/v1/docker", h.handleDockerCheck)                           // GET ?server= Docker 环境检测
	mux.HandleFunc("/api/v1/docker/images", h.handleDockerImages)                   // GET ?server= 镜像列表
	mux.HandleFunc("/api/v1/docker/images/pull", h.handleDockerPull)                // POST 拉取镜像
	mux.HandleFunc("/api/v1/docker/images/pull/job", h.handleDockerPullJob)         // GET ?server=&image= 拉取任务进度
	mux.HandleFunc("/api/v1/docker/images/", h.handleDockerImageDetail)             // DELETE 删除镜像
	mux.HandleFunc("/api/v1/docker/images/batch-delete", h.handleBatchDeleteImages) // POST 批量删除镜像
	mux.HandleFunc("/api/v1/docker/run", h.handleDockerRun)                         // POST 一键启动容器
	mux.HandleFunc("/api/v1/docker/containers", h.handleDockerContainers)           // GET ?server=&all= 容器列表
	mux.HandleFunc("/api/v1/docker/containers/", h.handleDockerContainer)           // POST 操作/DELETE 删除
	mux.HandleFunc("/api/v1/docker/containers/batch", h.handleBatchContainerAction) // POST 批量容器操作
	mux.HandleFunc("/api/v1/docker/exec", h.handleDockerExec)                       // POST 执行命令
	mux.HandleFunc("/api/v1/docker/logs", h.handleDockerLogs)                       // GET ?server=&id=&lines= 容器日志
	mux.HandleFunc("/api/v1/docker/files", h.handleDockerFiles)                     // GET ?server=&container=&path=&op=list|preview|download 容器文件

	// Docker 镜像源管理
	mux.HandleFunc("/api/v1/docker-sources", h.handleDockerSources)       // GET 列表 / POST 新增
	mux.HandleFunc("/api/v1/docker-sources/", h.handleDockerSourceDetail) // PUT 更新 / DELETE 删除 / POST 设为默认

	// 服务器镜像源配置
	mux.HandleFunc("/api/v1/docker/mirror", h.handleDockerMirror) // GET 获取服务器当前镜像源 / POST 应用镜像源

	// 交互式终端（WebSocket）
	mux.HandleFunc("/api/v1/terminal", h.handleTerminal)
}

// --- 软件目录与安装 ---

// handleCatalog: GET 软件目录列表（从数据库读取）。
func (h *SoftwareHandler) handleCatalog(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		Fail(w, http.StatusMethodNotAllowed, 405, "请求方法不支持")
		return
	}
	OK(w, h.swStore.ListPackages(r.Context()))
}

// handleStatus: GET ?server=<id> 查询所有软件包的安装状态。
func (h *SoftwareHandler) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		Fail(w, http.StatusMethodNotAllowed, 405, "请求方法不支持")
		return
	}

	serverID := r.URL.Query().Get("server")
	if serverID == "" || serverID == host.LocalServerID {
		Fail(w, http.StatusBadRequest, 400, "该操作仅支持远程服务器")
		return
	}

	srv, ok := h.store.Get(r.Context(), serverID)
	if !ok {
		Fail(w, http.StatusNotFound, 404, "目标服务器不存在")
		return
	}

	var statuses []*software.SoftwareStatus
	for _, pkg := range h.swStore.ListPackages(r.Context()) {
		status := h.manager.CheckStatus(r.Context(), srv, pkg)
		statuses = append(statuses, status)
	}

	OK(w, statuses)
}

// handleInstall: POST 安装软件。
func (h *SoftwareHandler) handleInstall(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		Fail(w, http.StatusMethodNotAllowed, 405, "请求方法不支持")
		return
	}

	var req struct {
		ServerID  string `json:"server_id"`
		PackageID string `json:"package_id"`
		Version   string `json:"version"`
	}
	if err := decodeBody(r, &req); err != nil {
		FailErr(w, http.StatusBadRequest, "请求体解析失败", err)
		return
	}

	srv, pkg, err := h.getServerAndPackage(r.Context(), req.ServerID, req.PackageID)
	if err != nil {
		FailErr(w, http.StatusBadRequest, "目标服务器或软件包不存在", err)
		return
	}

	// 异步安装：handler 立即返回，不能用请求 ctx（会随响应结束被取消），
	// 超时由 Manager.Install 内部的长操作超时兜底。
	go func() {
		if err := h.manager.Install(context.Background(), srv, pkg, req.Version); err != nil {
			h.log.Errorw("install failed", "server", srv.ID, "pkg", pkg.ID, "error", err)
		}
	}()

	OK(w, map[string]string{"status": "installing", "package": pkg.ID})
}

// handleUninstall: POST 卸载软件。
func (h *SoftwareHandler) handleUninstall(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		Fail(w, http.StatusMethodNotAllowed, 405, "请求方法不支持")
		return
	}

	var req struct {
		ServerID  string `json:"server_id"`
		PackageID string `json:"package_id"`
		Confirm   bool   `json:"confirm"`
	}
	if err := decodeBody(r, &req); err != nil {
		FailErr(w, http.StatusBadRequest, "请求体解析失败", err)
		return
	}
	if !requireBodyConfirm(w, req.Confirm) {
		return
	}

	srv, pkg, err := h.getServerAndPackage(r.Context(), req.ServerID, req.PackageID)
	if err != nil {
		FailErr(w, http.StatusBadRequest, "目标服务器或软件包不存在", err)
		return
	}

	// 异步卸载：理由同 handleInstall。
	go func() {
		if err := h.manager.Uninstall(context.Background(), srv, pkg); err != nil {
			h.log.Errorw("uninstall failed", "server", srv.ID, "pkg", pkg.ID, "error", err)
		}
	}()

	OK(w, map[string]string{"status": "uninstalling", "package": pkg.ID})
}

// handleService: POST 服务启停。
func (h *SoftwareHandler) handleService(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		Fail(w, http.StatusMethodNotAllowed, 405, "请求方法不支持")
		return
	}

	var req struct {
		ServerID  string `json:"server_id"`
		PackageID string `json:"package_id"`
		Action    string `json:"action"`
		Confirm   bool   `json:"confirm"`
	}
	if err := decodeBody(r, &req); err != nil {
		FailErr(w, http.StatusBadRequest, "请求体解析失败", err)
		return
	}
	if !requireBodyConfirm(w, req.Confirm) {
		return
	}

	if req.Action != "start" && req.Action != "stop" && req.Action != "restart" {
		Fail(w, http.StatusBadRequest, 400, "参数不合法：action")
		return
	}

	srv, pkg, err := h.getServerAndPackage(r.Context(), req.ServerID, req.PackageID)
	if err != nil {
		FailErr(w, http.StatusBadRequest, "目标服务器或软件包不存在", err)
		return
	}

	if err := h.manager.ServiceControl(r.Context(), srv, pkg, req.Action); err != nil {
		h.log.Errorw("service control failed", "server", srv.ID, "pkg", pkg.ID, "action", req.Action, "error", err)
		FailErr(w, http.StatusInternalServerError, "服务控制操作执行失败", err)
		return
	}

	OK(w, map[string]string{"status": "ok", "action": req.Action, "package": pkg.ID})
}

// --- 软件包管理 ---

// handlePackages: GET 列表 / POST 新增。
func (h *SoftwareHandler) handlePackages(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		OK(w, h.swStore.ListPackages(r.Context()))

	case http.MethodPost:
		var pkg software.Package
		if err := decodeBody(r, &pkg); err != nil {
			FailErr(w, http.StatusBadRequest, "请求体解析失败", err)
			return
		}
		if pkg.ID == "" || pkg.Name == "" {
			Fail(w, http.StatusBadRequest, 400, "缺少必填参数：id、name")
			return
		}
		if !pkgIDPattern.MatchString(pkg.ID) {
			Fail(w, http.StatusBadRequest, 400, "参数不合法：软件包 ID")
			return
		}
		if err := h.swStore.AddPackage(r.Context(), &pkg); err != nil {
			h.log.Errorw("add package failed", "error", err)
			FailErr(w, http.StatusInternalServerError, "添加软件包失败", err)
			return
		}
		// 刷新内存索引
		software.LoadFromStore(r.Context(), h.swStore)
		OK(w, pkg)

	default:
		Fail(w, http.StatusMethodNotAllowed, 405, "请求方法不支持")
	}
}

// handlePackageDetail: PUT 更新 / DELETE 删除。
func (h *SoftwareHandler) handlePackageDetail(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/v1/software/packages/")
	if id == "" {
		Fail(w, http.StatusBadRequest, 400, "缺少必填参数：软件包 ID")
		return
	}
	if !pkgIDPattern.MatchString(id) {
		Fail(w, http.StatusBadRequest, 400, "参数不合法：软件包 ID")
		return
	}

	switch r.Method {
	case http.MethodPut:
		var pkg software.Package
		if err := decodeBody(r, &pkg); err != nil {
			FailErr(w, http.StatusBadRequest, "请求体解析失败", err)
			return
		}
		pkg.ID = id
		if err := h.swStore.UpdatePackage(r.Context(), &pkg); err != nil {
			h.log.Errorw("update package failed", "error", err)
			FailErr(w, http.StatusInternalServerError, "更新软件包失败", err)
			return
		}
		software.LoadFromStore(r.Context(), h.swStore)
		OK(w, pkg)

	case http.MethodDelete:
		if err := h.swStore.DeletePackage(r.Context(), id); err != nil {
			h.log.Errorw("delete package failed", "error", err)
			FailErr(w, http.StatusInternalServerError, "删除软件包失败", err)
			return
		}
		software.LoadFromStore(r.Context(), h.swStore)
		OK(w, map[string]string{"deleted": id})

	default:
		Fail(w, http.StatusMethodNotAllowed, 405, "请求方法不支持")
	}
}

// --- 镜像源管理 ---

// handleSources: GET 列表 / POST 新增。
func (h *SoftwareHandler) handleSources(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		OK(w, h.swStore.ListSources(r.Context()))

	case http.MethodPost:
		var src storage.SourceRecord
		if err := decodeBody(r, &src); err != nil {
			FailErr(w, http.StatusBadRequest, "请求体解析失败", err)
			return
		}
		if src.ID == "" || src.Name == "" || src.URL == "" {
			Fail(w, http.StatusBadRequest, 400, "缺少必填参数：id、name、url")
			return
		}
		if err := h.swStore.AddSource(r.Context(), &src); err != nil {
			h.log.Errorw("add source failed", "error", err)
			FailErr(w, http.StatusInternalServerError, "添加软件源失败", err)
			return
		}
		OK(w, src)

	default:
		Fail(w, http.StatusMethodNotAllowed, 405, "请求方法不支持")
	}
}

// handleSourceDetail: PUT 更新 / DELETE 删除。
func (h *SoftwareHandler) handleSourceDetail(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/v1/software/sources/")
	if id == "" {
		Fail(w, http.StatusBadRequest, 400, "缺少必填参数：软件源 ID")
		return
	}

	switch r.Method {
	case http.MethodPut:
		var src storage.SourceRecord
		if err := decodeBody(r, &src); err != nil {
			FailErr(w, http.StatusBadRequest, "请求体解析失败", err)
			return
		}
		src.ID = id
		if err := h.swStore.UpdateSource(r.Context(), &src); err != nil {
			h.log.Errorw("update source failed", "error", err)
			FailErr(w, http.StatusInternalServerError, "更新软件源失败", err)
			return
		}
		OK(w, src)

	case http.MethodDelete:
		if err := h.swStore.DeleteSource(r.Context(), id); err != nil {
			h.log.Errorw("delete source failed", "error", err)
			FailErr(w, http.StatusInternalServerError, "删除软件源失败", err)
			return
		}
		OK(w, map[string]string{"deleted": id})

	default:
		Fail(w, http.StatusMethodNotAllowed, 405, "请求方法不支持")
	}
}

// --- 日志查看 ---

// handleLogs: GET ?server=<id>&package=<id>&lines=<n> 获取软件运行日志。
func (h *SoftwareHandler) handleLogs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		Fail(w, http.StatusMethodNotAllowed, 405, "请求方法不支持")
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

	srv, ok := h.store.Get(r.Context(), serverID)
	if !ok {
		Fail(w, http.StatusNotFound, 404, "目标服务器不存在")
		return
	}

	pkg, ok := software.Get(pkgID)
	if !ok {
		Fail(w, http.StatusNotFound, 404, "软件包不存在")
		return
	}

	log, err := h.manager.GetLog(r.Context(), srv, pkg.ID, lines)
	if err != nil {
		h.log.Errorw("get log failed", "server", serverID, "pkg", pkgID, "error", err)
		FailErr(w, http.StatusInternalServerError, "获取安装日志失败", err)
		return
	}

	OK(w, map[string]string{
		"package": pkgID,
		"server":  serverID,
		"log":     log,
	})
}

// --- Docker 管理 ---

// handleDockerCheck: GET ?server=<id> 检测 Docker 环境。
func (h *SoftwareHandler) handleDockerCheck(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		Fail(w, http.StatusMethodNotAllowed, 405, "请求方法不支持")
		return
	}
	serverID := r.URL.Query().Get("server")
	srv, ok := h.store.Get(r.Context(), serverID)
	if !ok {
		Fail(w, http.StatusNotFound, 404, "目标服务器不存在")
		return
	}
	available, version := h.remoteCollector.CheckDocker(r.Context(), toRemoteServer(srv))
	OK(w, map[string]any{"available": available, "version": version})
}

// handleDockerImages: GET ?server=<id> 列出镜像。
func (h *SoftwareHandler) handleDockerImages(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		Fail(w, http.StatusMethodNotAllowed, 405, "请求方法不支持")
		return
	}
	serverID := r.URL.Query().Get("server")
	srv, ok := h.store.Get(r.Context(), serverID)
	if !ok {
		Fail(w, http.StatusNotFound, 404, "目标服务器不存在")
		return
	}
	images, err := h.remoteCollector.ListImages(r.Context(), toRemoteServer(srv))
	if err != nil {
		FailErr(w, http.StatusInternalServerError, "获取镜像列表失败", err)
		return
	}
	OK(w, images)
}

// handleDockerPull: POST {"server_id": "...", "image": "..."} 拉取镜像。
// 任务在服务端异步执行，输出实时写入任务日志（GET /docker/images/pull/job 轮询展示进度），
// 结束后无论成功失败均写入站内通知（启用邮件通知时会同步发邮件）。
func (h *SoftwareHandler) handleDockerPull(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		Fail(w, http.StatusMethodNotAllowed, 405, "请求方法不支持")
		return
	}
	var req struct {
		ServerID string `json:"server_id"`
		Image    string `json:"image"`
	}
	if err := decodeBody(r, &req); err != nil {
		FailErr(w, http.StatusBadRequest, "请求体解析失败", err)
		return
	}
	srv, ok := h.store.Get(r.Context(), req.ServerID)
	if !ok {
		Fail(w, http.StatusNotFound, 404, "目标服务器不存在")
		return
	}
	job := pullJobs.start(req.ServerID, req.Image, "docker pull "+req.Image)
	// 异步拉取：理由同 handleInstall。
	go func() {
		err := h.remoteCollector.PullImageStream(context.Background(), toRemoteServer(srv), req.Image, job.append)
		job.finish(err)
		// 成败都通知：失败为 error 级，成功为 info 级
		node := srv.Name
		if node == "" {
			node = srv.Host
		}
		if err != nil {
			h.log.Errorw("docker pull failed", "image", req.Image, "error", err)
			if nerr := h.notifications.Add(context.Background(), "error", "docker",
				fmt.Sprintf("镜像拉取失败：%s（节点 %s）", req.Image, node), err.Error()); nerr != nil {
				h.log.Errorw("add notification", "image", req.Image, "error", nerr)
			}
			return
		}
		if nerr := h.notifications.Add(context.Background(), "info", "docker",
			fmt.Sprintf("镜像拉取成功：%s（节点 %s）", req.Image, node), "镜像已可用，可刷新镜像列表查看"); nerr != nil {
			h.log.Errorw("add notification", "image", req.Image, "error", nerr)
		}
	}()
	OK(w, map[string]string{"status": "pulling", "image": req.Image})
}

// handleDockerPullJob: GET ?server=&image= 查询镜像拉取任务的状态与实时输出。
func (h *SoftwareHandler) handleDockerPullJob(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		Fail(w, http.StatusMethodNotAllowed, 405, "请求方法不支持")
		return
	}
	serverID := r.URL.Query().Get("server")
	image := r.URL.Query().Get("image")
	if serverID == "" || image == "" {
		Fail(w, http.StatusBadRequest, 400, "缺少必填参数：server、image")
		return
	}
	job, err := pullJobs.get(serverID, image)
	if err != nil {
		// 任务不存在（服务重启或从未发起）：返回 none，前端据此提示
		OK(w, map[string]any{"action": "pull", "status": "none", "log": "", "error": ""})
		return
	}
	OK(w, job.snapshot())
}

// handleDockerRun: POST {"server_id": "...", "image": "...", ...} 基于已有镜像一键启动容器。
func (h *SoftwareHandler) handleDockerRun(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		Fail(w, http.StatusMethodNotAllowed, 405, "请求方法不支持")
		return
	}
	var req struct {
		ServerID string `json:"server_id"`
		Image    string `json:"image"`
		remote.DockerRunOptions
	}
	if err := decodeBody(r, &req); err != nil {
		FailErr(w, http.StatusBadRequest, "请求体解析失败", err)
		return
	}
	if strings.TrimSpace(req.Image) == "" {
		Fail(w, http.StatusBadRequest, 400, "缺少必填参数：image")
		return
	}
	srv, ok := h.store.Get(r.Context(), req.ServerID)
	if !ok {
		Fail(w, http.StatusNotFound, 404, "目标服务器不存在")
		return
	}
	containerID, err := h.remoteCollector.RunContainer(r.Context(), toRemoteServer(srv), req.Image, req.DockerRunOptions)
	if err != nil {
		FailErr(w, http.StatusInternalServerError, "容器启动失败", err)
		return
	}
	OK(w, map[string]string{"status": "started", "container_id": containerID})
}

// handleDockerImageDetail: DELETE /api/v1/docker/images/{id}?server=<id>&confirm=true 删除镜像。
func (h *SoftwareHandler) handleDockerImageDetail(w http.ResponseWriter, r *http.Request) {
	// 严格限制方法：该端点执行的是删除操作，绝不能让 GET 等方法落入删除分支
	if r.Method != http.MethodDelete {
		Fail(w, http.StatusMethodNotAllowed, 405, "请求方法不支持")
		return
	}
	if !requireQueryConfirm(w, r) {
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/api/v1/docker/images/")
	if id == "" {
		Fail(w, http.StatusBadRequest, 400, "缺少必填参数：镜像 ID")
		return
	}
	serverID := r.URL.Query().Get("server")
	srv, ok := h.store.Get(r.Context(), serverID)
	if !ok {
		Fail(w, http.StatusNotFound, 404, "目标服务器不存在")
		return
	}
	if err := h.remoteCollector.RemoveImage(r.Context(), toRemoteServer(srv), id); err != nil {
		FailErr(w, http.StatusInternalServerError, "删除镜像失败", err)
		return
	}
	OK(w, map[string]string{"deleted": id})
}

// handleDockerContainers: GET ?server=<id>&all=<bool> 列出容器。
func (h *SoftwareHandler) handleDockerContainers(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		Fail(w, http.StatusMethodNotAllowed, 405, "请求方法不支持")
		return
	}
	serverID := r.URL.Query().Get("server")
	all := r.URL.Query().Get("all") == "true"
	srv, ok := h.store.Get(r.Context(), serverID)
	if !ok {
		Fail(w, http.StatusNotFound, 404, "目标服务器不存在")
		return
	}
	containers, err := h.remoteCollector.ListContainers(r.Context(), toRemoteServer(srv), all)
	if err != nil {
		FailErr(w, http.StatusInternalServerError, "获取容器列表失败", err)
		return
	}
	OK(w, containers)
}

// handleDockerContainer: POST 操作容器 / DELETE 删除容器。
func (h *SoftwareHandler) handleDockerContainer(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/docker/containers/")
	parts := strings.SplitN(path, "/", 2)
	if len(parts) < 2 {
		Fail(w, http.StatusBadRequest, 400, "缺少必填参数：容器 ID 与操作")
		return
	}
	id, action := parts[0], parts[1]
	serverID := r.URL.Query().Get("server")
	srv, ok := h.store.Get(r.Context(), serverID)
	if !ok {
		Fail(w, http.StatusNotFound, 404, "目标服务器不存在")
		return
	}

	switch r.Method {
	case http.MethodPost:
		switch action {
		case "start", "stop", "restart", "kill":
		default:
			Fail(w, http.StatusBadRequest, 400, "参数不合法：action")
			return
		}
		// stop/restart/kill 影响运行中的服务：要求显式二次确认
		if action != "start" && !requireQueryConfirm(w, r) {
			return
		}
		if err := h.remoteCollector.ContainerAction(r.Context(), toRemoteServer(srv), id, action); err != nil {
			FailErr(w, http.StatusInternalServerError, "容器操作执行失败", err)
			return
		}
		OK(w, map[string]string{"status": "ok", "action": action})

	case http.MethodDelete:
		if !requireQueryConfirm(w, r) {
			return
		}
		if err := h.remoteCollector.RemoveContainer(r.Context(), toRemoteServer(srv), id); err != nil {
			FailErr(w, http.StatusInternalServerError, "删除容器失败", err)
			return
		}
		OK(w, map[string]string{"deleted": id})

	default:
		Fail(w, http.StatusMethodNotAllowed, 405, "请求方法不支持")
	}
}

// handleDockerExec: POST {"server_id": "...", "container_id": "...", "command": "..."} 执行命令。
func (h *SoftwareHandler) handleDockerExec(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		Fail(w, http.StatusMethodNotAllowed, 405, "请求方法不支持")
		return
	}
	var req struct {
		ServerID  string `json:"server_id"`
		Container string `json:"container_id"`
		Command   string `json:"command"`
	}
	if err := decodeBody(r, &req); err != nil {
		FailErr(w, http.StatusBadRequest, "请求体解析失败", err)
		return
	}
	srv, ok := h.store.Get(r.Context(), req.ServerID)
	if !ok {
		Fail(w, http.StatusNotFound, 404, "目标服务器不存在")
		return
	}
	output, err := h.remoteCollector.ExecInContainer(r.Context(), toRemoteServer(srv), req.Container, req.Command)
	if err != nil {
		FailErr(w, http.StatusInternalServerError, "容器命令执行失败", err)
		return
	}
	OK(w, map[string]string{"output": output})
}

// handleDockerLogs: GET ?server=&id=&lines= 容器日志。
func (h *SoftwareHandler) handleDockerLogs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		Fail(w, http.StatusMethodNotAllowed, 405, "请求方法不支持")
		return
	}
	serverID := r.URL.Query().Get("server")
	id := r.URL.Query().Get("id")
	lines := 100
	if l := r.URL.Query().Get("lines"); l != "" {
		fmt.Sscanf(l, "%d", &lines)
	}
	srv, ok := h.store.Get(r.Context(), serverID)
	if !ok {
		Fail(w, http.StatusNotFound, 404, "目标服务器不存在")
		return
	}
	logs, err := h.remoteCollector.ContainerLogs(r.Context(), toRemoteServer(srv), id, lines)
	if err != nil {
		FailErr(w, http.StatusInternalServerError, "获取容器日志失败", err)
		return
	}
	OK(w, map[string]string{"id": id, "logs": logs})
}

// --- 文件管理 ---

// handleFiles: GET ?server=<id>&path=<path> 列出目录内容。
func (h *SoftwareHandler) handleFiles(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		Fail(w, http.StatusMethodNotAllowed, 405, "请求方法不支持")
		return
	}

	serverID := r.URL.Query().Get("server")
	path := r.URL.Query().Get("path")
	if path == "" {
		path = "/"
	}

	srv, ok := h.store.Get(r.Context(), serverID)
	if !ok {
		Fail(w, http.StatusNotFound, 404, "目标服务器不存在")
		return
	}

	result, err := h.remoteCollector.ListFiles(r.Context(), toRemoteServer(srv), path)
	if err != nil {
		h.log.Errorw("list files failed", "server", serverID, "path", path, "error", err)
		FailErr(w, http.StatusInternalServerError, "读取目录失败", err)
		return
	}

	OK(w, result)
}

// handleFileInfo: GET ?server=<id>&path=<path> 获取文件详情。
func (h *SoftwareHandler) handleFileInfo(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		Fail(w, http.StatusMethodNotAllowed, 405, "请求方法不支持")
		return
	}

	serverID := r.URL.Query().Get("server")
	path := r.URL.Query().Get("path")
	if path == "" {
		Fail(w, http.StatusBadRequest, 400, "缺少必填参数：path")
		return
	}

	srv, ok := h.store.Get(r.Context(), serverID)
	if !ok {
		Fail(w, http.StatusNotFound, 404, "目标服务器不存在")
		return
	}

	info, err := h.remoteCollector.GetFileInfo(r.Context(), toRemoteServer(srv), path)
	if err != nil {
		h.log.Errorw("get file info failed", "server", serverID, "path", path, "error", err)
		FailErr(w, http.StatusInternalServerError, "读取文件信息失败", err)
		return
	}

	OK(w, info)
}

// handleFilePreview: GET ?server=<id>&path=<path>&lines=<n> 读取文件内容用于预览。
func (h *SoftwareHandler) handleFilePreview(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		Fail(w, http.StatusMethodNotAllowed, 405, "请求方法不支持")
		return
	}

	serverID := r.URL.Query().Get("server")
	path := r.URL.Query().Get("path")
	linesStr := r.URL.Query().Get("lines")
	lines := 100
	if linesStr != "" {
		fmt.Sscanf(linesStr, "%d", &lines)
	}

	srv, ok := h.store.Get(r.Context(), serverID)
	if !ok {
		Fail(w, http.StatusNotFound, 404, "目标服务器不存在")
		return
	}

	content, err := h.remoteCollector.ReadFileContent(r.Context(), toRemoteServer(srv), path, lines)
	if err != nil {
		h.log.Errorw("read file failed", "server", serverID, "path", path, "error", err)
		FailErr(w, http.StatusInternalServerError, "读取文件失败", err)
		return
	}

	OK(w, map[string]string{"path": path, "content": content})
}

// handleFileDownload: GET ?server=<id>&path=<path> 下载文件。
func (h *SoftwareHandler) handleFileDownload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		Fail(w, http.StatusMethodNotAllowed, 405, "请求方法不支持")
		return
	}

	serverID := r.URL.Query().Get("server")
	path := r.URL.Query().Get("path")
	if path == "" {
		Fail(w, http.StatusBadRequest, 400, "缺少必填参数：path")
		return
	}

	srv, ok := h.store.Get(r.Context(), serverID)
	if !ok {
		Fail(w, http.StatusNotFound, 404, "目标服务器不存在")
		return
	}

	data, filename, err := h.remoteCollector.DownloadFile(r.Context(), toRemoteServer(srv), path)
	if err != nil {
		h.log.Errorw("download file failed", "server", serverID, "path", path, "error", err)
		FailErr(w, http.StatusInternalServerError, "下载文件失败", err)
		return
	}

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(data)))
	w.Write(data)
}

// handleDockerFiles: GET ?server=<id>&container=<name>&path=<path>&op=list|preview|download
// 容器内文件浏览/预览/下载（docker exec 实现，详见 remote 层）。
func (h *SoftwareHandler) handleDockerFiles(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		Fail(w, http.StatusMethodNotAllowed, 405, "请求方法不支持")
		return
	}

	q := r.URL.Query()
	serverID := q.Get("server")
	container := q.Get("container")
	path := q.Get("path")
	op := q.Get("op")
	if op == "" {
		op = "list"
	}
	if container == "" {
		Fail(w, http.StatusBadRequest, 400, "缺少必填参数：container")
		return
	}
	if path == "" {
		path = "/"
	}

	srv, ok := h.store.Get(r.Context(), serverID)
	if !ok {
		Fail(w, http.StatusNotFound, 404, "目标服务器不存在")
		return
	}

	switch op {
	case "list":
		result, err := h.remoteCollector.ListContainerFiles(r.Context(), toRemoteServer(srv), container, path)
		if err != nil {
			h.log.Errorw("list container files failed", "server", serverID, "container", container, "path", path, "error", err)
			FailErr(w, http.StatusInternalServerError, "列出容器文件失败", err)
			return
		}
		OK(w, result)
	case "preview":
		content, err := h.remoteCollector.ReadContainerFile(r.Context(), toRemoteServer(srv), container, path)
		if err != nil {
			h.log.Errorw("read container file failed", "server", serverID, "container", container, "path", path, "error", err)
			FailErr(w, http.StatusInternalServerError, "读取容器文件失败", err)
			return
		}
		OK(w, map[string]string{"path": path, "content": content})
	case "download":
		data, filename, err := h.remoteCollector.DownloadContainerFile(r.Context(), toRemoteServer(srv), container, path)
		if err != nil {
			h.log.Errorw("download container file failed", "server", serverID, "container", container, "path", path, "error", err)
			FailErr(w, http.StatusInternalServerError, "下载容器文件失败", err)
			return
		}
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(data)))
		w.Write(data)
	default:
		Fail(w, http.StatusBadRequest, 400, "op 仅支持 list|preview|download")
	}
}

// --- Docker 批量操作 ---

// handleBatchDeleteImages: POST {"server_id": "...", "ids": [...], "confirm": true} 批量删除镜像。
func (h *SoftwareHandler) handleBatchDeleteImages(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		Fail(w, http.StatusMethodNotAllowed, 405, "请求方法不支持")
		return
	}
	var req struct {
		ServerID string   `json:"server_id"`
		IDs      []string `json:"ids"`
		Confirm  bool     `json:"confirm"`
	}
	if err := decodeBody(r, &req); err != nil {
		FailErr(w, http.StatusBadRequest, "请求体解析失败", err)
		return
	}
	if !requireBodyConfirm(w, req.Confirm) {
		return
	}
	srv, ok := h.store.Get(r.Context(), req.ServerID)
	if !ok {
		Fail(w, http.StatusNotFound, 404, "目标服务器不存在")
		return
	}
	var deleted []string
	var errors []string
	for _, id := range req.IDs {
		if err := h.remoteCollector.RemoveImage(r.Context(), toRemoteServer(srv), id); err != nil {
			errors = append(errors, fmt.Sprintf("%s: %s", id, err.Error()))
		} else {
			deleted = append(deleted, id)
		}
	}
	OK(w, map[string]any{"deleted": deleted, "errors": errors})
}

// handleBatchContainerAction: POST {"server_id": "...", "ids": [...], "action": "stop/start/restart", "confirm": true} 批量容器操作。
func (h *SoftwareHandler) handleBatchContainerAction(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		Fail(w, http.StatusMethodNotAllowed, 405, "请求方法不支持")
		return
	}
	var req struct {
		ServerID string   `json:"server_id"`
		IDs      []string `json:"ids"`
		Action   string   `json:"action"`
		Confirm  bool     `json:"confirm"`
	}
	if err := decodeBody(r, &req); err != nil {
		FailErr(w, http.StatusBadRequest, "请求体解析失败", err)
		return
	}
	if !requireBodyConfirm(w, req.Confirm) {
		return
	}
	if req.Action != "stop" && req.Action != "start" && req.Action != "restart" && req.Action != "rm" {
		Fail(w, http.StatusBadRequest, 400, "参数不合法：action")
		return
	}
	srv, ok := h.store.Get(r.Context(), req.ServerID)
	if !ok {
		Fail(w, http.StatusNotFound, 404, "目标服务器不存在")
		return
	}
	var done []string
	var errors []string
	for _, id := range req.IDs {
		var err error
		if req.Action == "rm" {
			err = h.remoteCollector.RemoveContainer(r.Context(), toRemoteServer(srv), id)
		} else {
			err = h.remoteCollector.ContainerAction(r.Context(), toRemoteServer(srv), id, req.Action)
		}
		if err != nil {
			errors = append(errors, fmt.Sprintf("%s: %s", id, err.Error()))
		} else {
			done = append(done, id)
		}
	}
	OK(w, map[string]any{"done": done, "errors": errors})
}

// --- Docker 镜像源管理 ---

// handleDockerSources: GET 列表 / POST 新增。
func (h *SoftwareHandler) handleDockerSources(w http.ResponseWriter, r *http.Request) {
	sourceStore := storage.NewDockerSourceStore(h.db)
	switch r.Method {
	case http.MethodGet:
		OK(w, sourceStore.ListSources(r.Context()))

	case http.MethodPost:
		var src storage.DockerSource
		if err := decodeBody(r, &src); err != nil {
			FailErr(w, http.StatusBadRequest, "请求体解析失败", err)
			return
		}
		if src.ID == "" || src.Name == "" || src.URL == "" {
			Fail(w, http.StatusBadRequest, 400, "缺少必填参数：id、name、url")
			return
		}
		if err := sourceStore.AddSource(r.Context(), &src); err != nil {
			FailErr(w, http.StatusInternalServerError, "添加软件源失败", err)
			return
		}
		OK(w, src)

	default:
		Fail(w, http.StatusMethodNotAllowed, 405, "请求方法不支持")
	}
}

// handleDockerSourceDetail: PUT 更新 / DELETE 删除 / POST 设为默认。
func (h *SoftwareHandler) handleDockerSourceDetail(w http.ResponseWriter, r *http.Request) {
	sourceStore := storage.NewDockerSourceStore(h.db)
	id := strings.TrimPrefix(r.URL.Path, "/api/v1/docker-sources/")
	if id == "" {
		Fail(w, http.StatusBadRequest, 400, "缺少必填参数：软件源 ID")
		return
	}

	switch r.Method {
	case http.MethodPut:
		var src storage.DockerSource
		if err := decodeBody(r, &src); err != nil {
			FailErr(w, http.StatusBadRequest, "请求体解析失败", err)
			return
		}
		src.ID = id
		if err := sourceStore.UpdateSource(r.Context(), &src); err != nil {
			FailErr(w, http.StatusInternalServerError, "更新软件源失败", err)
			return
		}
		OK(w, src)

	case http.MethodDelete:
		if err := sourceStore.DeleteSource(r.Context(), id); err != nil {
			FailErr(w, http.StatusInternalServerError, "删除软件源失败", err)
			return
		}
		OK(w, map[string]string{"deleted": id})

	case http.MethodPost:
		// 设为默认
		if err := sourceStore.SetDefaultSource(r.Context(), id); err != nil {
			FailErr(w, http.StatusInternalServerError, "设置默认软件源失败", err)
			return
		}
		OK(w, map[string]string{"default": id})

	default:
		Fail(w, http.StatusMethodNotAllowed, 405, "请求方法不支持")
	}
}

// --- 服务器镜像源配置 ---

// handleDockerMirror: GET 获取服务器当前镜像源 / POST 应用镜像源到服务器。
func (h *SoftwareHandler) handleDockerMirror(w http.ResponseWriter, r *http.Request) {
	serverID := r.URL.Query().Get("server")
	if serverID == "" {
		Fail(w, http.StatusBadRequest, 400, "缺少必填参数：server")
		return
	}

	srv, ok := h.store.Get(r.Context(), serverID)
	if !ok {
		Fail(w, http.StatusNotFound, 404, "目标服务器不存在")
		return
	}

	switch r.Method {
	case http.MethodGet:
		config, err := h.remoteCollector.GetMirrorSources(r.Context(), toRemoteServer(srv))
		if err != nil {
			FailErr(w, http.StatusInternalServerError, "获取镜像源配置失败", err)
			return
		}
		OK(w, config)

	case http.MethodPost:
		// 改写远程 daemon.json 并重启 Docker，属于影响面较大的操作：要求二次确认
		var req struct {
			Mirror  string `json:"mirror"`
			Confirm bool   `json:"confirm"`
		}
		if err := decodeBody(r, &req); err != nil {
			FailErr(w, http.StatusBadRequest, "请求体解析失败", err)
			return
		}
		if !requireBodyConfirm(w, req.Confirm) {
			return
		}
		if req.Mirror == "" {
			Fail(w, http.StatusBadRequest, 400, "缺少必填参数：镜像源地址")
			return
		}
		if err := h.remoteCollector.ApplyMirrorSource(r.Context(), toRemoteServer(srv), req.Mirror); err != nil {
			h.log.Errorw("apply mirror failed", "server", serverID, "error", err)
			FailErr(w, http.StatusInternalServerError, "应用镜像源配置失败", err)
			return
		}
		OK(w, map[string]string{"status": "ok", "mirror": req.Mirror})

	default:
		Fail(w, http.StatusMethodNotAllowed, 405, "请求方法不支持")
	}
}

// --- 进程管理 ---

// handleProcesses: GET ?server=<id>&lines=<n> 获取进程列表。
func (h *SoftwareHandler) handleProcesses(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		Fail(w, http.StatusMethodNotAllowed, 405, "请求方法不支持")
		return
	}

	serverID := r.URL.Query().Get("server")
	linesStr := r.URL.Query().Get("lines")
	lines := 50
	if linesStr != "" {
		fmt.Sscanf(linesStr, "%d", &lines)
	}

	srv, ok := h.store.Get(r.Context(), serverID)
	if !ok {
		Fail(w, http.StatusNotFound, 404, "目标服务器不存在")
		return
	}

	processes, err := h.remoteCollector.ListProcesses(r.Context(), toRemoteServer(srv), lines)
	if err != nil {
		h.log.Errorw("list processes failed", "server", serverID, "error", err)
		FailErr(w, http.StatusInternalServerError, "获取进程列表失败", err)
		return
	}

	OK(w, processes)
}

// handleKillProcess: POST {"server_id": "...", "pid": 123, "force": false, "confirm": true} 终止进程。
func (h *SoftwareHandler) handleKillProcess(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		Fail(w, http.StatusMethodNotAllowed, 405, "请求方法不支持")
		return
	}

	var req struct {
		ServerID string `json:"server_id"`
		PID      int    `json:"pid"`
		Force    bool   `json:"force"`
		Confirm  bool   `json:"confirm"`
	}
	if err := decodeBody(r, &req); err != nil {
		FailErr(w, http.StatusBadRequest, "请求体解析失败", err)
		return
	}
	if !requireBodyConfirm(w, req.Confirm) {
		return
	}
	// PID 合法性校验：0 会作用于整个进程组、1 是 init，负数非法 —— 绝不能对目标服务器执行
	if req.PID <= 1 {
		Fail(w, http.StatusBadRequest, 400, "参数不合法：pid 必须为大于 1 的进程 ID")
		return
	}

	srv, ok := h.store.Get(r.Context(), req.ServerID)
	if !ok {
		Fail(w, http.StatusNotFound, 404, "目标服务器不存在")
		return
	}

	if err := h.remoteCollector.KillProcess(r.Context(), toRemoteServer(srv), req.PID, req.Force); err != nil {
		h.log.Errorw("kill process failed", "server", req.ServerID, "pid", req.PID, "error", err)
		FailErr(w, http.StatusInternalServerError, "终止进程失败", err)
		return
	}

	OK(w, map[string]string{"status": "ok", "pid": fmt.Sprintf("%d", req.PID)})
}

// handleProcessLog: GET ?server=<id>&pid=<pid>&lines=<n> 获取进程日志。
func (h *SoftwareHandler) handleProcessLog(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		Fail(w, http.StatusMethodNotAllowed, 405, "请求方法不支持")
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

	srv, ok := h.store.Get(r.Context(), serverID)
	if !ok {
		Fail(w, http.StatusNotFound, 404, "目标服务器不存在")
		return
	}

	log, err := h.remoteCollector.GetProcessLog(r.Context(), toRemoteServer(srv), pid, lines)
	if err != nil {
		h.log.Errorw("get process log failed", "server", serverID, "pid", pid, "error", err)
		FailErr(w, http.StatusInternalServerError, "获取进程日志失败", err)
		return
	}

	OK(w, map[string]string{
		"pid":    fmt.Sprintf("%d", pid),
		"server": serverID,
		"log":    log,
	})
}

// --- 编程语言环境 ---

// handleLanguages: GET 列表 / POST 新增。
func (h *SoftwareHandler) handleLanguages(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		langStore := storage.NewLanguageStore(h.db)
		OK(w, langStore.ListLanguages(r.Context()))

	case http.MethodPost:
		var lang storage.LanguageRecord
		if err := decodeBody(r, &lang); err != nil {
			FailErr(w, http.StatusBadRequest, "请求体解析失败", err)
			return
		}
		if lang.ID == "" || lang.Name == "" {
			Fail(w, http.StatusBadRequest, 400, "缺少必填参数：id、name")
			return
		}
		langStore := storage.NewLanguageStore(h.db)
		if err := langStore.AddLanguage(r.Context(), &lang); err != nil {
			h.log.Errorw("add language failed", "error", err)
			FailErr(w, http.StatusInternalServerError, "添加语言环境失败", err)
			return
		}
		OK(w, lang)

	default:
		Fail(w, http.StatusMethodNotAllowed, 405, "请求方法不支持")
	}
}

// handleLanguageDetail: PUT 更新 / DELETE 删除。
func (h *SoftwareHandler) handleLanguageDetail(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/v1/languages/")
	if id == "" {
		Fail(w, http.StatusBadRequest, 400, "缺少必填参数：语言 ID")
		return
	}

	switch r.Method {
	case http.MethodPut:
		var lang storage.LanguageRecord
		if err := decodeBody(r, &lang); err != nil {
			FailErr(w, http.StatusBadRequest, "请求体解析失败", err)
			return
		}
		lang.ID = id
		langStore := storage.NewLanguageStore(h.db)
		if err := langStore.UpdateLanguage(r.Context(), &lang); err != nil {
			h.log.Errorw("update language failed", "error", err)
			FailErr(w, http.StatusInternalServerError, "更新语言环境失败", err)
			return
		}
		OK(w, lang)

	case http.MethodDelete:
		langStore := storage.NewLanguageStore(h.db)
		if err := langStore.DeleteLanguage(r.Context(), id); err != nil {
			h.log.Errorw("delete language failed", "error", err)
			FailErr(w, http.StatusInternalServerError, "删除语言环境失败", err)
			return
		}
		OK(w, map[string]string{"deleted": id})

	default:
		Fail(w, http.StatusMethodNotAllowed, 405, "请求方法不支持")
	}
}

// handleDetectLanguages: GET ?server=<id> 检测服务器已安装的语言。
func (h *SoftwareHandler) handleDetectLanguages(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		Fail(w, http.StatusMethodNotAllowed, 405, "请求方法不支持")
		return
	}

	serverID := r.URL.Query().Get("server")
	srv, ok := h.store.Get(r.Context(), serverID)
	if !ok {
		Fail(w, http.StatusNotFound, 404, "目标服务器不存在")
		return
	}

	langStore := storage.NewLanguageStore(h.db)
	languages := langStore.ListLanguages(r.Context())

	var detected []map[string]any
	for _, lang := range languages {
		if lang.VersionCmd == "" {
			continue
		}
		version := h.remoteCollector.DetectLanguage(r.Context(), toRemoteServer(srv), lang.VersionCmd)
		status := "not_installed"
		if version != "" {
			status = "installed"
		}
		detected = append(detected, map[string]any{
			"id":          lang.ID,
			"name":        lang.Name,
			"icon":        lang.Icon,
			"description": lang.Description,
			"version":     version,
			"status":      status,
			"versions":    lang.Versions,
		})
	}

	OK(w, detected)
}

// handleInstallLanguage: POST {"server_id": "...", "language_id": "..."} 安装语言环境。
func (h *SoftwareHandler) handleInstallLanguage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		Fail(w, http.StatusMethodNotAllowed, 405, "请求方法不支持")
		return
	}

	var req struct {
		ServerID   string `json:"server_id"`
		LanguageID string `json:"language_id"`
		Version    string `json:"version"`
	}
	if err := decodeBody(r, &req); err != nil {
		FailErr(w, http.StatusBadRequest, "请求体解析失败", err)
		return
	}

	srv, ok := h.store.Get(r.Context(), req.ServerID)
	if !ok {
		Fail(w, http.StatusNotFound, 404, "目标服务器不存在")
		return
	}

	langStore := storage.NewLanguageStore(h.db)
	lang, ok := langStore.GetLanguage(r.Context(), req.LanguageID)
	if !ok {
		Fail(w, http.StatusNotFound, 404, "语言环境不存在")
		return
	}
	if lang.InstallCmd == "" {
		Fail(w, http.StatusBadRequest, 400, "该语言未定义安装命令，请在编辑语言的高级选项中配置安装脚本")
		return
	}

	// 异步安装：handler 立即返回，不能用请求 ctx（会随响应结束被取消），
	// 超时由 RemoteCollector.InstallLanguageStream 内部的长操作超时兜底。
	// 输出实时写入任务日志，前端轮询 /languages/job 展示进度。
	displayCmd := lang.InstallCmd
	if req.Version != "" {
		displayCmd = strings.ReplaceAll(displayCmd, "{{VERSION}}", req.Version)
	}
	job := langJobs.start(req.ServerID, lang.ID, "install", displayCmd)
	go func() {
		err := h.remoteCollector.InstallLanguageStream(context.Background(), toRemoteServer(srv), lang.InstallCmd, req.Version, job.append)
		job.finish(err)
		if err != nil {
			h.log.Errorw("install language failed", "server", req.ServerID, "lang", lang.ID, "error", err)
		}
	}()

	OK(w, map[string]string{"status": "installing", "language": lang.ID})
}

// handleUninstallLanguage: POST {"server_id": "...", "language_id": "..."} 卸载语言环境。
func (h *SoftwareHandler) handleUninstallLanguage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		Fail(w, http.StatusMethodNotAllowed, 405, "请求方法不支持")
		return
	}

	var req struct {
		ServerID   string `json:"server_id"`
		LanguageID string `json:"language_id"`
	}
	if err := decodeBody(r, &req); err != nil {
		FailErr(w, http.StatusBadRequest, "请求体解析失败", err)
		return
	}

	srv, ok := h.store.Get(r.Context(), req.ServerID)
	if !ok {
		Fail(w, http.StatusNotFound, 404, "目标服务器不存在")
		return
	}

	langStore := storage.NewLanguageStore(h.db)
	lang, ok := langStore.GetLanguage(r.Context(), req.LanguageID)
	if !ok {
		Fail(w, http.StatusNotFound, 404, "语言环境不存在")
		return
	}
	if lang.UninstallCmd == "" {
		Fail(w, http.StatusBadRequest, 400, "该语言未定义卸载命令，请在语言环境编辑中配置「卸载脚本」后重试")
		return
	}

	// 异步卸载：理由同 handleInstallLanguage，输出实时写入任务日志。
	job := langJobs.start(req.ServerID, lang.ID, "uninstall", lang.UninstallCmd)
	go func() {
		// 使用 UninstallCmd 作为卸载命令
		err := h.remoteCollector.InstallLanguageStream(context.Background(), toRemoteServer(srv), lang.UninstallCmd, "", job.append)
		job.finish(err)
		if err != nil {
			h.log.Errorw("uninstall language failed", "server", req.ServerID, "lang", lang.ID, "error", err)
		}
	}()

	OK(w, map[string]string{"status": "uninstalling", "language": lang.ID})
}

// handleLanguageJob: GET ?server_id=&language_id=&action=install|uninstall 查询任务状态与输出。
func (h *SoftwareHandler) handleLanguageJob(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		Fail(w, http.StatusMethodNotAllowed, 405, "请求方法不支持")
		return
	}

	serverID := r.URL.Query().Get("server_id")
	langID := r.URL.Query().Get("language_id")
	action := r.URL.Query().Get("action")
	if serverID == "" || langID == "" || (action != "install" && action != "uninstall") {
		Fail(w, http.StatusBadRequest, 400, "缺少必填参数：server_id、language_id，或 action 不合法")
		return
	}

	job, err := langJobs.get(serverID, langID, action)
	if err != nil {
		// 任务不存在：服务重启或尚未开始，前端据此提示
		OK(w, map[string]any{"status": "none", "action": action, "log": "", "error": ""})
		return
	}
	OK(w, job.snapshot())
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

func (h *SoftwareHandler) getServerAndPackage(ctx context.Context, serverID, pkgID string) (*host.Server, *software.Package, error) {
	if serverID == "" || serverID == host.LocalServerID {
		return nil, nil, fmt.Errorf("software management requires a remote server")
	}
	srv, ok := h.store.Get(ctx, serverID)
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
