package api

import (
	"net/http"

	"github.com/smash-hq/server-management/internal/host"
	"github.com/smash-hq/server-management/internal/logger"
)

// SystemHandler 提供系统信息采集 API。
type SystemHandler struct {
	log       *logger.Logger
	collector *host.Collector
}

// NewSystemHandler 创建系统 API 处理器。
func NewSystemHandler(log *logger.Logger, store *host.Store) *SystemHandler {
	return &SystemHandler{
		log:       log.Named("api.system"),
		collector: host.NewCollector(log, store),
	}
}

// Register 注册路由。
func (h *SystemHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/system/info", h.handleInfo)
}

func (h *SystemHandler) handleInfo(w http.ResponseWriter, r *http.Request) {
	// 支持 ?server=<id> 指定目标服务器，默认本机
	serverID := r.URL.Query().Get("server")

	info, err := h.collector.Snapshot(serverID)
	if err != nil {
		h.log.Errorw("snapshot failed", "server", serverID, "error", err)
		Fail(w, http.StatusInternalServerError, 500, "failed to collect system info: "+err.Error())
		return
	}
	OK(w, info)
}
