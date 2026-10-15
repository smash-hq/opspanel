package api

// AuthHandler 提供登录认证、会话管理与账户持久化。
//
// 账户信息持久化在 meta 表（key = auth_user）。首次部署时懒初始化默认账户
// admin/admin123；修改后的密码随 SQLite 数据持续生效，重新部署（清空数据
// 目录）后恢复默认账户。首次登录强制改密由 enforcePasswordChange 开关控制，
// 当前项目完善前暂时关闭。

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/smash-hq/opspanel/internal/logger"
	"github.com/smash-hq/opspanel/internal/storage"
)

const (
	defaultUsername = "admin"
	defaultPassword = "admin123"

	authMetaKey    = "auth_user"        // meta 表中账户信息的键
	sessionCookie  = "opspanel_session" // 会话 Cookie 名
	sessionTTL     = 7 * 24 * time.Hour
	minPasswordLen = 8  // 新密码最小长度
	maxPasswordLen = 72 // bcrypt 输入上限

	// enforcePasswordChange 控制首次登录是否强制修改默认密码。
	// 项目完善前暂时关闭；置为 true 即可恢复强制改密流程。
	enforcePasswordChange = false
)

// authUserRecord 是持久化的账户信息。
type authUserRecord struct {
	Username     string `json:"username"`
	PasswordHash string `json:"password_hash"`
	MustChange   bool   `json:"must_change"`
}

// AuthHandler 提供认证 API 与全局会话拦截中间件。
type AuthHandler struct {
	log *logger.Logger
	db  *storage.DB

	mu       sync.RWMutex
	sessions map[string]time.Time // token -> 过期时间（内存会话，重启后需重新登录）
}

// NewAuthHandler 创建认证处理器。
func NewAuthHandler(log *logger.Logger, db *storage.DB) *AuthHandler {
	return &AuthHandler{
		log:      log.Named("api.auth"),
		db:       db,
		sessions: make(map[string]time.Time),
	}
}

// Register 注册认证路由。
func (h *AuthHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/auth/login", h.handleLogin)
	mux.HandleFunc("/api/v1/auth/logout", h.handleLogout)
	mux.HandleFunc("/api/v1/auth/me", h.handleMe)
	mux.HandleFunc("/api/v1/auth/change-password", h.handleChangePassword)
}

// Middleware 会话拦截：除登录接口、favicon（登录页引用）与静态资源外，
// 所有 /api 与 /ws 请求均需携带有效会话，并做滑动续期。
func (h *AuthHandler) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := r.URL.Path
		isPublic := p == "/api/v1/auth/login" ||
			p == "/api/v1/settings/favicon" ||
			(!strings.HasPrefix(p, "/api/") && p != "/ws")
		if isPublic {
			next.ServeHTTP(w, r)
			return
		}
		if !h.renewSession(h.cookieToken(r)) {
			Fail(w, http.StatusUnauthorized, 401, "未登录或会话已过期")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// handleLogin: POST {"username","password"} 登录并下发会话 Cookie。
func (h *AuthHandler) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		Fail(w, http.StatusMethodNotAllowed, 405, "method not allowed")
		return
	}
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Fail(w, http.StatusBadRequest, 400, "invalid request body")
		return
	}

	user := h.loadUser(r.Context())
	if req.Username != user.Username ||
		bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)) != nil {
		Fail(w, http.StatusUnauthorized, 401, "用户名或密码错误")
		return
	}

	token, err := h.newSession()
	if err != nil {
		// 随机源故障时拒绝签发可预测 token
		Fail(w, http.StatusInternalServerError, 500, "internal error")
		return
	}
	h.setSessionCookie(w, token)
	h.log.Infow("user logged in", "username", user.Username, "must_change", user.MustChange)
	OK(w, map[string]any{"username": user.Username, "must_change_password": user.MustChange && enforcePasswordChange})
}

// handleLogout: POST 注销当前会话并清除 Cookie。
func (h *AuthHandler) handleLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		Fail(w, http.StatusMethodNotAllowed, 405, "method not allowed")
		return
	}
	if token := h.cookieToken(r); token != "" {
		h.mu.Lock()
		delete(h.sessions, token)
		h.mu.Unlock()
	}
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	OK(w, map[string]bool{"ok": true})
}

