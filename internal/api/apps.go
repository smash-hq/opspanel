// apps.go 提供应用部署管理 API：服务列表、文件上传、启动/停止/重启、日志查看。
package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/smash-hq/opspanel/internal/host"
	"github.com/smash-hq/opspanel/internal/logger"
	"github.com/smash-hq/opspanel/internal/notify"
	"github.com/smash-hq/opspanel/internal/remote"
	"github.com/smash-hq/opspanel/internal/storage"
)

// uploadKeyPattern 校验上传会话 key：仅允许小写 hex，防路径穿越。
var uploadKeyPattern = regexp.MustCompile(`^[0-9a-f]{16,64}$`)

// newUploadKey 生成随机上传会话 key（32 位 hex）。
func newUploadKey() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic(fmt.Sprintf("生成上传会话 key 失败: %v", err))
	}
	return hex.EncodeToString(b)
}

// validUploadKey 校验 key 格式合法性。
func validUploadKey(key string) bool {
	return uploadKeyPattern.MatchString(key)
}

// AppHandler 提供应用部署管理 API。
type AppHandler struct {
	log             *logger.Logger
	store           *host.Store
	apps            *storage.AppStore
	remoteCollector *remote.RemoteCollector
	db              *storage.DB
	notifications   *notify.Notifier
}

// NewAppHandler 创建应用部署处理器。
func NewAppHandler(log *logger.Logger, store *host.Store, apps *storage.AppStore, db *storage.DB) *AppHandler {
	return &AppHandler{
		log:             log.Named("api.apps"),
		store:           store,
		apps:            apps,
		remoteCollector: remote.NewRemoteCollector(log),
		db:              db,
		notifications:   notify.New(db, log),
	}
}

// Register 注册路由。
func (h *AppHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/apps", h.handleApps) // GET 列表 / POST 创建
	// 分片上传（断点续传）：init 建会话 → status 查已传分片 → chunk 传分片 → complete 合并
	mux.HandleFunc("/api/v1/apps/upload/init", h.handleUploadInit)
	mux.HandleFunc("/api/v1/apps/upload/status", h.handleUploadStatus)
	mux.HandleFunc("/api/v1/apps/upload/chunk", h.handleUploadChunk)
	mux.HandleFunc("/api/v1/apps/upload/complete", h.handleUploadComplete)
	mux.HandleFunc("/api/v1/apps/", h.handleAppDetail) // {id} PUT/DELETE；{id}/start|stop|restart POST；{id}/logs GET
}

// handleApps: GET ?server=<id>&refresh=1 列表 / POST 创建部署应用。
func (h *AppHandler) handleApps(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.listApps(w, r)
	case http.MethodPost:
		h.createApp(w, r)
	default:
		Fail(w, http.StatusMethodNotAllowed, 405, "method not allowed")
	}
}

// appOpTimeout 是服务启动/停止/重启的独立超时：操作与 HTTP 请求上下文解耦后需要自带上限，
// 避免客户端断开（r.Context() 取消）中断远程操作导致状态与服务器实际不一致。
const appOpTimeout = 3 * time.Minute

// appOpCtx 返回与 HTTP 请求解耦的操作上下文（客户端断开不影响启动/停止继续执行与状态落库）。
func appOpCtx() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), appOpTimeout)
}

// listApps 返回部署应用列表；refresh=1 时逐个核实进程存活状态并回写。
func (h *AppHandler) listApps(w http.ResponseWriter, r *http.Request) {
	serverID := r.URL.Query().Get("server")
	apps := h.apps.ListApps(r.Context(), serverID)

	if r.URL.Query().Get("refresh") == "1" && len(apps) > 0 {
		srvCache := map[string]*remote.Server{}
		for _, app := range apps {
			srv, cached := srvCache[app.ServerID]
			if !cached {
				hs, found := h.store.Get(r.Context(), app.ServerID)
				if !found {
					continue
				}
				srv = toRemoteServer(hs)
				srvCache[app.ServerID] = srv
			}
			if app.PID <= 0 {
				// 无 PID：尝试认领面板外已在运行的服务进程（手动启动 / 历史启动状态未落库），
				// 认领失败时保留原状态（失败徽标不被覆盖）
				if pid := h.remoteCollector.AdoptApp(r.Context(), srv, app.StartCmd); pid > 0 {
					app.PID = pid
					app.Status = "running"
					_ = h.apps.UpdateApp(r.Context(), app)
				}
				continue
			}
			if status := h.remoteCollector.AppStatus(r.Context(), srv, app.PID); status != app.Status {
				app.Status = status
				_ = h.apps.UpdateApp(r.Context(), app)
			}
		}
	}
	if apps == nil {
		apps = []*storage.DeployedApp{}
	}
	OK(w, apps)
}

