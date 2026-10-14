// Package host 管理受管服务器的配置集合，并提供统一的信息采集入口。
package host

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"time"

	"github.com/smash-hq/server-management/internal/storage"
)

// Server 是 storage.Server 的类型别名，保持向后兼容。
type Server = storage.Server

// 导出常量供其他包使用。
const (
	DefaultSSHPort = 22
	LocalServerID  = "srv_local"
)

// Store 是服务器配置的存储抽象。
// 底层使用 SQLite 持久化，对外保持与原来 JSON 实现相同的接口。
type Store struct {
	inner *storage.ServerStore
	db    *storage.DB
}

// NewStore 创建存储实例并初始化数据库。
// 如果数据库为空，自动生成本机服务器条目。
// dataDir 是存放数据库文件的目录。
func NewStore(dataDir string) (*Store, error) {
	db, err := storage.New(dataDir)
	if err != nil {
		return nil, err
	}

	// 初始化默认数据（本机服务器 + 软件源）
	if err := db.InitDefaultData(); err != nil {
		db.Close()
		return nil, fmt.Errorf("init default data: %w", err)
	}

	inner := storage.NewServerStore(db)

	// 兜底：如果服务器表仍为空，写入本机条目（启动期引导，使用 Background）
	if inner.Count(context.Background()) == 0 {
		srv := buildLocalServer()
		_ = inner.Add(context.Background(), srv)
	}

	return &Store{inner: inner, db: db}, nil
}

// Close 关闭底层数据库连接。
func (s *Store) Close() error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

// List 返回所有服务器配置（按创建时间排序）。
func (s *Store) List(ctx context.Context) ([]*Server, error) {
	return s.inner.List(ctx)
}

// Get 按 ID 查找服务器。
func (s *Store) Get(ctx context.Context, id string) (*Server, bool) {
	return s.inner.Get(ctx, id)
}

// Add 新增服务器配置。
func (s *Store) Add(ctx context.Context, srv *Server) error {
	return s.inner.Add(ctx, srv)
}

// Update 修改已有配置。
func (s *Store) Update(ctx context.Context, srv *Server) error {
	return s.inner.Update(ctx, srv)
}

// Delete 移除配置。
func (s *Store) Delete(ctx context.Context, id string) error {
	return s.inner.Delete(ctx, id)
}

// Count 返回服务器数量。
func (s *Store) Count(ctx context.Context) int {
	return s.inner.Count(ctx)
}

// DB 返回底层存储的数据库实例（供软件存储等共享使用）。
func (s *Store) DB() *storage.DB {
	return s.db
}

// --- 内部方法 ---

func buildLocalServer() *Server {
	hostname, _ := os.Hostname()
	if hostname == "" {
		hostname = "localhost"
	}

	username := os.Getenv("USER")
	if username == "" {
		username = os.Getenv("USERNAME")
	}
	if username == "" {
		username = "root"
	}

	return &Server{
		ID:        LocalServerID,
		Name:      fmt.Sprintf("%s (%s/%s)", hostname, runtime.GOOS, runtime.GOARCH),
		Type:      "local",
		Host:      "127.0.0.1",
		Port:      DefaultSSHPort,
		Username:  username,
		AuthType:  "local",
		CreatedAt: time.Now().Unix(),
	}
}
