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

// --- 编程语言环境管理 ---

// LanguageRecord 是数据库中的语言环境行。
type LanguageRecord struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Category      string `json:"category"`
	Description   string `json:"description"`
	Icon          string `json:"icon"`
	VersionCmd    string `json:"version_cmd"`
	InstallCmd    string `json:"install_cmd"`
	UninstallCmd  string `json:"uninstall_cmd"`
	CreatedAt     int64  `json:"created_at"`
	UpdatedAt     int64  `json:"updated_at"`
}

// LanguageStore 管理编程语言环境定义。
type LanguageStore struct {
	db *DB
}

// NewLanguageStore 创建语言存储实例。
func NewLanguageStore(db *DB) *LanguageStore {
	return &LanguageStore{db: db}
}

// InitDefaultLanguages 写入内置语言环境定义（幂等）。
func (s *LanguageStore) InitDefaultLanguages() error {
	// 清理不属于编程语言的条目（如 Docker 属于软件包）
	_, _ = s.db.Raw().Exec(`DELETE FROM languages WHERE id = 'docker'`)
	now := time.Now().Unix()
		languages := []*LanguageRecord{
			{ID: "go", Name: "Go", Category: "language", Description: "Google 开发的高性能编译语言", Icon: "🐹", VersionCmd: "go version 2>/dev/null", InstallCmd: "apt-get install -y golang-go || yum install -y golang || dnf install -y golang", CreatedAt: now, UpdatedAt: now},
			{ID: "java", Name: "Java", Category: "language", Description: "广泛使用的面向对象编程语言", Icon: "☕", VersionCmd: "java -version 2>&1 | head -1", InstallCmd: "apt-get install -y default-jdk || yum install -y java-11-openjdk-devel || dnf install -y java-11-openjdk-devel", CreatedAt: now, UpdatedAt: now},
			{ID: "python", Name: "Python", Category: "language", Description: "简洁优雅的解释型编程语言", Icon: "🐍", VersionCmd: "python3 --version 2>/dev/null", InstallCmd: "apt-get install -y python3 python3-pip || yum install -y python3 python3-pip || dnf install -y python3 python3-pip", CreatedAt: now, UpdatedAt: now},
			{ID: "nodejs", Name: "Node.js", Category: "runtime", Description: "基于 Chrome V8 引擎的 JavaScript 运行时", Icon: "⬢", VersionCmd: "node --version 2>/dev/null", InstallCmd: "curl -fsSL https://deb.nodesource.com/setup_lts.x | sudo -E bash - && apt-get install -y nodejs || yum install -y nodejs || dnf install -y nodejs", CreatedAt: now, UpdatedAt: now},
			{ID: "rust", Name: "Rust", Category: "language", Description: "注重安全和性能的系统编程语言", Icon: "🦀", VersionCmd: "rustc --version 2>/dev/null", InstallCmd: "curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh -s -- -y", CreatedAt: now, UpdatedAt: now},
			{ID: "php", Name: "PHP", Category: "language", Description: "广泛使用的服务端脚本语言", Icon: "🐘", VersionCmd: "php --version 2>/dev/null | head -1", InstallCmd: "apt-get install -y php-cli php-fpm || yum install -y php-cli php-fpm || dnf install -y php-cli php-fpm", CreatedAt: now, UpdatedAt: now},
			{ID: "ruby", Name: "Ruby", Category: "language", Description: "简洁优雅的动态编程语言", Icon: "💎", VersionCmd: "ruby --version 2>/dev/null", InstallCmd: "apt-get install -y ruby-full || yum install -y ruby || dnf install -y ruby", CreatedAt: now, UpdatedAt: now},
			{ID: "dotnet", Name: ".NET", Category: "runtime", Description: "微软跨平台开发框架（C#/F#/VB）", Icon: "🔷", VersionCmd: "dotnet --version 2>/dev/null", InstallCmd: "apt-get install -y dotnet-sdk-8.0 || yum install -y dotnet-sdk-8.0 || dnf install -y dotnet-sdk-8.0", CreatedAt: now, UpdatedAt: now},
		}
	for _, lang := range languages {
		_, err := s.db.Raw().Exec(`
			INSERT OR IGNORE INTO languages (id, name, category, description, icon, version_cmd, install_cmd, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			lang.ID, lang.Name, lang.Category, lang.Description, lang.Icon, lang.VersionCmd, lang.InstallCmd, lang.CreatedAt, lang.UpdatedAt)
		if err != nil {
			return fmt.Errorf("insert language %s: %w", lang.ID, err)
		}
	}
	return nil
}

// ListLanguages 返回所有语言环境。
func (s *LanguageStore) ListLanguages() []*LanguageRecord {
	rows, err := s.db.Raw().Query(`SELECT id, name, category, description, icon, version_cmd, install_cmd, uninstall_cmd, created_at, updated_at FROM languages ORDER BY category, name`)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var languages []*LanguageRecord
	for rows.Next() {
		var lang LanguageRecord
		if err := rows.Scan(&lang.ID, &lang.Name, &lang.Category, &lang.Description, &lang.Icon, &lang.VersionCmd, &lang.InstallCmd, &lang.UninstallCmd, &lang.CreatedAt, &lang.UpdatedAt); err != nil {
			continue
		}
		languages = append(languages, &lang)
	}
	return languages
}

// GetLanguage 按 ID 获取语言环境。
func (s *LanguageStore) GetLanguage(id string) (*LanguageRecord, bool) {
	var lang LanguageRecord
	err := s.db.Raw().QueryRow(`SELECT id, name, category, description, icon, version_cmd, install_cmd, uninstall_cmd, created_at, updated_at FROM languages WHERE id = ?`, id).Scan(
		&lang.ID, &lang.Name, &lang.Category, &lang.Description, &lang.Icon, &lang.VersionCmd, &lang.InstallCmd, &lang.UninstallCmd, &lang.CreatedAt, &lang.UpdatedAt)
	if err != nil {
		return nil, false
	}
	return &lang, true
}

// AddLanguage 新增自定义语言环境。
func (s *LanguageStore) AddLanguage(lang *LanguageRecord) error {
	if lang.ID == "" {
		return fmt.Errorf("language id is required")
	}
	now := time.Now().Unix()
	_, err := s.db.Raw().Exec(`
		INSERT INTO languages (id, name, category, description, icon, version_cmd, install_cmd, uninstall_cmd, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		lang.ID, lang.Name, lang.Category, lang.Description, lang.Icon, lang.VersionCmd, lang.InstallCmd, lang.UninstallCmd, now, now)
	return err
}

// UpdateLanguage 更新语言环境定义。
func (s *LanguageStore) UpdateLanguage(lang *LanguageRecord) error {
	now := time.Now().Unix()
	_, err := s.db.Raw().Exec(`
		UPDATE languages SET name=?, category=?, description=?, icon=?, version_cmd=?, install_cmd=?, uninstall_cmd=?, updated_at=?
		WHERE id=?`,
		lang.Name, lang.Category, lang.Description, lang.Icon, lang.VersionCmd, lang.InstallCmd, lang.UninstallCmd, now, lang.ID)
	return err
}

// DeleteLanguage 删除语言环境。
func (s *LanguageStore) DeleteLanguage(id string) error {
	_, err := s.db.Raw().Exec(`DELETE FROM languages WHERE id = ?`, id)
	return err
}

// --- Docker 镜像源管理 ---

// DockerSource 是 Docker 镜像源定义。
type DockerSource struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	URL       string `json:"url"`
	Enabled   bool   `json:"enabled"`
	IsDefault bool   `json:"is_default"`
	CreatedAt int64  `json:"created_at"`
	UpdatedAt int64  `json:"updated_at"`
}

// DockerSourceStore 管理 Docker 镜像源。
type DockerSourceStore struct {
	db *DB
}

// NewDockerSourceStore 创建镜像源存储实例。
func NewDockerSourceStore(db *DB) *DockerSourceStore {
	return &DockerSourceStore{db: db}
}

// InitDefaultSources 写入内置镜像源（幂等）。
func (s *DockerSourceStore) InitDefaultSources() error {
	now := time.Now().Unix()
	sources := []*DockerSource{
		{ID: "dockerhub", Name: "Docker Hub 官方", URL: "https://registry-1.docker.io", Enabled: true, IsDefault: true, CreatedAt: now, UpdatedAt: now},
		{ID: "aliyun", Name: "阿里云镜像加速器", URL: "https://cr.console.aliyun.com/cn-hangzhou/instances/mirrors", Enabled: true, IsDefault: false, CreatedAt: now, UpdatedAt: now},
		{ID: "tencent", Name: "腾讯云镜像加速器", URL: "https://mirror.ccs.tencentyun.com", Enabled: true, IsDefault: false, CreatedAt: now, UpdatedAt: now},
		{ID: "daocloud", Name: "DaoCloud 镜像加速器", URL: "https://docker.m.daocloud.io", Enabled: true, IsDefault: false, CreatedAt: now, UpdatedAt: now},
		{ID: "ustc", Name: "中科大镜像源", URL: "https://docker.mirrors.ustc.edu.cn", Enabled: true, IsDefault: false, CreatedAt: now, UpdatedAt: now},
		{ID: "tuna", Name: "清华 TUNA 镜像源", URL: "https://docker.mirror.tuna.tsinghua.edu.cn", Enabled: true, IsDefault: false, CreatedAt: now, UpdatedAt: now},
		{ID: "163", Name: "网易 163 镜像源", URL: "https://hub-mirror.c.163.com", Enabled: true, IsDefault: false, CreatedAt: now, UpdatedAt: now},
		{ID: "huawei", Name: "华为云镜像加速器", URL: "https://swr.cn-southwest-2.myhuaweicloud.com", Enabled: true, IsDefault: false, CreatedAt: now, UpdatedAt: now},
	}
	for _, src := range sources {
		enabled := 0
		if src.Enabled {
			enabled = 1
		}
		isDefault := 0
		if src.IsDefault {
			isDefault = 1
		}
		_, err := s.db.Raw().Exec(`
			INSERT OR IGNORE INTO docker_sources (id, name, url, enabled, is_default, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?)`,
			src.ID, src.Name, src.URL, enabled, isDefault, src.CreatedAt, src.UpdatedAt)
		if err != nil {
			return fmt.Errorf("insert docker source %s: %w", src.ID, err)
		}
	}
	return nil
}

// ListSources 返回所有镜像源。
func (s *DockerSourceStore) ListSources() []*DockerSource {
	rows, err := s.db.Raw().Query(`SELECT id, name, url, enabled, is_default, created_at, updated_at FROM docker_sources ORDER BY is_default DESC, name`)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var sources []*DockerSource
	for rows.Next() {
		var src DockerSource
		var enabled, isDefault int
		if err := rows.Scan(&src.ID, &src.Name, &src.URL, &enabled, &isDefault, &src.CreatedAt, &src.UpdatedAt); err != nil {
			continue
		}
		src.Enabled = enabled != 0
		src.IsDefault = isDefault != 0
		sources = append(sources, &src)
	}
	return sources
}

// AddSource 新增镜像源。
func (s *DockerSourceStore) AddSource(src *DockerSource) error {
	if src.ID == "" {
		return fmt.Errorf("source id is required")
	}
	now := time.Now().Unix()
	enabled := 0
	if src.Enabled {
		enabled = 1
	}
	_, err := s.db.Raw().Exec(`
		INSERT INTO docker_sources (id, name, url, enabled, is_default, created_at, updated_at)
		VALUES (?, ?, ?, ?, 0, ?, ?)`,
		src.ID, src.Name, src.URL, enabled, now, now)
	return err
}

// UpdateSource 更新镜像源。
func (s *DockerSourceStore) UpdateSource(src *DockerSource) error {
	now := time.Now().Unix()
	enabled := 0
	if src.Enabled {
		enabled = 1
	}
	_, err := s.db.Raw().Exec(`
		UPDATE docker_sources SET name=?, url=?, enabled=?, updated_at=?
		WHERE id=?`,
		src.Name, src.URL, enabled, now, src.ID)
	return err
}

// DeleteSource 删除镜像源。
func (s *DockerSourceStore) DeleteSource(id string) error {
	_, err := s.db.Raw().Exec(`DELETE FROM docker_sources WHERE id = ?`, id)
	return err
}

// SetDefaultSource 设置默认镜像源。
func (s *DockerSourceStore) SetDefaultSource(id string) error {
	_, _ = s.db.Raw().Exec(`UPDATE docker_sources SET is_default = 0`)
	_, err := s.db.Raw().Exec(`UPDATE docker_sources SET is_default = 1, enabled = 1 WHERE id = ?`, id)
	return err
}
