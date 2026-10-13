// Package api 中的 SettingsHandler 提供系统设置 API。
package api

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/smash-hq/server-management/internal/logger"
	"github.com/smash-hq/server-management/internal/storage"
)

// PushController 控制推送循环的接口。
type PushController interface {
	PausePush(paused bool)
}

// SettingsHandler 提供系统设置管理 API。
type SettingsHandler struct {
	log      *logger.Logger
	db       *storage.DB
	dataDir  string
	pushCtrl PushController
}

// NewSettingsHandler 创建设置处理器。
func NewSettingsHandler(log *logger.Logger, db *storage.DB, dataDir string) *SettingsHandler {
	return &SettingsHandler{
		log:     log.Named("api.settings"),
		db:      db,
		dataDir: dataDir,
	}
}

// SetPushController 设置推送控制器。
func (h *SettingsHandler) SetPushController(ctrl PushController) {
	h.pushCtrl = ctrl
}

// Register 注册路由。
func (h *SettingsHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/settings/", h.handleSettings)
	mux.HandleFunc("/api/v1/settings/favicon", h.handleFavicon)
	mux.HandleFunc("/api/v1/settings/push", h.handlePushControl)
}

// handlePushControl: POST {"paused": true/false} 暂停/恢复系统快照推送。
func (h *SettingsHandler) handlePushControl(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		Fail(w, http.StatusMethodNotAllowed, 405, "method not allowed")
		return
	}
	var req struct {
		Paused bool `json:"paused"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Fail(w, http.StatusBadRequest, 400, "invalid request body")
		return
	}
	if h.pushCtrl != nil {
		h.pushCtrl.PausePush(req.Paused)
	}
	OK(w, map[string]bool{"paused": req.Paused})
}

// handleSettings: GET 获取设置 / PUT 更新设置。
func (h *SettingsHandler) handleSettings(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		key := strings.TrimPrefix(r.URL.Path, "/api/v1/settings/")
		if key == "" {
			// 返回所有设置
			OK(w, map[string]string{
				"theme":            h.getMeta("theme", "system"),
				"panel_name":       h.getMeta("panel_name", ""),
				"admin_email":      h.getMeta("admin_email", ""),
				"push_interval":    h.getMeta("push_interval", "3"),
				"default_ssh_port": h.getMeta("default_ssh_port", "22"),
				"welcome_message":  h.getMeta("welcome_message", ""),
			})
			return
		}
		OK(w, map[string]string{key: h.getMeta(key, "")})

	case http.MethodPut:
		var settings map[string]string
		if err := json.NewDecoder(r.Body).Decode(&settings); err != nil {
			Fail(w, http.StatusBadRequest, 400, "invalid request body")
			return
		}
		for key, value := range settings {
			if err := h.setMeta(key, value); err != nil {
				h.log.Errorw("save setting failed", "key", key, "error", err)
			}
		}
		OK(w, settings)

	default:
		Fail(w, http.StatusMethodNotAllowed, 405, "method not allowed")
	}
}

// handleFavicon: GET 获取自定义 favicon / POST 上传自定义 favicon。
func (h *SettingsHandler) handleFavicon(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		// 检查是否有自定义 favicon
		customPath := filepath.Join(h.dataDir, "favicon.custom")
		if data, err := os.ReadFile(customPath); err == nil {
			w.Header().Set("Content-Type", detectImageType(data))
			w.Write(data)
			return
		}
		// 返回默认 favicon
		http.Redirect(w, r, "/favicon.svg", http.StatusFound)

	case http.MethodPost:
		// 限制 1MB
		r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
		data, err := io.ReadAll(r.Body)
		if err != nil {
			Fail(w, http.StatusBadRequest, 400, "file too large (max 1MB)")
			return
		}
		customPath := filepath.Join(h.dataDir, "favicon.custom")
		if err := os.WriteFile(customPath, data, 0644); err != nil {
			h.log.Errorw("save favicon failed", "error", err)
			Fail(w, http.StatusInternalServerError, 500, "failed to save favicon")
			return
		}
		OK(w, map[string]string{"status": "ok"})

	case http.MethodDelete:
		customPath := filepath.Join(h.dataDir, "favicon.custom")
		_ = os.Remove(customPath)
		OK(w, map[string]string{"deleted": "favicon"})

	default:
		Fail(w, http.StatusMethodNotAllowed, 405, "method not allowed")
	}
}

// detectImageType 通过魔数嗅探图片 Content-Type（favicon 上传时不保存后缀，需按内容判断）。
func detectImageType(data []byte) string {
	switch {
	case len(data) >= 8 && bytes.Equal(data[:8], []byte("\x89PNG\r\n\x1a\n")):
		return "image/png"
	case len(data) >= 3 && data[0] == 0xFF && data[1] == 0xD8 && data[2] == 0xFF:
		return "image/jpeg"
	case len(data) >= 4 && bytes.Equal(data[:4], []byte("GIF8")):
		return "image/gif"
	case len(data) >= 4 && data[0] == 0x00 && data[1] == 0x00 && data[2] == 0x01 && data[3] == 0x00:
		return "image/x-icon"
	case len(data) > 0:
		if trimmed := bytes.TrimLeft(data, " \t\r\n\xef\xbb\xbf"); len(trimmed) > 0 && trimmed[0] == '<' {
			return "image/svg+xml"
		}
	}
	return "application/octet-stream"
}

// getMeta 从 meta 表读取值。
func (h *SettingsHandler) getMeta(key, defaultVal string) string {
	if h.db == nil {
		return defaultVal
	}
	var val string
	err := h.db.Raw().QueryRow(`SELECT value FROM meta WHERE key = ?`, key).Scan(&val)
	if err != nil {
		return defaultVal
	}
	return val
}

// setMeta 写入 meta 表。
func (h *SettingsHandler) setMeta(key, value string) error {
	if h.db == nil {
		return nil
	}
	_, err := h.db.Raw().Exec(`
		INSERT INTO meta (key, value) VALUES (?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value`, key, value)
	return err
}