// createApp 创建部署应用记录（不校验语言环境，启动时再检测并提示安装）。
func (h *AppHandler) createApp(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ServerID  string `json:"server_id"`
		Name      string `json:"name"`
		Language  string `json:"language"`
		DeployDir string `json:"deploy_dir"`
		StartCmd  string `json:"start_cmd"`
		LogFile   string `json:"log_file"`
		Remark    string `json:"remark"`
	}
	if err := decodeBody(r, &req); err != nil {
		Fail(w, http.StatusBadRequest, 400, "invalid request body")
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	req.DeployDir = strings.TrimSuffix(strings.TrimSpace(req.DeployDir), "/")
	req.StartCmd = strings.TrimSpace(req.StartCmd)
	req.LogFile = strings.TrimSpace(req.LogFile)

	if req.ServerID == "" || req.Name == "" || req.StartCmd == "" {
		Fail(w, http.StatusBadRequest, 400, "服务器、名称与启动命令不能为空")
		return
	}
	switch req.Language {
	case "java", "python", "go":
	default:
		Fail(w, http.StatusBadRequest, 400, "部署语言仅支持 java / python / go")
		return
	}
	if !strings.HasPrefix(req.DeployDir, "/") {
		Fail(w, http.StatusBadRequest, 400, "部署目录必须为绝对路径")
		return
	}
	if _, ok := h.store.Get(r.Context(), req.ServerID); !ok {
		Fail(w, http.StatusNotFound, 404, "server not found")
		return
	}

	app := &storage.DeployedApp{
		ID:        storage.GenerateAppID(),
		ServerID:  req.ServerID,
		Name:      req.Name,
		Language:  req.Language,
		DeployDir: req.DeployDir,
		StartCmd:  req.StartCmd,
		LogFile:   req.LogFile,
		Status:    "stopped",
		Remark:    strings.TrimSpace(req.Remark),
	}
	if err := h.apps.AddApp(r.Context(), app); err != nil {
		Fail(w, http.StatusInternalServerError, 500, "保存部署记录失败: "+err.Error())
		return
	}
	OK(w, app)
}

// appChunkSize 是分片上传的默认分片大小（4MB）。
const appChunkSize = 4 << 20

// handleUploadInit: POST {app_id, rel_path, size, key?} 创建分片上传会话，返回 key 与分片大小。
// key 可选：前端对同一文件生成稳定 key，重试/重开弹窗时复用已落盘分片（断点续传）。
func (h *AppHandler) handleUploadInit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		Fail(w, http.StatusMethodNotAllowed, 405, "method not allowed")
		return
	}
	var req struct {
		AppID   string `json:"app_id"`
		RelPath string `json:"rel_path"`
		Size    int64  `json:"size"`
		Key     string `json:"key"`
	}
	if err := decodeBody(r, &req); err != nil {
		Fail(w, http.StatusBadRequest, 400, "invalid request body")
		return
	}
	app, err := h.apps.GetApp(r.Context(), strings.TrimSpace(req.AppID))
	if err != nil {
		Fail(w, http.StatusNotFound, 404, "部署应用不存在")
		return
	}
	key := strings.TrimSpace(req.Key)
	if key == "" {
		key = newUploadKey()
	} else if !validUploadKey(key) {
		Fail(w, http.StatusBadRequest, 400, "非法的上传会话 key")
		return
	}
	srv, err := h.uploadServer(r.Context(), app)
	if err != nil {
		Fail(w, http.StatusNotFound, 404, err.Error())
		return
	}
	if err := h.remoteCollector.AppUploadInit(r.Context(), srv, app.DeployDir, key); err != nil {
		h.log.Errorw("init upload session failed", "app", app.ID, "error", err)
		Fail(w, http.StatusInternalServerError, 500, err.Error())
		return
	}
	OK(w, map[string]any{"key": key, "chunk_size": appChunkSize})
}

