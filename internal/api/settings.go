// Package api 中的 SettingsHandler 提供系统设置 API。
package api

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/smash-hq/server-management/internal/logger"
	"github.com/smash-hq/server-management/internal/storage"
)

// SettingsHandler 提供系统设置管理 API。
type SettingsHandler struct {
	log     *logger.Logger
	db      *storage.DB
	dataDir string
}

// NewSettingsHandler 创建设置处理器。
func NewSettingsHandler(log *logger.Logger, db *storage.DB, dataDir string) *SettingsHandler {
	return &SettingsHandler{
		log:     log.Named("api.settings"),
		db:      db,
		dataDir: dataDir,
	}
}

// Register 注册路由。
func (h *SettingsHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/settings/", h.handleSettings)
	mux.HandleFunc("/api/v1/settings/favicon", h.handleFavicon)
}

// handleSettings: GET 获取设置 / PUT 更新设置。
func (h *SettingsHandler) handleSettings(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		key := strings.TrimPrefix(r.URL.Path, "/api/v1/settings/")
		if key == "" {
			// 返回所有设置
			OK(w, map[string]string{
				"theme": h.getMeta("theme", "system"),
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
			ct := "image/x-icon"
			if strings.HasSuffix(customPath, ".png") {
				ct = "image/png"
			} else if strings.HasSuffix(customPath, ".svg") {
				ct = "image/svg+xml"
			}
			w.Header().Set("Content-Type", ct)
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
