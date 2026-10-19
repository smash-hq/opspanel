package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"strings"

	"github.com/smash-hq/opspanel/internal/host"
	"github.com/smash-hq/opspanel/internal/logger"
	"github.com/smash-hq/opspanel/internal/remote"
)

// ServersHandler 提供服务器配置的 CRUD API 与连接测试。
type ServersHandler struct {
	log    *logger.Logger
	store  *host.Store
	remote *host.RemoteCollector
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
	mux.HandleFunc("/api/v1/servers", h.handleServers)       // GET 列表 / POST 新增
	mux.HandleFunc("/api/v1/servers/test", h.handleTest)     // POST 连接测试（不保存）
	mux.HandleFunc("/api/v1/servers/", h.handleServerDetail) // PUT 更新 / DELETE 删除
}

// handleServers: GET 列表，POST 新增。
func (h *ServersHandler) handleServers(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		servers, err := h.store.List(r.Context())
		if err != nil {
			h.log.Errorw("list servers failed", "error", err)
			FailErr(w, http.StatusInternalServerError, "获取服务器列表失败", err)
			return
		}
		// 返回时清除敏感字段
		for _, s := range servers {
			s.Password = ""
		}
		OK(w, servers)

	case http.MethodPost:
		var srv host.Server
		if err := json.NewDecoder(r.Body).Decode(&srv); err != nil {
			FailErr(w, http.StatusBadRequest, "请求体解析失败", err)
			return
		}
		if err := validateServer(&srv); err != nil {
			FailErr(w, http.StatusBadRequest, "服务器配置不合法", err)
			return
		}
		if err := h.store.Add(r.Context(), &srv); err != nil {
			h.log.Errorw("add server failed", "error", err)
			FailErr(w, http.StatusInternalServerError, "新增服务器失败", err)
			return
		}
		srv.Password = ""
		OK(w, srv)

	default:
		Fail(w, http.StatusMethodNotAllowed, 405, "请求方法不支持")
	}
}

// handleTest: POST 连接测试（不保存配置）。
// 前端在「添加服务器」时先调用此接口验证 SSH 凭据，
// 成功时返回远程主机基本信息供预览。
func (h *ServersHandler) handleTest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		Fail(w, http.StatusMethodNotAllowed, 405, "请求方法不支持")
		return
	}

	var srv host.Server
	if err := json.NewDecoder(r.Body).Decode(&srv); err != nil {
		FailErr(w, http.StatusBadRequest, "请求体解析失败", err)
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
		FailErr(w, http.StatusBadRequest, "服务器配置不合法", err)
		return
	}

	hostInfo, err := h.remote.TestConnection(r.Context(), &remote.Server{
		ID:       srv.ID,
		Name:     srv.Name,
		Type:     srv.Type,
		Host:     srv.Host,
		Port:     srv.Port,
		Username: srv.Username,
		AuthType: srv.AuthType,
		Password: srv.Password,
		KeyPath:  srv.KeyPath,
	})
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
		Fail(w, http.StatusBadRequest, 400, "缺少必填参数：server_id")
		return
	}

	switch r.Method {
	case http.MethodPut:
		var srv host.Server
		if err := json.NewDecoder(r.Body).Decode(&srv); err != nil {
			FailErr(w, http.StatusBadRequest, "请求体解析失败", err)
			return
		}
		srv.ID = id // URL 中的 ID 优先
		// 编辑普通配置时，前端不会回传敏感字段；沿用已保存的凭据和创建时间。
		if existing, ok := h.store.Get(r.Context(), id); ok {
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
			FailErr(w, http.StatusBadRequest, "服务器配置不合法", err)
			return
		}
		if err := h.store.Update(r.Context(), &srv); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				Fail(w, http.StatusNotFound, 404, "目标服务器不存在")
			} else {
				h.log.Errorw("update server failed", "error", err)
				FailErr(w, http.StatusInternalServerError, "更新服务器配置失败", err)
			}
			return
		}
		// 配置已变更：优雅关闭旧连接，下次使用时按新配置重建
		h.remote.CloseConn(id)
		srv.Password = ""
		OK(w, srv)

	case http.MethodDelete:
		if !requireQueryConfirm(w, r) {
			return
		}
		// 本机节点是面板自身的锚点，绝不可删除
		if id == host.LocalServerID {
			Fail(w, http.StatusBadRequest, 400, "本机节点不可删除")
			return
		}
		if err := h.store.Delete(r.Context(), id); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				Fail(w, http.StatusNotFound, 404, "目标服务器不存在")
			} else {
				h.log.Errorw("delete server failed", "server", id, "error", err)
				FailErr(w, http.StatusInternalServerError, "删除服务器配置失败", err)
			}
			return
		}
		// 优雅关闭该服务器的池化连接
		h.remote.CloseConn(id)
		OK(w, map[string]string{"deleted": id})

	default:
		Fail(w, http.StatusMethodNotAllowed, 405, "请求方法不支持")
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
	errEmptyName     = &validationError{"缺少必填参数：name（服务器名称）"}
	errInvalidType   = &validationError{"参数不合法：type（仅支持 local / remote）"}
	errEmptyHost     = &validationError{"缺少必填参数：host（远程服务器必填）"}
	errEmptyUsername = &validationError{"缺少必填参数：username（远程服务器必填）"}
)

type validationError struct {
	msg string
}

func (e *validationError) Error() string { return e.msg }