// handleUploadStatus: GET ?key=&total= 查询已落盘分片索引（断点续传依据）。
func (h *AppHandler) handleUploadStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		Fail(w, http.StatusMethodNotAllowed, 405, "method not allowed")
		return
	}
	key := r.URL.Query().Get("key")
	if !validUploadKey(key) {
		Fail(w, http.StatusBadRequest, 400, "非法的上传会话 key")
		return
	}
	// 会话 key 无归属校验风险：key 为服务端随机数且仅能写入部署目录暂存区
	appID := r.URL.Query().Get("app_id")
	app, err := h.apps.GetApp(r.Context(), appID)
	if err != nil {
		Fail(w, http.StatusNotFound, 404, "部署应用不存在")
		return
	}
	srv, err := h.uploadServer(r.Context(), app)
	if err != nil {
		Fail(w, http.StatusNotFound, 404, err.Error())
		return
	}
	received, err := h.remoteCollector.AppUploadStatus(r.Context(), srv, app.DeployDir, key)
	if err != nil {
		Fail(w, http.StatusInternalServerError, 500, err.Error())
		return
	}
	OK(w, map[string]any{"received": received})
}

// handleUploadChunk: POST multipart {key, index, file} 上传单个分片。
func (h *AppHandler) handleUploadChunk(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		Fail(w, http.StatusMethodNotAllowed, 405, "method not allowed")
		return
	}
	if err := r.ParseMultipartForm(8 << 20); err != nil {
		Fail(w, http.StatusBadRequest, 400, "解析上传数据失败: "+err.Error())
		return
	}
	key := strings.TrimSpace(r.FormValue("key"))
	if !validUploadKey(key) {
		Fail(w, http.StatusBadRequest, 400, "非法的上传会话 key")
		return
	}
	idx, err := strconv.Atoi(strings.TrimSpace(r.FormValue("index")))
	if err != nil || idx < 0 || idx > 1_000_000 {
		Fail(w, http.StatusBadRequest, 400, "非法的分片序号")
		return
	}
	appID := strings.TrimSpace(r.FormValue("app_id"))
	app, err := h.apps.GetApp(r.Context(), appID)
	if err != nil {
		Fail(w, http.StatusNotFound, 404, "部署应用不存在")
		return
	}
	file, _, err := r.FormFile("file")
	if err != nil {
		Fail(w, http.StatusBadRequest, 400, "缺少分片数据字段 file")
		return
	}
	defer file.Close()

	srv, err := h.uploadServer(r.Context(), app)
	if err != nil {
		Fail(w, http.StatusNotFound, 404, err.Error())
		return
	}
	if err := h.remoteCollector.AppUploadChunk(r.Context(), srv, app.DeployDir, key, idx, file); err != nil {
		h.log.Errorw("upload chunk failed", "app", app.ID, "index", idx, "error", err)
		Fail(w, http.StatusInternalServerError, 500, err.Error())
		return
	}
	OK(w, map[string]any{"index": idx})
}

// handleUploadComplete: POST {app_id, key, rel_path, total, size} 合并分片为最终文件。
func (h *AppHandler) handleUploadComplete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		Fail(w, http.StatusMethodNotAllowed, 405, "method not allowed")
		return
	}
	var req struct {
		AppID   string `json:"app_id"`
		Key     string `json:"key"`
		RelPath string `json:"rel_path"`
		Total   int    `json:"total"`
		Size    int64  `json:"size"`
	}
	if err := decodeBody(r, &req); err != nil {
		Fail(w, http.StatusBadRequest, 400, "invalid request body")
		return
	}
	app, err := h.apps.GetApp(r.Context(), strings.TrimSpace(req.AppID))
	if err != nil {
		Fail(w, http.StatusNotFound, 404, "部署应用不存在")
		return
	}
	if !validUploadKey(req.Key) {
		Fail(w, http.StatusBadRequest, 400, "非法的上传会话 key")
		return
	}
	if req.Total <= 0 {
		Fail(w, http.StatusBadRequest, 400, "分片总数不合法")
		return
	}
	relPath, err := sanitizeRelPath(req.RelPath, "")
	if err != nil {
		Fail(w, http.StatusBadRequest, 400, err.Error())
		return
	}

	srv, err := h.uploadServer(r.Context(), app)
	if err != nil {
		Fail(w, http.StatusNotFound, 404, err.Error())
		return
	}
	size, err := h.remoteCollector.AppUploadComplete(r.Context(), srv, app.DeployDir, req.Key, relPath, req.Total, req.Size)
	if err != nil {
		h.log.Errorw("complete upload failed", "app", app.ID, "path", relPath, "error", err)
		Fail(w, http.StatusInternalServerError, 500, err.Error())
		return
	}

	// 记录已上传文件清单
	app, err = h.apps.GetApp(r.Context(), app.ID)
	if err == nil {
		addArtifact(app, relPath)
		_ = h.apps.UpdateApp(r.Context(), app)
	}
	h.log.Infow("app file uploaded", "app", app.ID, "path", relPath, "size", size)
	OK(w, map[string]any{"path": relPath, "size": size})
}

