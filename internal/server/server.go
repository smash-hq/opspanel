// Package server 组装 HTTP 服务、路由、中间件与优雅关闭。
package server

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"net/http"
	"time"

	"github.com/smash-hq/server-management/internal/api"
	"github.com/smash-hq/server-management/internal/config"
	"github.com/smash-hq/server-management/internal/host"
	"github.com/smash-hq/server-management/internal/logger"
	"github.com/smash-hq/server-management/internal/middleware"
	"github.com/smash-hq/server-management/internal/ws"
)

// Server 封装 HTTP 服务生命周期。
type Server struct {
	cfg       *config.Config
	log       *logger.Logger
	httpSrv   *http.Server
	hub       *ws.Hub
	store     *host.Store
	collector *host.Collector
}

// New 创建服务实例，注册全部路由与中间件。
func New(cfg *config.Config, log *logger.Logger, staticFS embed.FS, dataDir string) *Server {
	mux := http.NewServeMux()

	// --- 服务器配置存储 ---
	store, err := host.NewStore(dataDir)
	if err != nil {
		log.Fatal("host store init failed", "error", err)
	}

	// --- API 处理器 ---
	healthHandler := api.NewHealthHandler()
	healthHandler.Register(mux)

	systemHandler := api.NewSystemHandler(log, store)
	systemHandler.Register(mux)

	serversHandler := api.NewServersHandler(log, store)
	serversHandler.Register(mux)

	softwareHandler := api.NewSoftwareHandler(log, store)
	softwareHandler.Register(mux)

	// --- WebSocket ---
	hub := ws.NewHub(log)
	go hub.Run()
	mux.HandleFunc("/ws", hub.ServeWS)

	// --- 静态前端 ---
	subFS, err := fs.Sub(staticFS, "static")
	if err != nil {
		log.Fatal("static sub fs failed", "error", err)
	}
	fileServer := http.FileServer(http.FS(subFS))
	mux.Handle("/", fileServer)

	// --- 中间件链：Recovery → Logging → SecurityHeaders → mux ---
	var handler http.Handler = mux
	handler = middleware.SecurityHeaders(handler)
	handler = middleware.Logging(log.Named("http"))(handler)
	handler = middleware.Recovery(log.Named("recovery"))(handler)

	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	httpSrv := &http.Server{
		Addr:         addr,
		Handler:      handler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	return &Server{
		cfg:       cfg,
		log:       log,
		httpSrv:   httpSrv,
		hub:       hub,
		store:     store,
		collector: host.NewCollector(log, store),
	}
}

// Start 启动 HTTP 服务并阻塞，直到收到中断信号或出错。
func (s *Server) Start() error {
	s.log.Infow("server starting", "addr", s.httpSrv.Addr)
	go s.pushLoop()
	return s.httpSrv.ListenAndServe()
}

// Shutdown 优雅关闭 HTTP 服务。
func (s *Server) Shutdown(ctx context.Context) error {
	s.log.Infow("server shutting down")
	return s.httpSrv.Shutdown(ctx)
}

// pushLoop 定时采集所有服务器的系统快照并按 serverID 分发到 WebSocket 客户端。
func (s *Server) pushLoop() {
	interval := time.Duration(s.cfg.Server.PushInterval) * time.Second
	if interval < 1*time.Second {
		interval = 3 * time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for range ticker.C {
		// 本机 + 所有已配置服务器
		serverIDs := []string{host.LocalServerID}
		for _, srv := range s.store.List() {
			serverIDs = append(serverIDs, srv.ID)
		}

		for _, id := range serverIDs {
			info, err := s.collector.Snapshot(id)
			if err != nil {
				s.log.Errorw("push snapshot failed", "server", id, "error", err)
				continue
			}
			s.hub.Broadcast(info, id)
		}
	}
}
