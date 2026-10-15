package storage

import (
	"context"
	"fmt"
	"time"
)

// DeployedApp 是用户部署的应用服务（存储层）。
type DeployedApp struct {
	ID        string `json:"id"`
	ServerID  string `json:"server_id"`
	Name      string `json:"name"`
	Language  string `json:"language"`   // java / python / go
	DeployDir string `json:"deploy_dir"` // 远端部署目录
	Artifacts string `json:"artifacts"`  // 已上传文件相对路径 JSON 数组
	StartCmd  string `json:"start_cmd"`  // 启动命令（在部署目录内执行）
	LogFile   string `json:"log_file"`   // 日志文件路径（绝对路径或相对部署目录）
	PID       int    `json:"pid"`
	Status    string `json:"status"`     // running / stopped / failed / unknown
	LastError string `json:"last_error"` // 上次启动失败原因（成功启动/手动停止后清空）
	Remark    string `json:"remark"`
	CreatedAt int64  `json:"created_at"`
	UpdatedAt int64  `json:"updated_at"`
}

// AppStore 管理部署应用记录。
type AppStore struct {
	db *DB
}

// NewAppStore 创建应用部署存储实例。
func NewAppStore(db *DB) *AppStore {
	return &AppStore{db: db}
}

// GenerateAppID 生成部署应用 ID。
func GenerateAppID() string {
	return fmt.Sprintf("app_%d_%s", time.Now().UnixNano(), randomStr(6))
}

// appCols 是 deployed_apps 的查询列清单（Scan 顺序与之一致）。
const appCols = `id, server_id, name, language, deploy_dir, artifacts, start_cmd, log_file, pid, status, last_error, remark, created_at, updated_at`

// ListApps 返回指定服务器的部署应用（serverID 为空返回全部）。
func (s *AppStore) ListApps(ctx context.Context, serverID string) []*DeployedApp {
	ctx, cancel := withQueryTimeout(ctx)
	defer cancel()

	query := `SELECT ` + appCols + ` FROM deployed_apps`
	args := []any{}
	if serverID != "" {
		query += ` WHERE server_id = ?`
		args = append(args, serverID)
	}
	query += ` ORDER BY created_at DESC`

	rows, err := s.db.Raw().QueryContext(ctx, query, args...)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var apps []*DeployedApp
	for rows.Next() {
		app := scanApp(rows)
		if app != nil {
			apps = append(apps, app)
		}
	}
	return apps
}

// GetApp 按 ID 查询部署应用。
func (s *AppStore) GetApp(ctx context.Context, id string) (*DeployedApp, error) {
	ctx, cancel := withQueryTimeout(ctx)
	defer cancel()

	row := s.db.Raw().QueryRowContext(ctx,
		`SELECT `+appCols+` FROM deployed_apps WHERE id = ?`, id)
	app := scanApp(row)
	if app == nil {
		return nil, fmt.Errorf("app not found: %s", id)
	}
	return app, nil
}

// AddApp 新增部署应用记录。
func (s *AppStore) AddApp(ctx context.Context, app *DeployedApp) error {
	ctx, cancel := withQueryTimeout(ctx)
	defer cancel()

	now := time.Now().Unix()
	app.CreatedAt, app.UpdatedAt = now, now
	_, err := s.db.Raw().ExecContext(ctx, `
		INSERT INTO deployed_apps
		(id, server_id, name, language, deploy_dir, artifacts, start_cmd, log_file, pid, status, last_error, remark, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		app.ID, app.ServerID, app.Name, app.Language, app.DeployDir, app.Artifacts,
		app.StartCmd, app.LogFile, app.PID, app.Status, app.LastError, app.Remark, now, now)
	return err
}

// UpdateApp 更新部署应用记录（保留 created_at，刷新 updated_at）。
func (s *AppStore) UpdateApp(ctx context.Context, app *DeployedApp) error {
	ctx, cancel := withQueryTimeout(ctx)
	defer cancel()

	_, err := s.db.Raw().ExecContext(ctx, `
		UPDATE deployed_apps SET
			name = ?, language = ?, deploy_dir = ?, artifacts = ?, start_cmd = ?,
			log_file = ?, pid = ?, status = ?, last_error = ?, remark = ?, updated_at = ?
		WHERE id = ?`,
		app.Name, app.Language, app.DeployDir, app.Artifacts, app.StartCmd,
		app.LogFile, app.PID, app.Status, app.LastError, app.Remark, time.Now().Unix(), app.ID)
	return err
}

// DeleteApp 删除部署应用记录。
func (s *AppStore) DeleteApp(ctx context.Context, id string) error {
	ctx, cancel := withQueryTimeout(ctx)
	defer cancel()

	_, err := s.db.Raw().ExecContext(ctx, `DELETE FROM deployed_apps WHERE id = ?`, id)
	return err
}

// appScanner 抽象 *sql.Rows / *sql.Row 的 Scan 能力。
type appScanner interface {
	Scan(dest ...any) error
}

// scanApp 从行中扫描部署应用记录；失败返回 nil。
func scanApp(row appScanner) *DeployedApp {
	var app DeployedApp
	if err := row.Scan(&app.ID, &app.ServerID, &app.Name, &app.Language, &app.DeployDir,
		&app.Artifacts, &app.StartCmd, &app.LogFile, &app.PID, &app.Status,
		&app.LastError, &app.Remark, &app.CreatedAt, &app.UpdatedAt); err != nil {
		return nil
	}
	return &app
}
