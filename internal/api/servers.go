package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"strings"

	"github.com/smash-hq/server-management/internal/host"
	"github.com/smash-hq/server-management/internal/logger"
)

// ServersHandler 提供服务器配置的 CRUD API 与连接测试。
type ServersHandler struct {
	log      *logger.Logger
	store    *host.Store
	remote   *host.RemoteCollector
}

// NewServersHandler 创建处理器。
func NewServersHandler(log *logger.Logger, store *host.Store) *ServersHandler {
	return &ServersHandler{
		log:    log.Named("api.servers"),
		store:  store,
		remote: host.NewRemoteCollector(log),
	}
}

// Register 注册路由。
func (h *ServersHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/servers", h.handleServers)           // GET 列表 / POST 新增
	mux.HandleFunc("/api/v1/servers/test", h.handleTest)         // POST 连接测试（不保存）
	mux.HandleFunc("/api/v1/servers/", h.handleServerDetail)     // PUT 更新 / DELETE 删除
}

// handleServers: GET 列表，POST 新增。
func (h *ServersHandler) handleServers(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		servers := h.store.List()
		// 返回时清除敏感字段
		for _, s := range servers {
			s.Password = ""
		}
		OK(w, servers)

	case http.MethodPost:
		var srv host.Server
		if err := json.NewDecoder(r.Body).Decode(&srv); err != nil {
			Fail(w, http.StatusBadRequest, 400, "invalid request body")
			return
		}
		if err := validateServer(&srv); err != nil {
			Fail(w, http.StatusBadRequest, 400, err.Error())
			return
		}
		if err := h.store.Add(&srv); err != nil {
			h.log.Errorw("add server failed", "error", err)
			Fail(w, http.StatusInternalServerError, 500, "failed to add server")
			return
		}
		srv.Password = ""
		OK(w, srv)

	default:
		Fail(w, http.StatusMethodNotAllowed, 405, "method not allowed")
	}
}

// handleTest: POST 连接测试（不保存配置）。
// 前端在「添加服务器」时先调用此接口验证 SSH 凭据，
// 成功时返回远程主机基本信息供预览。
func (h *ServersHandler) handleTest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		Fail(w, http.StatusMethodNotAllowed, 405, "method not allowed")
		return
	}

	var srv host.Server
	if err := json.NewDecoder(r.Body).Decode(&srv); err != nil {
		Fail(w, http.StatusBadRequest, 400, "invalid request body")
		return
	}

	// 本机节点无需测试
	if srv.Type == "local" {
		OK(w, map[string]any{
			"success": true,
			"host": map[string]string{
				"hostname": "localhost",
				"os":       "local",
				"arch":     "local",
			},
		})
		return
	}

	// 远程节点：校验必填字段后尝试 SSH 连接
	if err := validateServer(&srv); err != nil {
		Fail(w, http.StatusBadRequest, 400, err.Error())
		return
	}

	hostInfo, err := h.remote.TestConnection(&srv)
	if err != nil {
		h.log.Errorw("test connection failed", "host", srv.Host, "error", err)
		OK(w, map[string]any{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	OK(w, map[string]any{
		"success": true,
		"host":    hostInfo,
	})
}

// handleServerDetail: PUT 更新，DELETE 删除。
func (h *ServersHandler) handleServerDetail(w http.ResponseWriter, r *http.Request) {
	// 从 URL 提取 ID: /api/v1/servers/{id}
	id := strings.TrimPrefix(r.URL.Path, "/api/v1/servers/")
	if id == "" {
		Fail(w, http.StatusBadRequest, 400, "server id required")
		return
	}

	switch r.Method {
	case http.MethodPut:
		var srv host.Server
		if err := json.NewDecoder(r.Body).Decode(&srv); err != nil {
			Fail(w, http.StatusBadRequest, 400, "invalid request body")
			return
		}
		srv.ID = id // URL 中的 ID 优先
		// 编辑普通配置时，前端不会回传敏感字段；沿用已保存的凭据和创建时间。
		if existing, ok := h.store.Get(id); ok {
			if srv.CreatedAt == 0 {
				srv.CreatedAt = existing.CreatedAt
			}
			if srv.Password == "" {
				srv.Password = existing.Password
			}
			if srv.KeyPath == "" {
				srv.KeyPath = existing.KeyPath
			}
		}
		if err := validateServer(&srv); err != nil {
			Fail(w, http.StatusBadRequest, 400, err.Error())
			return
		}
		if err := h.store.Update(&srv); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				Fail(w, http.StatusNotFound, 404, "server not found")
			} else {
				h.log.Errorw("update server failed", "error", err)
				Fail(w, http.StatusInternalServerError, 500, "failed to update server")
			}
			return
		}
		srv.Password = ""
		OK(w, srv)

	case http.MethodDelete:
		if err := h.store.Delete(id); err != nil {
			Fail(w, http.StatusNotFound, 404, "server not found")
			return
		}
		OK(w, map[string]string{"deleted": id})

	default:
		Fail(w, http.StatusMethodNotAllowed, 405, "method not allowed")
	}
}

// validateServer 校验必填字段。
func validateServer(srv *host.Server) error {
	if strings.TrimSpace(srv.Name) == "" {
		return errEmptyName
	}
	if srv.Type != "local" && srv.Type != "remote" {
		return errInvalidType
	}
	if srv.Type == "remote" {
		if strings.TrimSpace(srv.Host) == "" {
			return errEmptyHost
		}
		if strings.TrimSpace(srv.Username) == "" {
			return errEmptyUsername
		}
	}
	return nil
}

var (
	errEmptyName   = &validationError{"name is required"}
	errInvalidType = &validationError{"type must be 'local' or 'remote'"}
	errEmptyHost   = &validationError{"host is required for remote server"}
	errEmptyUsername = &validationError{"username is required for remote server"}
)

type validationError struct {
	msg string
}

func (e *validationError) Error() string { return e.msg }
