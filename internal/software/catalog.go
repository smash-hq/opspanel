// Package software 提供软件包管理与安装能力。
package software

import (
	"context"

	"github.com/smash-hq/server-management/internal/storage"
)

// Package 是 storage.SoftwarePackage 的类型别名。
type Package = storage.SoftwarePackage

// Category 分类常量。
const (
	CategoryDatabase = "database"
	CategoryWeb      = "web"
	CategoryRuntime  = "runtime"
	CategoryTool     = "tool"
)

// Catalog 是内置的软件包目录。
var Catalog = []*Package{
	{
		ID:          "mysql",
		Name:        "MySQL",
		Category:    CategoryDatabase,
		Description: "最流行的开源关系型数据库",
		Icon:        "🐬",
		Version:     "8.0",
		Versions:    []string{"8.0", "8.4"},
		InstallCmd: map[string][]string{
			"apt": {"apt-get install -y mysql-server"},
			"yum": {"yum install -y mysql-server"},
			"dnf": {"dnf install -y mysql-server"},
		},
		UninstallCmd: map[string][]string{
			"apt": {"apt-get remove -y mysql-server"},
			"yum": {"yum remove -y mysql-server"},
			"dnf": {"dnf remove -y mysql-server"},
		},
		StatusCmd:  "which mysql",
		VersionCmd: "mysql --version 2>/dev/null",
		StartCmd:   "systemctl start mysql || systemctl start mysqld",
		StopCmd:    "systemctl stop mysql || systemctl stop mysqld",
		RestartCmd: "systemctl restart mysql || systemctl restart mysqld",
	},
	{
		ID:          "postgresql",
		Name:        "PostgreSQL",
		Category:    CategoryDatabase,
		Description: "强大的开源对象关系型数据库",
		Icon:        "🐘",
		Version:     "15",
		Versions:    []string{"14", "15", "16"},
		InstallCmd: map[string][]string{
			"apt": {"apt-get install -y postgresql postgresql-contrib"},
			"yum": {"yum install -y postgresql-server postgresql-contrib"},
			"dnf": {"dnf install -y postgresql-server postgresql-contrib"},
		},
		UninstallCmd: map[string][]string{
			"apt": {"apt-get remove -y postgresql*"},
			"yum": {"yum remove -y postgresql*"},
			"dnf": {"dnf remove -y postgresql*"},
		},
		StatusCmd:  "which psql",
		VersionCmd: "psql --version 2>/dev/null",
		StartCmd:   "systemctl start postgresql",
		StopCmd:    "systemctl stop postgresql",
		RestartCmd: "systemctl restart postgresql",
	},
	{
		ID:          "redis",
		Name:        "Redis",
		Category:    CategoryDatabase,
		Description: "高性能的内存键值数据库",
		Icon:        "🔴",
		Version:     "7.0",
		Versions:    []string{"7.0", "7.2", "7.4"},
		InstallCmd: map[string][]string{
			"apt": {"apt-get install -y redis-server"},
			"yum": {"yum install -y redis"},
			"dnf": {"dnf install -y redis"},
		},
		UninstallCmd: map[string][]string{
			"apt": {"apt-get remove -y redis-server"},
			"yum": {"yum remove -y redis"},
			"dnf": {"dnf remove -y redis"},
		},
		StatusCmd:  "which redis-server",
		VersionCmd: "redis-server --version 2>/dev/null",
		StartCmd:   "systemctl start redis || systemctl start redis-server",
		StopCmd:    "systemctl stop redis || systemctl stop redis-server",
		RestartCmd: "systemctl restart redis || systemctl restart redis-server",
	},
	{
		ID:          "nginx",
		Name:        "Nginx",
		Category:    CategoryWeb,
		Description: "高性能的 Web 服务器与反向代理",
		Icon:        "🌐",
		Version:     "1.24",
		Versions:    []string{"1.24", "1.26", "1.28"},
		InstallCmd: map[string][]string{
			"apt": {"apt-get install -y nginx"},
			"yum": {"yum install -y nginx"},
			"dnf": {"dnf install -y nginx"},
		},
		UninstallCmd: map[string][]string{
			"apt": {"apt-get remove -y nginx"},
			"yum": {"yum remove -y nginx"},
			"dnf": {"dnf remove -y nginx"},
		},
		StatusCmd:  "which nginx",
		VersionCmd: "nginx -v 2>&1",
		StartCmd:   "systemctl start nginx",
		StopCmd:    "systemctl stop nginx",
		RestartCmd: "systemctl restart nginx",
	},
	{
		ID:          "docker",
		Name:        "Docker",
		Category:    CategoryRuntime,
		Description: "容器化平台，支持应用容器化部署",
		Icon:        "🐳",
		Version:     "24.0",
		Versions:    []string{"24.0", "25.0", "26.0", "27.0"},
		InstallCmd: map[string][]string{
			"apt": {"apt-get install -y docker.io docker-compose"},
			"yum": {"yum install -y docker docker-compose"},
			"dnf": {"dnf install -y docker docker-compose"},
		},
		UninstallCmd: map[string][]string{
			"apt": {"apt-get remove -y docker.io docker-compose"},
			"yum": {"yum remove -y docker docker-compose"},
			"dnf": {"dnf remove -y docker docker-compose"},
		},
		StatusCmd:  "which docker",
		VersionCmd: "docker --version 2>/dev/null",
		StartCmd:   "systemctl start docker",
		StopCmd:    "systemctl stop docker",
		RestartCmd: "systemctl restart docker",
	},
	{
		ID:          "nodejs",
		Name:        "Node.js",
		Category:    CategoryRuntime,
		Description: "基于 Chrome V8 引擎的 JavaScript 运行时",
		Icon:        "⬢",
		Version:     "20 LTS",
		Versions:    []string{"18", "20", "22"},
		InstallCmd: map[string][]string{
			"apt": {"apt-get install -y nodejs npm"},
			"yum": {"yum install -y nodejs npm"},
			"dnf": {"dnf install -y nodejs npm"},
		},
		UninstallCmd: map[string][]string{
			"apt": {"apt-get remove -y nodejs npm"},
			"yum": {"yum remove -y nodejs npm"},
			"dnf": {"dnf remove -y nodejs npm"},
		},
		StatusCmd:  "which node",
		VersionCmd: "node --version 2>/dev/null",
		StartCmd:   "",
		StopCmd:    "",
		RestartCmd: "",
	},
}

// PackageIndex 用于按 ID 快速查找。
var PackageIndex = map[string]*Package{}

func init() {
	for _, p := range Catalog {
		PackageIndex[p.ID] = p
	}
}

// Get 按 ID 获取软件包定义。
func Get(id string) (*Package, bool) {
	p, ok := PackageIndex[id]
	return p, ok
}

// ListByCategory 返回指定分类的软件包。
func ListByCategory(category string) []*Package {
	var result []*Package
	for _, p := range Catalog {
		if p.Category == category {
			result = append(result, p)
		}
	}
	return result
}

// LoadFromStore 从数据库存储刷新内存索引。
func LoadFromStore(ctx context.Context, swStore interface {
	ListPackages(ctx context.Context) []*Package
}) {
	for _, pkg := range swStore.ListPackages(ctx) {
		PackageIndex[pkg.ID] = pkg
	}
}