// handleMe: GET 返回当前登录账户（中间件已校验会话，此处兜底）。
func (h *AuthHandler) handleMe(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		Fail(w, http.StatusMethodNotAllowed, 405, "method not allowed")
		return
	}
	if !h.renewSession(h.cookieToken(r)) {
		Fail(w, http.StatusUnauthorized, 401, "未登录或会话已过期")
		return
	}
	user := h.loadUser(r.Context())
	OK(w, map[string]any{"username": user.Username, "must_change_password": user.MustChange && enforcePasswordChange})
}

// handleChangePassword: POST {"old_password","new_password"} 校验旧密码后更新。
// 修改成功后清除 must_change 标记，新密码持久化生效（重新部署前）。
func (h *AuthHandler) handleChangePassword(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		Fail(w, http.StatusMethodNotAllowed, 405, "method not allowed")
		return
	}
	if !h.renewSession(h.cookieToken(r)) {
		Fail(w, http.StatusUnauthorized, 401, "未登录或会话已过期")
		return
	}
	var req struct {
		OldPassword string `json:"old_password"`
		NewPassword string `json:"new_password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Fail(w, http.StatusBadRequest, 400, "invalid request body")
		return
	}

	user := h.loadUser(r.Context())
	if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.OldPassword)) != nil {
		Fail(w, http.StatusBadRequest, 400, "当前密码不正确")
		return
	}
	if len(req.NewPassword) < minPasswordLen {
		Fail(w, http.StatusBadRequest, 400, "新密码长度至少 8 位")
		return
	}
	if len(req.NewPassword) > maxPasswordLen {
		Fail(w, http.StatusBadRequest, 400, "新密码长度不能超过 72 位")
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		Fail(w, http.StatusInternalServerError, 500, "密码加密失败")
		return
	}
	user.PasswordHash = string(hash)
	user.MustChange = false
	if err := h.saveUser(r.Context(), user); err != nil {
		h.log.Errorw("save user failed", "error", err)
		Fail(w, http.StatusInternalServerError, 500, "保存账户信息失败")
		return
	}
	h.log.Infow("password changed", "username", user.Username)
	OK(w, map[string]bool{"ok": true})
}

// loadUser 读取账户信息；首次部署（无记录）时懒初始化默认账户并持久化。
func (h *AuthHandler) loadUser(ctx context.Context) authUserRecord {
	if h.db != nil {
		if raw, err := h.db.GetMeta(ctx, authMetaKey); err == nil && raw != "" {
			var u authUserRecord
			if json.Unmarshal([]byte(raw), &u) == nil && u.Username != "" && u.PasswordHash != "" {
				return u
			}
		}
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(defaultPassword), bcrypt.DefaultCost)
	if err != nil {
		h.log.Errorw("hash default password failed", "error", err)
	}
	u := authUserRecord{Username: defaultUsername, PasswordHash: string(hash), MustChange: enforcePasswordChange}
	if err := h.saveUser(ctx, u); err != nil {
		h.log.Errorw("init default user failed", "error", err)
	}
	return u
}

// saveUser 将账户信息写入 meta 表。
func (h *AuthHandler) saveUser(ctx context.Context, u authUserRecord) error {
	if h.db == nil {
		return nil
	}
	raw, err := json.Marshal(u)
	if err != nil {
		return err
	}
	return h.db.SetMeta(ctx, authMetaKey, string(raw))
}

// newSession 生成随机 token 并登记会话（顺带清理过期会话）。
func (h *AuthHandler) newSession() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		h.log.Errorw("generate session token failed", "error", err)
		return "", fmt.Errorf("generate session token: %w", err)
	}
	token := hex.EncodeToString(b)

	h.mu.Lock()
	now := time.Now()
	for t, exp := range h.sessions {
		if now.After(exp) {
			delete(h.sessions, t)
		}
	}
	h.sessions[token] = now.Add(sessionTTL)
	h.mu.Unlock()
	return token, nil
}

// cookieToken 从请求 Cookie 中提取会话 token。
func (h *AuthHandler) cookieToken(r *http.Request) string {
	c, err := r.Cookie(sessionCookie)
	if err != nil {
		return ""
	}
	return c.Value
}

// renewSession 校验会话有效性并滑动续期。
func (h *AuthHandler) renewSession(token string) bool {
	if token == "" {
		return false
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	exp, ok := h.sessions[token]
	if !ok || time.Now().After(exp) {
		delete(h.sessions, token)
		return false
	}
	h.sessions[token] = time.Now().Add(sessionTTL)
	return true
}

// setSessionCookie 下发会话 Cookie。
func (h *AuthHandler) setSessionCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    token,
		Path:     "/",
		MaxAge:   int(sessionTTL.Seconds()),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}
