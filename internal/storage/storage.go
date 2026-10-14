// Package storage 提供基于 SQLite 的本地持久化存储。
// 使用 modernc.org/sqlite（纯 Go 实现，无需 CGO），支持静态交叉编译。
package storage

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// DB 封装 SQLite 数据库连接与初始化逻辑。
type DB struct {
	db *sql.DB
}

// DefaultQueryTimeout 是单次数据库操作的默认超时（调用方未设 deadline 时生效）。
const DefaultQueryTimeout = 10 * time.Second

// withQueryTimeout 在调用方未设置 deadline 时套用默认超时。
func withQueryTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if _, ok := ctx.Deadline(); ok {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, DefaultQueryTimeout)
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

// migrate 创建表结构（幂等），并对已有数据库执行增量变更。
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

	CREATE TABLE IF NOT EXISTS languages (
		id            TEXT PRIMARY KEY,
		name          TEXT NOT NULL,
		category      TEXT NOT NULL DEFAULT 'runtime',
		description   TEXT NOT NULL DEFAULT '',
		icon          TEXT NOT NULL DEFAULT '',
		version_cmd   TEXT NOT NULL DEFAULT '',
		install_cmd   TEXT NOT NULL DEFAULT '',
		uninstall_cmd TEXT NOT NULL DEFAULT '',
		created_at    INTEGER NOT NULL DEFAULT 0,
		updated_at    INTEGER NOT NULL DEFAULT 0
	);

	CREATE TABLE IF NOT EXISTS docker_sources (
		id          TEXT PRIMARY KEY,
		name        TEXT NOT NULL,
		url         TEXT NOT NULL,
		enabled     INTEGER NOT NULL DEFAULT 1,
		is_default  INTEGER NOT NULL DEFAULT 0,
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
	CREATE INDEX IF NOT EXISTS idx_languages_category ON languages(category);
	`

	// 启动期引导，无请求 ctx，使用 Background
	_, err := d.db.ExecContext(context.Background(), schema)
	if err != nil {
		return err
	}

	// 增量迁移：为已有数据库补上新加列（忽略已存在错误）
	_ = d.addColumnIfNotExists("languages", "uninstall_cmd", "TEXT NOT NULL DEFAULT ''")
	_ = d.addColumnIfNotExists("software_packages", "versions", "TEXT NOT NULL DEFAULT '[]'")
	_ = d.addColumnIfNotExists("languages", "versions", "TEXT NOT NULL DEFAULT '[]'")
	return nil
}

// addColumnIfNotExists 给表追加列（若列已存在则忽略错误）。
func (d *DB) addColumnIfNotExists(table, column, definition string) error {
	_, err := d.db.ExecContext(context.Background(), fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", table, column, definition))
	if err != nil {
		// SQLite 已存在该列时会返回 duplicate column error，忽略即可
		if strings.Contains(err.Error(), "duplicate column") {
			return nil
		}
		return err
	}
	return nil
}

// InitDefaultData 写入首次启动的默认数据（幂等）。
// 属启动期引导，内部使用 context.Background()。
func (d *DB) InitDefaultData() error {
	ctx := context.Background()
	now := time.Now().Unix()

	// 本机服务器（INSERT OR IGNORE 保证幂等）
	_, err := d.db.ExecContext(ctx, `
		INSERT OR IGNORE INTO servers (id, name, type, host, port, username, auth_type, created_at)
		VALUES ('srv_local', ?, 'local', '127.0.0.1', 22, ?, 'local', ?)`,
		localHostname(), localUser(), now)
	if err != nil {
		return fmt.Errorf("init local server: %w", err)
	}

	// 默认软件源（仅管理普通软件安装镜像，Docker 引擎仓库与 registry 镜像加速不在其列）
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
		{"src_nodejs_official", "Node.js 官方源", "repo", "https://deb.nodesource.com/"},
	}
	// Docker 引擎仓库不属于软件源范畴（Docker 相关配置由「Docker 镜像源」管理），清理历史种子数据
	d.db.ExecContext(ctx, `DELETE FROM software_sources WHERE id IN ('src_docker_official', 'src_docker_aliyun')`)
	for _, s := range sources {
		d.db.ExecContext(ctx, `
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

// GetMeta 读取 meta 表中指定键的值（未命中返回 sql.ErrNoRows）。
func (d *DB) GetMeta(ctx context.Context, key string) (string, error) {
	ctx, cancel := withQueryTimeout(ctx)
	defer cancel()
	var val string
	err := d.db.QueryRowContext(ctx, `SELECT value FROM meta WHERE key = ?`, key).Scan(&val)
	return val, err
}

// SetMeta 写入 meta 表中指定键的值（upsert）。
func (d *DB) SetMeta(ctx context.Context, key, value string) error {
	ctx, cancel := withQueryTimeout(ctx)
	defer cancel()
	_, err := d.db.ExecContext(ctx, `
		INSERT INTO meta (key, value) VALUES (?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value`, key, value)
	return err
}

// Raw 返回底层 *sql.DB（供高级查询使用，须使用 Context 变体方法）。
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
