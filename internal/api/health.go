package api

import (
	"net/http"
	"runtime"
)

// HealthHandler 提供存活 / 就绪探针。
type HealthHandler struct{}

// NewHealthHandler 创建健康检查处理器。
func NewHealthHandler() *HealthHandler {
	return &HealthHandler{}
}

// Register 注册路由。
func (h *HealthHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/health", h.handleHealth)
}

func (h *HealthHandler) handleHealth(w http.ResponseWriter, r *http.Request) {
	OK(w, map[string]any{
		"status":    "up",
		"go":        runtime.Version(),
		"goroutine": runtime.NumGoroutine(),
	})
}
