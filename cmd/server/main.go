// Server Plane 服务器管理面板 - 入口
//
// 启动后访问 http://<host>:18080 查看控制台，
// REST API: /api/v1/health, /api/v1/system/info
// WebSocket: /ws
package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/smash-hq/server-management/internal/config"
	"github.com/smash-hq/server-management/internal/logger"
	"github.com/smash-hq/server-management/internal/server"
	"github.com/smash-hq/server-management/web"
	"github.com/spf13/cobra"
)

// version 与 gitCommit 由构建时通过 -ldflags 注入（见 Makefile）。
var (
	version   = "dev"
	gitCommit = "unknown"

	cfgFile string
	cfg     *config.Config
	log     *logger.Logger
)

func main() {
	rootCmd := &cobra.Command{
		Use:     "smp-server",
		Short:   "Server Plane — 服务器管理面板",
		Long:    "单机优先的 Go 原生服务器运维平台，提供浏览器控制台与 REST/WebSocket API。",
		Version: version + " (commit: " + gitCommit + ")",
		RunE:    runServer,
	}

	rootCmd.PersistentFlags().StringVarP(&cfgFile, "config", "c", "", "配置文件路径 (默认: ./config.yaml)")

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func runServer(cmd *cobra.Command, args []string) error {
	// 加载配置
	var err error
	cfg, err = config.Load(cfgFile)
	if err != nil {
		return err
	}

	// 初始化日志
	log = logger.New(cfg.Log.Level, cfg.Log.Format)
	defer log.Sync()

	log.Infow("config loaded",
		"addr", cfg.Server.Host, "port", cfg.Server.Port,
		"push_interval", cfg.Server.PushInterval,
	)

	// 数据存储目录（服务器配置 JSON）
	dataDir := "data"
	if cfg.Server.DataDir != "" {
		dataDir = cfg.Server.DataDir
	}

	// 创建并启动服务
	srv := server.New(cfg, log, web.FS, dataDir)

	// 优雅关闭：监听 SIGINT / SIGTERM
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		if err := srv.Start(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	log.Infow("server is ready", "url", "http://"+cfg.Server.Host+":"+strconv.Itoa(cfg.Server.Port))

	// 等待关闭信号或启动错误
	select {
	case <-ctx.Done():
		log.Infow("shutdown signal received")
	case err := <-errCh:
		log.Errorw("server error", "error", err)
		return err
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), time.Duration(cfg.Server.ShutdownTimeout)*time.Second)
	defer cancel()
	srv.Shutdown(shutdownCtx)
	// 关闭数据库连接
	if storeErr := srv.CloseStore(); storeErr != nil {
		log.Errorw("store close failed", "error", storeErr)
	}
	return nil
}
