package api

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/smash-hq/opspanel/internal/gpu"
	"github.com/smash-hq/opspanel/internal/host"
	"github.com/smash-hq/opspanel/internal/logger"
	"github.com/smash-hq/opspanel/internal/remote"
)

// GPUHandler 服务器 GPU/NPU 信息接口：无 agent，经 SSH 实时按需拉取厂商工具输出。
type GPUHandler struct {
	log     *logger.Logger
	servers *host.Store
	rc      *remote.RemoteCollector
}

// NewGPUHandler 创建 GPU 信息处理器。
func NewGPUHandler(log *logger.Logger, servers *host.Store) *GPUHandler {
	return &GPUHandler{
		log:     log.Named("api.gpu"),
		servers: servers,
		rc:      remote.NewRemoteCollector(log),
	}
}

// Register 注册路由。
func (h *GPUHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/gpu", h.handleSnapshot)
}

// handleSnapshot: GET /api/v1/gpu?server_id= 返回该服务器 GPU 快照。
func (h *GPUHandler) handleSnapshot(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		Fail(w, http.StatusMethodNotAllowed, 405, "请求方法不支持")
		return
	}
	srv, ok := h.server(r.Context(), r.URL.Query().Get("server_id"))
	if !ok {
		Fail(w, http.StatusBadRequest, 400, "目标服务器不存在")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	client, err := h.rc.Acquire(toRemoteServer(srv))
	if err != nil {
		h.log.Errorw("acquire connection failed", "server", srv.ID, "error", err)
		FailErr(w, http.StatusBadGateway, "连接服务器失败", err)
		return
	}
	defer h.rc.Release(srv.ID)

	snap, err := gpu.Collect(ctx, sshRunner{client})
	if err != nil {
		h.log.Errorw("gpu collect failed", "server", srv.ID, "error", err)
		FailErr(w, http.StatusBadGateway, "GPU 信息采集失败", err)
		return
	}
	OK(w, snap)
}

// server 解析目标服务器；server_id 为空时取本机节点。
func (h *GPUHandler) server(ctx context.Context, serverID string) (*host.Server, bool) {
	if strings.TrimSpace(serverID) == "" {
		serverID = host.LocalServerID
	}
	return h.servers.Get(ctx, serverID)
}

// runOncer 与 remote 包内 commandRunner 的 RunOnce 签名一致（接口未导出，此处重新声明）。
type runOncer interface {
	RunOnce(ctx context.Context, cmd string) (string, error)
}

// sshRunner 将 remote 连接适配为 gpu.Runner。
type sshRunner struct{ c runOncer }

func (r sshRunner) Run(ctx context.Context, cmd string) (string, error) {
	return r.c.RunOnce(ctx, cmd)
}
