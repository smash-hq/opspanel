package storage

import (
	"encoding/json"
	"fmt"
	"time"
)

// SoftwarePackage 是软件包定义（存储层）。
type SoftwarePackage struct {
	ID            string              `json:"id"`
	Name          string              `json:"name"`
	Category      string              `json:"category"`
	Description   string              `json:"description"`
	Version       string              `json:"version"`
	Icon          string              `json:"icon"`
	InstallCmd    map[string][]string `json:"install_cmd"`
	UninstallCmd  map[string][]string `json:"uninstall_cmd"`
	StatusCmd     string              `json:"status_cmd"`
	VersionCmd    string              `json:"version_cmd"`
	StartCmd      string              `json:"start_cmd"`
	StopCmd       string              `json:"stop_cmd"`
	RestartCmd    string              `json:"restart_cmd"`
}

// SoftwareStore 管理软件包定义与镜像源。
type SoftwareStore struct {
	db *DB
}

// NewSoftwareStore 创建软件存储实例。
func NewSoftwareStore(db *DB) *SoftwareStore {
	return &SoftwareStore{db: db}
}

// InitDefaultPackages 写入内置软件包目录（幂等）。
func (s *SoftwareStore) InitDefaultPackages(packages []*SoftwarePackage) error {
	now := time.Now().Unix()
	for _, pkg := range packages {
		installJSON, _ := json.Marshal(pkg.InstallCmd)
		uninstallJSON, _ := json.Marshal(pkg.UninstallCmd)

		_, err := s.db.Raw().Exec(`
			INSERT OR REPLACE INTO software_packages
			(id, name, category, description, version, icon, install_cmd, uninstall_cmd,
			 status_cmd, version_cmd, start_cmd, stop_cmd, restart_cmd, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			pkg.ID, pkg.Name, pkg.Category, pkg.Description, pkg.Version, pkg.Icon,
			string(installJSON), string(uninstallJSON),
			pkg.StatusCmd, pkg.VersionCmd, pkg.StartCmd, pkg.StopCmd, pkg.RestartCmd,
			now, now)
		if err != nil {
			return fmt.Errorf("insert package %s: %w", pkg.ID, err)
		}
	}
	return nil
}

// ListPackages 返回所有软件包。
func (s *SoftwareStore) ListPackages() []*SoftwarePackage {
	rows, err := s.db.Raw().Query(`
		SELECT id, name, category, description, version, icon, install_cmd, uninstall_cmd,
		       status_cmd, version_cmd, start_cmd, stop_cmd, restart_cmd
		FROM software_packages ORDER BY category, name`)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var packages []*SoftwarePackage
	for rows.Next() {
		var id, name, category, description, version, icon string
		var installJSON, uninstallJSON string
		var statusCmd, versionCmd, startCmd, stopCmd, restartCmd string
		if err := rows.Scan(&id, &name, &category, &description, &version, &icon,
			&installJSON, &uninstallJSON, &statusCmd, &versionCmd,
			&startCmd, &stopCmd, &restartCmd); err != nil {
			continue
		}
		pkg := &SoftwarePackage{
			ID:          id,
			Name:        name,
			Category:    category,
			Description: description,
			Version:     version,
			Icon:        icon,
			StatusCmd:   statusCmd,
			VersionCmd:  versionCmd,
			StartCmd:    startCmd,
			StopCmd:     stopCmd,
			RestartCmd:  restartCmd,
		}
		_ = json.Unmarshal([]byte(installJSON), &pkg.InstallCmd)
		_ = json.Unmarshal([]byte(uninstallJSON), &pkg.UninstallCmd)
		packages = append(packages, pkg)
	}
	return packages
}

// GetPackage 按 ID 获取软件包。
func (s *SoftwareStore) GetPackage(id string) (*SoftwarePackage, bool) {
	pkgs := s.ListPackages()
	for _, pkg := range pkgs {
		if pkg.ID == id {
			return pkg, true
		}
	}
	return nil, false
}

// UpdatePackage 更新软件包定义。
func (s *SoftwareStore) UpdatePackage(pkg *SoftwarePackage) error {
	installJSON, _ := json.Marshal(pkg.InstallCmd)
	uninstallJSON, _ := json.Marshal(pkg.UninstallCmd)
	now := time.Now().Unix()

	_, err := s.db.Raw().Exec(`
		UPDATE software_packages SET name=?, category=?, description=?, version=?, icon=?,
			install_cmd=?, uninstall_cmd=?, status_cmd=?, version_cmd=?, start_cmd=?, stop_cmd=?,
			restart_cmd=?, updated_at=? WHERE id=?`,
		pkg.Name, pkg.Category, pkg.Description, pkg.Version, pkg.Icon,
		string(installJSON), string(uninstallJSON),
		pkg.StatusCmd, pkg.VersionCmd, pkg.StartCmd, pkg.StopCmd, pkg.RestartCmd,
		now, pkg.ID)
	return err
}

// AddPackage 新增自定义软件包。
func (s *SoftwareStore) AddPackage(pkg *SoftwarePackage) error {
	if pkg.ID == "" {
		return fmt.Errorf("package id is required")
	}
	installJSON, _ := json.Marshal(pkg.InstallCmd)
	uninstallJSON, _ := json.Marshal(pkg.UninstallCmd)
	now := time.Now().Unix()

	_, err := s.db.Raw().Exec(`
		INSERT INTO software_packages
		(id, name, category, description, version, icon, install_cmd, uninstall_cmd,
		 status_cmd, version_cmd, start_cmd, stop_cmd, restart_cmd, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		pkg.ID, pkg.Name, pkg.Category, pkg.Description, pkg.Version, pkg.Icon,
		string(installJSON), string(uninstallJSON),
		pkg.StatusCmd, pkg.VersionCmd, pkg.StartCmd, pkg.StopCmd, pkg.RestartCmd,
		now, now)
	return err
}

// DeletePackage 删除软件包。
func (s *SoftwareStore) DeletePackage(id string) error {
	_, err := s.db.Raw().Exec(`DELETE FROM software_packages WHERE id = ?`, id)
	return err
}

// --- 镜像源管理 ---

// SourceRecord 是数据库中的软件源行。
type SourceRecord struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Type      string `json:"type"`
	URL       string `json:"url"`
	Enabled   bool   `json:"enabled"`
	Priority  int    `json:"priority"`
	CreatedAt int64  `json:"created_at"`
	UpdatedAt int64  `json:"updated_at"`
}

// ListSources 返回所有软件源。
func (s *SoftwareStore) ListSources() []*SourceRecord {
	rows, err := s.db.Raw().Query(`
		SELECT id, name, type, url, enabled, priority, created_at, updated_at
		FROM software_sources ORDER BY priority ASC`)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var sources []*SourceRecord
	for rows.Next() {
		var src SourceRecord
		var enabled int
		if err := rows.Scan(&src.ID, &src.Name, &src.Type, &src.URL, &enabled,
			&src.Priority, &src.CreatedAt, &src.UpdatedAt); err != nil {
			continue
		}
		src.Enabled = enabled != 0
		sources = append(sources, &src)
	}
	return sources
}

// GetSource 按 ID 获取软件源。
func (s *SoftwareStore) GetSource(id string) (*SourceRecord, bool) {
	var src SourceRecord
	var enabled int
	err := s.db.Raw().QueryRow(`
		SELECT id, name, type, url, enabled, priority, created_at, updated_at
		FROM software_sources WHERE id = ?`, id).Scan(
		&src.ID, &src.Name, &src.Type, &src.URL, &enabled,
		&src.Priority, &src.CreatedAt, &src.UpdatedAt)
	if err != nil {
		return nil, false
	}
	src.Enabled = enabled != 0
	return &src, true
}

// AddSource 新增软件源。
func (s *SoftwareStore) AddSource(src *SourceRecord) error {
	now := time.Now().Unix()
	enabled := 0
	if src.Enabled {
		enabled = 1
	}
	_, err := s.db.Raw().Exec(`
		INSERT INTO software_sources (id, name, type, url, enabled, priority, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		src.ID, src.Name, src.Type, src.URL, enabled, src.Priority, now, now)
	return err
}

// UpdateSource 更新软件源。
func (s *SoftwareStore) UpdateSource(src *SourceRecord) error {
	now := time.Now().Unix()
	enabled := 0
	if src.Enabled {
		enabled = 1
	}
	_, err := s.db.Raw().Exec(`
		UPDATE software_sources SET name=?, type=?, url=?, enabled=?, priority=?, updated_at=?
		WHERE id=?`,
		src.Name, src.Type, src.URL, enabled, src.Priority, now, src.ID)
	return err
}

// DeleteSource 删除软件源。
func (s *SoftwareStore) DeleteSource(id string) error {
	_, err := s.db.Raw().Exec(`DELETE FROM software_sources WHERE id = ?`, id)
	return err
}
