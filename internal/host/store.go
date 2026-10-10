// Package host 管理受管服务器的配置集合，并提供统一的信息采集入口。
package host

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Server 代表一台受管服务器的配置。
type Server struct {
	ID        string `json:"id"`         // 唯一标识 (uuid)
	Name      string `json:"name"`       // 显示名称，如 "生产服务器-01"
	Type      string `json:"type"`       // "local" 本机 / "remote" 远程 SSH
	Host      string `json:"host"`       // IP 或域名
	Port      int    `json:"port"`       // SSH 端口，默认 22
	Username  string `json:"username"`   // SSH 用户名
	AuthType  string `json:"auth_type"`  // "password" 或 "key"
	Password  string `json:"password,omitempty"`
	KeyPath   string `json:"key_path,omitempty"` // 私钥路径
	CreatedAt int64  `json:"created_at"`         // Unix 秒
}

// DefaultSSHPort 是默认 SSH 端口。
const DefaultSSHPort = 22

// Store 是服务器配置的内存缓存 + JSON 持久化。
type Store struct {
	mu       sync.RWMutex
	servers  map[string]*Server
	filePath string
}

// NewStore 创建存储实例并加载已有配置。
// dataDir 是存放 servers.json 的目录。
func NewStore(dataDir string) (*Store, error) {
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, err
	}

	s := &Store{
		servers:  make(map[string]*Server),
		filePath: filepath.Join(dataDir, "servers.json"),
	}
	_ = s.load() // 首次启动可能无文件，忽略错误
	return s, nil
}

// List 返回所有服务器配置（按创建时间排序的副本）。
func (s *Store) List() []*Server {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*Server, 0, len(s.servers))
	for _, srv := range s.servers {
		result = append(result, srv.clone())
	}
	// 按创建时间升序
	for i := 0; i < len(result)-1; i++ {
		for j := i + 1; j < len(result); j++ {
			if result[j].CreatedAt < result[i].CreatedAt {
				result[i], result[j] = result[j], result[i]
			}
		}
	}
	return result
}

// Get 按 ID 查找服务器。
func (s *Store) Get(id string) (*Server, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	srv, ok := s.servers[id]
	if !ok {
		return nil, false
	}
	return srv.clone(), true
}

// Add 新增服务器配置。
func (s *Store) Add(srv *Server) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if srv.ID == "" {
		srv.ID = generateID()
	}
	if srv.Port == 0 {
		srv.Port = DefaultSSHPort
	}
	if srv.CreatedAt == 0 {
		srv.CreatedAt = time.Now().Unix()
	}
	s.servers[srv.ID] = srv
	return s.save()
}

// Update 修改已有配置。
func (s *Store) Update(srv *Server) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.servers[srv.ID]; !ok {
		return os.ErrNotExist
	}
	s.servers[srv.ID] = srv
	return s.save()
}

// Delete 移除配置。
func (s *Store) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.servers[id]; !ok {
		return os.ErrNotExist
	}
	delete(s.servers, id)
	return s.save()
}

// Count 返回服务器数量。
func (s *Store) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.servers)
}

// --- 内部方法 ---

func (s *Server) clone() *Server {
	cp := *s
	return &cp
}

func (s *Store) load() error {
	data, err := os.ReadFile(s.filePath)
	if err != nil {
		return err
	}
	var list []*Server
	if err := json.Unmarshal(data, &list); err != nil {
		return err
	}
	for _, srv := range list {
		s.servers[srv.ID] = srv
	}
	return nil
}

func (s *Store) save() error {
	list := make([]*Server, 0, len(s.servers))
	for _, srv := range s.servers {
		list = append(list, srv)
	}
	data, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.filePath, data, 0644)
}

// generateID 生成简单唯一 ID（时间戳 + 随机后缀）。
func generateID() string {
	return fmt.Sprintf("srv_%d_%s", time.Now().UnixNano(), randomStr(6))
}

func randomStr(n int) string {
	b := make([]byte, n/2+1)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)[:n]
}
