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
	ID          string `json:"id"`
	Name        string `json:"name"`
	Category    string `json:"category"`
	Description string `json:"description"`
	Icon        string `json:"icon"`
	VersionCmd  string `json:"version_cmd"`
	InstallCmd  string `json:"install_cmd"`
	CreatedAt   int64  `json:"created_at"`
	UpdatedAt   int64  `json:"updated_at"`
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
			// 主流编译型语言
			{ID: "go", Name: "Go", Category: "language", Description: "Google 开发的高性能编译语言", Icon: "🐹", VersionCmd: "go version 2>/dev/null", InstallCmd: "apt-get install -y golang-go || yum install -y golang || dnf install -y golang", CreatedAt: now, UpdatedAt: now},
			{ID: "rust", Name: "Rust", Category: "language", Description: "注重安全和性能的系统编程语言", Icon: "🦀", VersionCmd: "rustc --version 2>/dev/null", InstallCmd: "curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh -s -- -y", CreatedAt: now, UpdatedAt: now},
			{ID: "cpp", Name: "C/C++", Category: "language", Description: "经典的系统级编译语言", Icon: "⚙️", VersionCmd: "gcc --version 2>/dev/null | head -1", InstallCmd: "apt-get install -y gcc g++ build-essential || yum install -y gcc gcc-c++ || dnf install -y gcc gcc-c++", CreatedAt: now, UpdatedAt: now},
			{ID: "swift", Name: "Swift", Category: "language", Description: "Apple 开发的现代化编程语言", Icon: "🦅", VersionCmd: "swift --version 2>/dev/null | head -1", InstallCmd: "apt-get install -y clang || yum install -y clang", CreatedAt: now, UpdatedAt: now},
			{ID: "kotlin", Name: "Kotlin", Category: "language", Description: "JVM 平台现代编程语言", Icon: "🟣", VersionCmd: "kotlin -version 2>&1 | head -1", InstallCmd: "curl -s https://get.sdkman.io | bash", CreatedAt: now, UpdatedAt: now},
			// 主流解释型/VM 语言
			{ID: "java", Name: "Java", Category: "language", Description: "广泛使用的面向对象编程语言", Icon: "☕", VersionCmd: "java -version 2>&1 | head -1", InstallCmd: "apt-get install -y default-jdk || yum install -y java-11-openjdk-devel || dnf install -y java-11-openjdk-devel", CreatedAt: now, UpdatedAt: now},
			{ID: "python", Name: "Python", Category: "language", Description: "简洁优雅的解释型编程语言", Icon: "🐍", VersionCmd: "python3 --version 2>/dev/null", InstallCmd: "apt-get install -y python3 python3-pip python3-venv || yum install -y python3 python3-pip || dnf install -y python3 python3-pip", CreatedAt: now, UpdatedAt: now},
			{ID: "nodejs", Name: "Node.js", Category: "runtime", Description: "基于 Chrome V8 引擎的 JavaScript 运行时", Icon: "⬢", VersionCmd: "node --version 2>/dev/null", InstallCmd: "curl -fsSL https://deb.nodesource.com/setup_lts.x | sudo -E bash - && apt-get install -y nodejs || yum install -y nodejs || dnf install -y nodejs", CreatedAt: now, UpdatedAt: now},
			{ID: "php", Name: "PHP", Category: "language", Description: "广泛使用的服务端脚本语言", Icon: "🐘", VersionCmd: "php --version 2>/dev/null | head -1", InstallCmd: "apt-get install -y php-cli php-fpm || yum install -y php-cli php-fpm || dnf install -y php-cli php-fpm", CreatedAt: now, UpdatedAt: now},
			{ID: "ruby", Name: "Ruby", Category: "language", Description: "简洁优雅的动态编程语言", Icon: "💎", VersionCmd: "ruby --version 2>/dev/null", InstallCmd: "apt-get install -y ruby-full || yum install -y ruby || dnf install -y ruby", CreatedAt: now, UpdatedAt: now},
			{ID: "dotnet", Name: ".NET", Category: "runtime", Description: "微软跨平台开发框架（C#/F#/VB）", Icon: "🔷", VersionCmd: "dotnet --version 2>/dev/null", InstallCmd: "apt-get install -y dotnet-sdk-8.0 || yum install -y dotnet-sdk-8.0 || dnf install -y dotnet-sdk-8.0", CreatedAt: now, UpdatedAt: now},
			// 其他主流语言
			{ID: "scala", Name: "Scala", Category: "language", Description: "JVM 平台函数式与面向对象混合语言", Icon: "🔴", VersionCmd: "scala -version 2>&1 | head -1", InstallCmd: "curl -s https://get.sdkman.io | bash", CreatedAt: now, UpdatedAt: now},
			{ID: "perl", Name: "Perl", Category: "language", Description: "强大的文本处理脚本语言", Icon: "🐪", VersionCmd: "perl -v 2>/dev/null | head -2 | tail -1", InstallCmd: "apt-get install -y perl || yum install -y perl || dnf install -y perl", CreatedAt: now, UpdatedAt: now},
			{ID: "lua", Name: "Lua", Category: "language", Description: "轻量级嵌入式脚本语言", Icon: "🌙", VersionCmd: "lua -v 2>/dev/null || lua5.3 -v 2>/dev/null || lua5.4 -v 2>/dev/null", InstallCmd: "apt-get install -y lua5.4 || yum install -y lua || dnf install -y lua", CreatedAt: now, UpdatedAt: now},
			{ID: "typescript", Name: "TypeScript", Category: "language", Description: "JavaScript 的超集，添加了类型系统", Icon: "🔷", VersionCmd: "tsc --version 2>/dev/null", InstallCmd: "npm install -g typescript", CreatedAt: now, UpdatedAt: now},
			{ID: "elixir", Name: "Elixir", Category: "language", Description: "基于 Erlang VM 的函数式编程语言", Icon: "💧", VersionCmd: "elixir --version 2>/dev/null | head -1", InstallCmd: "apt-get install -y elixir || yum install -y elixir", CreatedAt: now, UpdatedAt: now},
			{ID: "haskell", Name: "Haskell", Category: "language", Description: "纯函数式编程语言", Icon: "λ", VersionCmd: "ghc --version 2>/dev/null", InstallCmd: "apt-get install -y ghc || yum install -y ghc || dnf install -y ghc", CreatedAt: now, UpdatedAt: now},
			{ID: "julia", Name: "Julia", Category: "language", Description: "高性能科学计算编程语言", Icon: "📊", VersionCmd: "julia --version 2>/dev/null", InstallCmd: "curl -fsSL https://install.julialang.org | sh", CreatedAt: now, UpdatedAt: now},
			{ID: "dart", Name: "Dart", Category: "language", Description: "Google 开发的客户端优化语言", Icon: "🎯", VersionCmd: "dart --version 2>&1 | head -1", InstallCmd: "apt-get install -y dart", CreatedAt: now, UpdatedAt: now},
			{ID: "zig", Name: "Zig", Category: "language", Description: "现代系统编程语言，C 的替代者", Icon: "⚡", VersionCmd: "zig version 2>/dev/null", InstallCmd: "apt-get install -y zig || yum install -y zig || dnf install -y zig", CreatedAt: now, UpdatedAt: now},
			{ID: "r", Name: "R", Category: "language", Description: "统计计算与数据可视化语言", Icon: "📈", VersionCmd: "R --version 2>/dev/null | head -1", InstallCmd: "apt-get install -y r-base || yum install -y R || dnf install -y R", CreatedAt: now, UpdatedAt: now},
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
	rows, err := s.db.Raw().Query(`SELECT id, name, category, description, icon, version_cmd, install_cmd, created_at, updated_at FROM languages ORDER BY category, name`)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var languages []*LanguageRecord
	for rows.Next() {
		var lang LanguageRecord
		if err := rows.Scan(&lang.ID, &lang.Name, &lang.Category, &lang.Description, &lang.Icon, &lang.VersionCmd, &lang.InstallCmd, &lang.CreatedAt, &lang.UpdatedAt); err != nil {
			continue
		}
		languages = append(languages, &lang)
	}
	return languages
}

