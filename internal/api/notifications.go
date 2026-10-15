// notifications.go 提供站内通知 API：列表、单条/全部已读、清空，
// 以及邮件通知（SMTP）配置读写与测试发送。
package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/smash-hq/opspanel/internal/logger"
	"github.com/smash-hq/opspanel/internal/notify"
	"github.com/smash-hq/opspanel/internal/storage"
)

// NotificationHandler 提供站内通知与邮件通知 API。
type NotificationHandler struct {
	log           *logger.Logger
	notifications *notify.Notifier
}

// NewNotificationHandler 创建通知处理器。
func NewNotificationHandler(log *logger.Logger, db *storage.DB) *NotificationHandler {
	return &NotificationHandler{
		log:           log.Named("api.notifications"),
		notifications: notify.New(db, log),
	}
}

// Notifier 暴露通知服务（供其他 handler 复用同一实例）。
func (h *NotificationHandler) Notifier() *notify.Notifier {
	return h.notifications
}

// Register 注册路由。
func (h *NotificationHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/notifications", h.handleNotifications)              // GET 列表 / DELETE 清空
	mux.HandleFunc("/api/v1/notifications/read", h.handleMarkRead)              // POST 全部已读
	mux.HandleFunc("POST /api/v1/notifications/{id}/read", h.handleMarkOneRead) // POST 单条已读
	mux.HandleFunc("/api/v1/notifications/email-config", h.handleEmailConfig)   // GET 读取 / PUT 保存
	mux.HandleFunc("POST /api/v1/notifications/email-test", h.handleEmailTest)  // POST 发送测试邮件
}

// handleNotifications: GET ?limit=50 返回 {items, unread}；DELETE 清空。
func (h *NotificationHandler) handleNotifications(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		limit := 50
		if v := r.URL.Query().Get("limit"); v != "" {
			if n, err := strconv.Atoi(v); err == nil && n > 0 {
				limit = n
			}
		}
		items := h.notifications.List(r.Context(), limit)
		if items == nil {
			items = []*storage.Notification{}
		}
		OK(w, map[string]any{"items": items, "unread": h.notifications.UnreadCount(r.Context())})
	case http.MethodDelete:
		if err := h.notifications.Clear(r.Context()); err != nil {
			Fail(w, http.StatusInternalServerError, 500, "清空通知失败")
			return
		}
		OK(w, map[string]string{"status": "cleared"})
	default:
		Fail(w, http.StatusMethodNotAllowed, 405, "method not allowed")
	}
}

// handleMarkRead: POST 全部标记已读，返回最新未读数。
func (h *NotificationHandler) handleMarkRead(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		Fail(w, http.StatusMethodNotAllowed, 405, "method not allowed")
		return
	}
	if err := h.notifications.MarkAllRead(r.Context()); err != nil {
		Fail(w, http.StatusInternalServerError, 500, "标记已读失败")
		return
	}
	OK(w, map[string]int{"unread": 0})
}

// handleMarkOneRead: POST /{id}/read 标记单条已读，返回最新未读数。
func (h *NotificationHandler) handleMarkOneRead(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		Fail(w, http.StatusBadRequest, 400, "缺少通知 ID")
		return
	}
	if err := h.notifications.MarkRead(r.Context(), id); err != nil {
		Fail(w, http.StatusInternalServerError, 500, "标记已读失败")
		return
	}
	OK(w, map[string]int{"unread": h.notifications.UnreadCount(r.Context())})
}

// handleEmailConfig: GET 返回脱敏邮件配置 / PUT 保存（密码留空表示保持不变）。
func (h *NotificationHandler) handleEmailConfig(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		cfg, hasPassword := h.notifications.EmailConfigMasked(r.Context())
		OK(w, map[string]any{
			"enabled":      cfg.Enabled,
			"host":         cfg.Host,
			"port":         cfg.Port,
			"user":         cfg.User,
			"from":         cfg.From,
			"to":           cfg.To,
			"encryption":   cfg.Encryption,
			"has_password": hasPassword,
		})
	case http.MethodPut:
		var req struct {
			Enabled    bool   `json:"enabled"`
			Host       string `json:"host"`
			Port       int    `json:"port"`
			User       string `json:"user"`
			Password   string `json:"password"`
			From       string `json:"from"`
			To         string `json:"to"`
			Encryption string `json:"encryption"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			Fail(w, http.StatusBadRequest, 400, "invalid request body")
			return
		}
		if req.Port <= 0 {
			req.Port = 465
		}
		if req.Encryption == "" {
			req.Encryption = "ssl"
		}
		cfg := notify.MailConfig{
			Enabled:    req.Enabled,
			Host:       req.Host,
			Port:       req.Port,
			User:       req.User,
			Password:   req.Password,
			From:       req.From,
			To:         req.To,
			Encryption: req.Encryption,
		}
		keepPassword := req.Password == ""
		if err := h.notifications.SaveEmailConfig(r.Context(), cfg, keepPassword); err != nil {
			h.log.Errorw("save email config failed", "error", err)
			Fail(w, http.StatusInternalServerError, 500, "保存邮件配置失败")
			return
		}
		masked, hasPassword := h.notifications.EmailConfigMasked(r.Context())
		OK(w, map[string]any{"enabled": masked.Enabled, "has_password": hasPassword})
	default:
		Fail(w, http.StatusMethodNotAllowed, 405, "method not allowed")
	}
}

// handleEmailTest: POST 发送测试邮件。body 可选 {to}, {password}(回传明文覆盖已存密码用于测试)。
func (h *NotificationHandler) handleEmailTest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		Fail(w, http.StatusMethodNotAllowed, 405, "method not allowed")
		return
	}
	var req struct {
		To       string `json:"to"`
		Password string `json:"password"`
	}
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&req)
	}
	cfg := h.notifications.EmailConfig(r.Context())
	if req.Password != "" {
		cfg.Password = req.Password
	}
	if err := h.notifications.SendTest(r.Context(), cfg, req.To); err != nil {
		h.log.Errorw("send test email failed", "error", err)
		Fail(w, http.StatusBadRequest, 400, err.Error())
		return
	}
	OK(w, map[string]string{"status": "sent"})
}