// uploadTargetApp 解析上传请求中的 app_id 并校验应用存在。
func (h *AppHandler) uploadTargetApp(r *http.Request) (*storage.DeployedApp, error) {
	var req struct {
		AppID string `json:"app_id"`
	}
	// init/complete 走 JSON body；兼容 form 提交
	if strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
		if err := decodeBody(r, &req); err != nil {
			return nil, fmt.Errorf("invalid request body")
		}
	} else {
		req.AppID = strings.TrimSpace(r.FormValue("app_id"))
	}
	if req.AppID == "" {
		return nil, fmt.Errorf("app_id 不能为空")
	}
	app, err := h.apps.GetApp(r.Context(), req.AppID)
	if err != nil {
		return nil, fmt.Errorf("部署应用不存在")
	}
	return app, nil
}

// uploadServer 获取应用所属服务器的远程执行对象；服务器已删除时返回错误。
func (h *AppHandler) uploadServer(ctx context.Context, app *storage.DeployedApp) (*remote.Server, error) {
	srv, ok := h.store.Get(ctx, app.ServerID)
	if !ok {
		return nil, fmt.Errorf("服务器不存在或已被删除")
	}
	return toRemoteServer(srv), nil
}

// sanitizeRelPath 清洗部署内相对路径：拒绝目录穿越与绝对路径；为空时回退到 fallback 文件名。
func sanitizeRelPath(relPath, fallback string) (string, error) {
	relPath = strings.TrimSpace(relPath)
	if relPath == "" && fallback != "" {
		relPath = path.Base(fallback)
	}
	relPath = path.Clean(relPath)
	if relPath == "." || relPath == ".." || strings.HasPrefix(relPath, "../") || path.IsAbs(relPath) {
		return "", fmt.Errorf("非法的文件路径: %s", relPath)
	}
	return relPath, nil
}

// addArtifact 将 relPath 追加到应用已上传文件清单（去重）。
func addArtifact(app *storage.DeployedApp, relPath string) {
	artifacts := parseArtifacts(app.Artifacts)
	for _, a := range artifacts {
		if a == relPath {
			return
		}
	}
	artifacts = append(artifacts, relPath)
	if blob, err := json.Marshal(artifacts); err == nil {
		app.Artifacts = string(blob)
	}
}

// handleAppDetail 按 ID 分发：{id} 更新/删除，{id}/start|stop|restart 生命周期，{id}/logs 日志。
func (h *AppHandler) handleAppDetail(w http.ResponseWriter, r *http.Request) {
	p := strings.TrimPrefix(r.URL.Path, "/api/v1/apps/")
	parts := strings.Split(p, "/")
	if len(parts) == 0 || parts[0] == "" {
		Fail(w, http.StatusNotFound, 404, "not found")
		return
	}
	app, err := h.apps.GetApp(r.Context(), parts[0])
	if err != nil {
		Fail(w, http.StatusNotFound, 404, "部署应用不存在")
		return
	}

	if len(parts) == 1 {
		switch r.Method {
		case http.MethodPut:
			h.updateApp(w, r, app)
		case http.MethodDelete:
			h.deleteApp(w, r, app)
		default:
			Fail(w, http.StatusMethodNotAllowed, 405, "method not allowed")
		}
		return
	}

	srv, ok := h.store.Get(r.Context(), app.ServerID)
	if !ok {
		Fail(w, http.StatusNotFound, 404, "server not found")
		return
	}
	rs := toRemoteServer(srv)

	switch parts[1] {
	case "start":
		h.startApp(w, r, app, rs)
	case "stop":
		h.stopApp(w, r, app, rs)
	case "restart":
		h.restartApp(w, r, app, rs)
	case "logs":
		h.appLogs(w, r, app, rs)
	default:
		Fail(w, http.StatusNotFound, 404, "not found")
	}
}

// maxLastErrorLen 限制 last_error 存储长度，避免异常堆栈撑爆记录。
const maxLastErrorLen = 2000

