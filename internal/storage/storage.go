// Package storage 提供基于 SQLite 的本地持久化存储。
// 使用 modernc.org/sqlite（纯 Go 实现，无需 CGO），支持静态交叉编译。
package storage

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

// DB 封装 SQLite 数据库连接与初始化逻辑。
type DB struct {
	db *sql.DB
}

// New 创建并初始化数据库。
// dataDir 是存放数据库文件的目录，文件名为 smp.db。
func New(dataDir string) (*DB, error) {
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("create data dir: %w", err)
	}

	dbPath := filepath.Join(dataDir, "smp.db")
	dsn := fmt.Sprintf("file:%s?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(ON)", dbPath)

	sqlDB, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	// 连接池设置（SQLite 单写者，限制为 1 避免锁冲突）
	sqlDB.SetMaxOpenConns(1)
	sqlDB.SetMaxIdleConns(1)
	sqlDB.SetConnMaxLifetime(0)

	d := &DB{db: sqlDB}
	if err := d.migrate(); err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}

	return d, nil
}

// migrate 创建表结构（幂等）。
func (d *DB) migrate() error {
	schema := `
	CREATE TABLE IF NOT EXISTS servers (
		id          TEXT PRIMARY KEY,
		name        TEXT NOT NULL,
		type        TEXT NOT NULL DEFAULT 'local',
		host        TEXT NOT NULL DEFAULT '127.0.0.1',
		port        INTEGER NOT NULL DEFAULT 22,
		username    TEXT NOT NULL DEFAULT '',
		auth_type   TEXT NOT NULL DEFAULT 'local',
		password    TEXT NOT NULL DEFAULT '',
		key_path    TEXT NOT NULL DEFAULT '',
		created_at  INTEGER NOT NULL DEFAULT 0
	);

	CREATE TABLE IF NOT EXISTS software_packages (
		id            TEXT PRIMARY KEY,
		name          TEXT NOT NULL,
		category      TEXT NOT NULL DEFAULT 'tool',
		description   TEXT NOT NULL DEFAULT '',
		version       TEXT NOT NULL DEFAULT '',
		icon          TEXT NOT NULL DEFAULT '',
		install_cmd   TEXT NOT NULL DEFAULT '{}',
		uninstall_cmd TEXT NOT NULL DEFAULT '{}',
		status_cmd    TEXT NOT NULL DEFAULT '',
		version_cmd   TEXT NOT NULL DEFAULT '',
		start_cmd     TEXT NOT NULL DEFAULT '',
		stop_cmd      TEXT NOT NULL DEFAULT '',
		restart_cmd   TEXT NOT NULL DEFAULT '',
		created_at    INTEGER NOT NULL DEFAULT 0,
		updated_at    INTEGER NOT NULL DEFAULT 0
	);

	CREATE TABLE IF NOT EXISTS software_sources (
		id          TEXT PRIMARY KEY,
		name        TEXT NOT NULL,
		type        TEXT NOT NULL DEFAULT 'mirror',
		url         TEXT NOT NULL,
		enabled     INTEGER NOT NULL DEFAULT 1,
		priority    INTEGER NOT NULL DEFAULT 100,
		created_at  INTEGER NOT NULL DEFAULT 0,
		updated_at  INTEGER NOT NULL DEFAULT 0
	);

	CREATE TABLE IF NOT EXISTS meta (
		key   TEXT PRIMARY KEY,
		value TEXT NOT NULL DEFAULT ''
	);

	CREATE INDEX IF NOT EXISTS idx_servers_type ON servers(type);
	CREATE INDEX IF NOT EXISTS idx_servers_created ON servers(created_at);
	CREATE INDEX IF NOT EXISTS idx_sw_packages_category ON software_packages(category);
	CREATE INDEX IF NOT EXISTS idx_sw_sources_type ON software_sources(type);
	`

	_, err := d.db.Exec(schema)
	return err
}

// InitDefaultData 写入首次启动的默认数据（幂等）。
func (d *DB) InitDefaultData() error {
	now := time.Now().Unix()

	// 本机服务器（INSERT OR IGNORE 保证幂等）
	_, err := d.db.Exec(`
		INSERT OR IGNORE INTO servers (id, name, type, host, port, username, auth_type, created_at)
		VALUES ('srv_local', ?, 'local', '127.0.0.1', 22, ?, 'local', ?)`,
		localHostname(), localUser(), now)
	if err != nil {
		return fmt.Errorf("init local server: %w", err)
	}

	// 默认软件源
	sources := []struct {
		id   string
		name string
		typ  string
		url  string
	}{
		{"src_ubuntu_official", "Ubuntu 官方源", "mirror", "http://archive.ubuntu.com/ubuntu/"},
		{"src_ubuntu_aliyun", "阿里云 Ubuntu 镜像", "mirror", "https://mirrors.aliyun.com/ubuntu/"},
		{"src_centos_official", "CentOS 官方源", "mirror", "http://mirror.centos.org/centos/"},
		{"src_centos_aliyun", "阿里云 CentOS 镜像", "mirror", "https://mirrors.aliyun.com/centos/"},
		{"src_docker_official", "Docker 官方源", "repo", "https://download.docker.com/linux/"},
		{"src_docker_aliyun", "阿里云 Docker 镜像", "repo", "https://mirrors.aliyun.com/docker-ce/linux/"},
		{"src_nodejs_official", "Node.js 官方源", "repo", "https://deb.nodesource.com/"},
	}
	for _, s := range sources {
		d.db.Exec(`
			INSERT OR IGNORE INTO software_sources (id, name, type, url, enabled, priority, created_at, updated_at)
			VALUES (?, ?, ?, ?, 1, 100, ?, ?)`,
			s.id, s.name, s.typ, s.url, now, now)
	}

	return nil
}

// Close 关闭数据库连接。
func (d *DB) Close() error {
	return d.db.Close()
}

// Raw 返回底层 *sql.DB（供高级查询使用）。
func (d *DB) Raw() *sql.DB {
	return d.db
}

// localHostname 获取本机主机名。
func localHostname() string {
	h, _ := os.Hostname()
	if h == "" {
		h = "localhost"
	}
	return h
}

func localUser() string {
	u := os.Getenv("USER")
	if u == "" {
		u = os.Getenv("USERNAME")
	}
	if u == "" {
		u = "root"
	}
	return u
}
