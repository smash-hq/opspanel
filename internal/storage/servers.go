package storage

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"time"
)

// Server 代表一台受管服务器的配置。
// 定义在 storage 层，供 host 包复用。
type Server struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Type      string `json:"type"`
	Host      string `json:"host"`
	Port      int    `json:"port"`
	Username  string `json:"username"`
	AuthType  string `json:"auth_type"`
	Password  string `json:"password,omitempty"`
	KeyPath   string `json:"key_path,omitempty"`
	CreatedAt int64  `json:"created_at"`
}

// ServerStore 是基于 SQLite 的服务器配置存储。
type ServerStore struct {
	db *DB
}

// NewServerStore 创建服务器存储实例。
func NewServerStore(db *DB) *ServerStore {
	return &ServerStore{db: db}
}

// List 返回所有服务器配置（按创建时间排序）。
func (s *ServerStore) List() []*Server {
	rows, err := s.db.Raw().Query(`
		SELECT id, name, type, host, port, username, auth_type, password, key_path, created_at
		FROM servers ORDER BY created_at ASC`)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var servers []*Server
	for rows.Next() {
		srv := &Server{}
		if err := rows.Scan(&srv.ID, &srv.Name, &srv.Type, &srv.Host, &srv.Port,
			&srv.Username, &srv.AuthType, &srv.Password, &srv.KeyPath, &srv.CreatedAt); err != nil {
			continue
		}
		servers = append(servers, srv)
	}
	return servers
}

// Get 按 ID 查找服务器。
func (s *ServerStore) Get(id string) (*Server, bool) {
	srv := &Server{}
	err := s.db.Raw().QueryRow(`
		SELECT id, name, type, host, port, username, auth_type, password, key_path, created_at
		FROM servers WHERE id = ?`, id).Scan(
		&srv.ID, &srv.Name, &srv.Type, &srv.Host, &srv.Port,
		&srv.Username, &srv.AuthType, &srv.Password, &srv.KeyPath, &srv.CreatedAt)
	if err != nil {
		return nil, false
	}
	return srv, true
}

// Add 新增服务器配置。
func (s *ServerStore) Add(srv *Server) error {
	if srv.ID == "" {
		srv.ID = generateID()
	}
	if srv.Port == 0 {
		srv.Port = 22
	}
	if srv.CreatedAt == 0 {
		srv.CreatedAt = time.Now().Unix()
	}

	_, err := s.db.Raw().Exec(`
		INSERT INTO servers (id, name, type, host, port, username, auth_type, password, key_path, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		srv.ID, srv.Name, srv.Type, srv.Host, srv.Port,
		srv.Username, srv.AuthType, srv.Password, srv.KeyPath, srv.CreatedAt)
	if err != nil {
		return fmt.Errorf("insert server: %w", err)
	}
	return nil
}

// Update 修改已有配置。
func (s *ServerStore) Update(srv *Server) error {
	if srv.CreatedAt == 0 {
		if existing, ok := s.Get(srv.ID); ok {
			srv.CreatedAt = existing.CreatedAt
		}
	}

	res, err := s.db.Raw().Exec(`
		UPDATE servers SET name=?, type=?, host=?, port=?, username=?, auth_type=?, password=?, key_path=?
		WHERE id=?`,
		srv.Name, srv.Type, srv.Host, srv.Port, srv.Username, srv.AuthType, srv.Password, srv.KeyPath, srv.ID)
	if err != nil {
		return fmt.Errorf("update server: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return os.ErrNotExist
	}
	return nil
}

// Delete 移除配置。
func (s *ServerStore) Delete(id string) error {
	if id == "srv_local" {
		return fmt.Errorf("cannot delete local server entry")
	}
	res, err := s.db.Raw().Exec(`DELETE FROM servers WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete server: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return os.ErrNotExist
	}
	return nil
}

// Count 返回服务器数量。
func (s *ServerStore) Count() int {
	var count int
	_ = s.db.Raw().QueryRow(`SELECT COUNT(*) FROM servers`).Scan(&count)
	return count
}

func generateID() string {
	return fmt.Sprintf("srv_%d_%s", time.Now().UnixNano(), randomStr(6))
}

func randomStr(n int) string {
	b := make([]byte, n/2+1)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)[:n]
}