// markStartFailed 记录一次启动失败：置 failed 状态、保存原因并写入站内通知。
func (h *AppHandler) markStartFailed(ctx context.Context, app *storage.DeployedApp, reason string) {
	reason = strings.TrimSpace(reason)
	if len(reason) > maxLastErrorLen {
		reason = reason[:maxLastErrorLen] + "…"
	}
	app.PID = 0
	app.Status = "failed"
	app.LastError = reason
	if err := h.apps.UpdateApp(ctx, app); err != nil {
		h.log.Errorw("save failed status", "app", app.ID, "error", err)
	}
	title := fmt.Sprintf("服务「%s」启动失败", app.Name)
	if err := h.notifications.Add(ctx, "error", "apps", title, reason); err != nil {
		h.log.Errorw("add notification", "app", app.ID, "error", err)
	}
}

// startApp: POST {id}/start 启动服务（先检测语言环境，缺失则提示安装）。
func (h *AppHandler) startApp(w http.ResponseWriter, r *http.Request, app *storage.DeployedApp, rs *remote.Server) {
	if r.Method != http.MethodPost {
		Fail(w, http.StatusMethodNotAllowed, 405, "method not allowed")
		return
	}
	opCtx, cancel := appOpCtx()
	defer cancel()
	if msg := h.checkLanguage(opCtx, app, rs); msg != "" {
		h.markStartFailed(opCtx, app, "缺少语言环境："+msg)
		Fail(w, http.StatusBadRequest, 400, msg)
		return
	}

	pid, err := h.remoteCollector.StartApp(opCtx, rs, app.DeployDir, app.StartCmd, app.LogFile)
	if err != nil {
		h.log.Errorw("start app failed", "app", app.ID, "error", err)
		h.markStartFailed(opCtx, app, err.Error())
		Fail(w, http.StatusInternalServerError, 500, err.Error())
		return
	}
	app.PID = pid
	app.Status = "running"
	app.LastError = ""
	_ = h.apps.UpdateApp(opCtx, app)
	h.log.Infow("app started", "app", app.ID, "pid", pid)
	OK(w, map[string]any{"status": "running", "pid": pid})
}

// stopApp: POST {id}/stop 停止服务。
func (h *AppHandler) stopApp(w http.ResponseWriter, r *http.Request, app *storage.DeployedApp, rs *remote.Server) {
	if r.Method != http.MethodPost {
		Fail(w, http.StatusMethodNotAllowed, 405, "method not allowed")
		return
	}
	opCtx, cancel := appOpCtx()
	defer cancel()
	if app.PID > 0 {
		if err := h.remoteCollector.StopApp(opCtx, rs, app.PID); err != nil {
			Fail(w, http.StatusInternalServerError, 500, err.Error())
			return
		}
	}
	app.PID = 0
	app.Status = "stopped"
	app.LastError = ""
	_ = h.apps.UpdateApp(opCtx, app)
	h.log.Infow("app stopped", "app", app.ID)
	OK(w, map[string]string{"status": "stopped"})
}

// restartApp: POST {id}/restart 重启服务（先停旧进程再启动）。
func (h *AppHandler) restartApp(w http.ResponseWriter, r *http.Request, app *storage.DeployedApp, rs *remote.Server) {
	if r.Method != http.MethodPost {
		Fail(w, http.StatusMethodNotAllowed, 405, "method not allowed")
		return
	}
	opCtx, cancel := appOpCtx()
	defer cancel()
	if app.PID > 0 {
		if err := h.remoteCollector.StopApp(opCtx, rs, app.PID); err != nil {
			Fail(w, http.StatusInternalServerError, 500, "停止旧进程失败: "+err.Error())
			return
		}
	}
	if msg := h.checkLanguage(opCtx, app, rs); msg != "" {
		h.markStartFailed(opCtx, app, "缺少语言环境："+msg)
		Fail(w, http.StatusBadRequest, 400, msg)
		return
	}

	pid, err := h.remoteCollector.StartApp(opCtx, rs, app.DeployDir, app.StartCmd, app.LogFile)
	if err != nil {
		h.log.Errorw("restart app failed", "app", app.ID, "error", err)
		h.markStartFailed(opCtx, app, err.Error())
		Fail(w, http.StatusInternalServerError, 500, err.Error())
		return
	}
	app.PID = pid
	app.Status = "running"
	app.LastError = ""
	_ = h.apps.UpdateApp(opCtx, app)
	h.log.Infow("app restarted", "app", app.ID, "pid", pid)
	OK(w, map[string]any{"status": "running", "pid": pid})
}