// GetLanguage 按 ID 获取语言环境。
func (s *LanguageStore) GetLanguage(id string) (*LanguageRecord, bool) {
	var lang LanguageRecord
	err := s.db.Raw().QueryRow(`SELECT id, name, category, description, icon, version_cmd, install_cmd, created_at, updated_at FROM languages WHERE id = ?`, id).Scan(
		&lang.ID, &lang.Name, &lang.Category, &lang.Description, &lang.Icon, &lang.VersionCmd, &lang.InstallCmd, &lang.CreatedAt, &lang.UpdatedAt)
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
		INSERT INTO languages (id, name, category, description, icon, version_cmd, install_cmd, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		lang.ID, lang.Name, lang.Category, lang.Description, lang.Icon, lang.VersionCmd, lang.InstallCmd, now, now)
	return err
}

// UpdateLanguage 更新语言环境定义。
func (s *LanguageStore) UpdateLanguage(lang *LanguageRecord) error {
	now := time.Now().Unix()
	_, err := s.db.Raw().Exec(`
		UPDATE languages SET name=?, category=?, description=?, icon=?, version_cmd=?, install_cmd=?, updated_at=?
		WHERE id=?`,
		lang.Name, lang.Category, lang.Description, lang.Icon, lang.VersionCmd, lang.InstallCmd, now, lang.ID)
	return err
}

// DeleteLanguage 删除语言环境。
func (s *LanguageStore) DeleteLanguage(id string) error {
	_, err := s.db.Raw().Exec(`DELETE FROM languages WHERE id = ?`, id)
	return err
}
