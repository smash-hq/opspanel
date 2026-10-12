// Package server 组装 HTTP 服务、路由、中间件与优雅关闭。
package server

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/smash-hq/server-management/internal/api"
	"github.com/smash-hq/server-management/internal/config"
	"github.com/smash-hq/server-management/internal/host"
	"github.com/smash-hq/server-management/internal/logger"
	"github.com/smash-hq/server-management/internal/middleware"
	"github.com/smash-hq/server-management/internal/remote"
	"github.com/smash-hq/server-management/internal/software"
	"github.com/smash-hq/server-management/internal/storage"
	"github.com/smash-hq/server-management/internal/ws"
)

// Server 封装 HTTP 服务生命周期。
type Server struct {
	cfg        *config.Config
	log        *logger.Logger
	httpSrv    *http.Server
	hub        *ws.Hub
	store      *host.Store
	collector  *host.Collector
	pushPaused atomic.Bool // 暂停推送循环（用户不在系统信息页时）
}

// New 创建服务实例，注册全部路由与中间件。
func New(cfg *config.Config, log *logger.Logger, staticFS embed.FS, dataDir string) *Server {
	mux := http.NewServeMux()

	// --- 存储层初始化 ---
	store, err := host.NewStore(dataDir)
	if err != nil {
		log.Fatal("host store init failed", "error", err)
	}

	// 初始化软件包目录到数据库
	swStore := storage.NewSoftwareStore(store.DB())
	if err := swStore.InitDefaultPackages(software.Catalog); err != nil {
		log.Fatal("software packages init failed", "error", err)
	}
	// 刷新内存中的软件包索引
	software.LoadFromStore(swStore)

	// 初始化编程语言环境到数据库
	langStore := storage.NewLanguageStore(store.DB())
	if err := langStore.InitDefaultLanguages(); err != nil {
		log.Fatal("languages init failed", "error", err)
	}

	// 初始化 Docker 镜像源到数据库
	sourceStore := storage.NewDockerSourceStore(store.DB())
	if err := sourceStore.InitDefaultSources(); err != nil {
		log.Fatal("docker sources init failed", "error", err)
	}

	// --- API 处理器 ---
	healthHandler := api.NewHealthHandler()
	healthHandler.Register(mux)

	systemHandler := api.NewSystemHandler(log, store)
	systemHandler.Register(mux)

	serversHandler := api.NewServersHandler(log, store)
	serversHandler.Register(mux)

	softwareHandler := api.NewSoftwareHandler(log, store, swStore, store.DB())
	softwareHandler.Register(mux)

	settingsHandler := api.NewSettingsHandler(log, store.DB(), dataDir)
	settingsHandler.Register(mux)

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

	s := &Server{
		cfg:       cfg,
		log:       log,
		httpSrv:   httpSrv,
		hub:       hub,
		store:     store,
		collector: host.NewCollector(log, store),
	}
	// 设置推送控制器引用
	settingsHandler.SetPushController(s)
	return s
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
	err := s.httpSrv.Shutdown(ctx)
	// HTTP 服务关闭后，优雅关闭全部池化 SSH 连接
	remote.CloseAllSSH()
	return err
}

// CloseStore 关闭存储层（数据库连接）。
func (s *Server) CloseStore() error {
	if s.store != nil {
		return s.store.Close()
	}
	return nil
}

// PausePush 暂停/恢复系统快照推送循环。
func (s *Server) PausePush(paused bool) {
	s.pushPaused.Store(paused)
	if paused {
		s.log.Info("push loop paused")
	} else {
		s.log.Info("push loop resumed")
	}
}

// pushLoop 定时采集所有服务器的系统快照并按 serverID 分发到 WebSocket 客户端。
// 当 pushPaused 为 true 时跳过采集（用户不在系统信息页）。
func (s *Server) pushLoop() {
	interval := time.Duration(s.cfg.Server.PushInterval) * time.Second
	if interval < 1*time.Second {
		interval = 3 * time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for range ticker.C {
		// 暂停时跳过采集，减少不必要的 SSH 连接和日志
		if s.pushPaused.Load() {
			continue
		}

		// 本机 + 所有已配置服务器
		serverIDs := []string{host.LocalServerID}
		for _, srv := range s.store.List() {
			serverIDs = append(serverIDs, srv.ID)
		}

		for _, id := range serverIDs {
			info, err := s.collector.Snapshot(id)
			if err != nil {
				s.log.Debugw("push snapshot failed", "server", id, "error", err)
				continue
			}
			s.hub.Broadcast(info, id)
		}
	}
}