// appLogs: GET {id}/logs?lines=200 查看服务日志末尾。
func (h *AppHandler) appLogs(w http.ResponseWriter, r *http.Request, app *storage.DeployedApp, rs *remote.Server) {
	if r.Method != http.MethodGet {
		Fail(w, http.StatusMethodNotAllowed, 405, "method not allowed")
		return
	}
	lines := 200
	if v := r.URL.Query().Get("lines"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 5000 {
			lines = n
		}
	}
	logFile := app.LogFile
	if logFile == "" {
		logFile = path.Join(app.DeployDir, "app.log")
	}
	out := h.remoteCollector.TailAppLog(r.Context(), rs, logFile, lines)
	OK(w, map[string]string{"log": out, "log_file": logFile})
}

// updateApp: PUT {id} 更新部署配置。
func (h *AppHandler) updateApp(w http.ResponseWriter, r *http.Request, app *storage.DeployedApp) {
	var req struct {
		Name      *string `json:"name"`
		Language  *string `json:"language"`
		DeployDir *string `json:"deploy_dir"`
		StartCmd  *string `json:"start_cmd"`
		LogFile   *string `json:"log_file"`
		Remark    *string `json:"remark"`
	}
	if err := decodeBody(r, &req); err != nil {
		Fail(w, http.StatusBadRequest, 400, "invalid request body")
		return
	}
	if req.Name != nil && strings.TrimSpace(*req.Name) != "" {
		app.Name = strings.TrimSpace(*req.Name)
	}
	if req.Language != nil {
		switch *req.Language {
		case "java", "python", "go":
			app.Language = *req.Language
		default:
			Fail(w, http.StatusBadRequest, 400, "部署语言仅支持 java / python / go")
			return
		}
	}
	if req.DeployDir != nil && strings.HasPrefix(strings.TrimSpace(*req.DeployDir), "/") {
		app.DeployDir = strings.TrimSuffix(strings.TrimSpace(*req.DeployDir), "/")
	}
	if req.StartCmd != nil && strings.TrimSpace(*req.StartCmd) != "" {
		app.StartCmd = strings.TrimSpace(*req.StartCmd)
	}
	if req.LogFile != nil {
		app.LogFile = strings.TrimSpace(*req.LogFile)
	}
	if req.Remark != nil {
		app.Remark = strings.TrimSpace(*req.Remark)
	}
	if err := h.apps.UpdateApp(r.Context(), app); err != nil {
		Fail(w, http.StatusInternalServerError, 500, "更新部署记录失败: "+err.Error())
		return
	}
	OK(w, app)
}

// deleteApp: DELETE {id} 删除部署应用（进程仍在运行时先尽力停止，失败不阻断删除）。
func (h *AppHandler) deleteApp(w http.ResponseWriter, r *http.Request, app *storage.DeployedApp) {
	if r.Method != http.MethodDelete {
		Fail(w, http.StatusMethodNotAllowed, 405, "method not allowed")
		return
	}
	if srv, ok := h.store.Get(r.Context(), app.ServerID); ok && app.PID > 0 {
		_ = h.remoteCollector.StopApp(r.Context(), toRemoteServer(srv), app.PID)
	}
	if err := h.apps.DeleteApp(r.Context(), app.ID); err != nil {
		Fail(w, http.StatusInternalServerError, 500, "删除部署记录失败: "+err.Error())
		return
	}
	h.log.Infow("app deleted", "app", app.ID)
	OK(w, map[string]string{"status": "deleted"})
}

// checkLanguage 检测部署语言运行环境；缺失时返回提示文案，已安装或无法判定返回空串。
func (h *AppHandler) checkLanguage(ctx context.Context, app *storage.DeployedApp, rs *remote.Server) string {
	lang, ok := storage.NewLanguageStore(h.db).GetLanguage(ctx, app.Language)
	if !ok || lang.VersionCmd == "" {
		return ""
	}
	if version := h.remoteCollector.DetectLanguage(ctx, rs, lang.VersionCmd); version != "" {
		return ""
	}
	return fmt.Sprintf("服务器缺少 %s 运行环境，请先在「语言环境」页安装后再启动", lang.Name)
}

// parseArtifacts 解析已上传文件清单 JSON。
func parseArtifacts(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	var arr []string
	if err := json.Unmarshal([]byte(s), &arr); err != nil {
		return nil
	}
	return arr
}
