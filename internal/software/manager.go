// Package software 提供软件包管理与安装能力。
package software

import (
	"fmt"
	"strings"

	"github.com/smash-hq/server-management/internal/host"
	"github.com/smash-hq/server-management/internal/logger"
)

// Manager 管理远程服务器上的软件包。
type Manager struct {
	log    *logger.Logger
	remote *host.RemoteCollector
}

// NewManager 创建软件管理器。
func NewManager(log *logger.Logger) *Manager {
	return &Manager{
		log:    log.Named("software"),
		remote: host.NewRemoteCollector(log),
	}
}

// SoftwareStatus 表示单个软件的安装状态。
type SoftwareStatus struct {
	Package     *Package `json:"package"`      // 软件包定义
	Installed   bool     `json:"installed"`    // 是否已安装
	Version     string   `json:"version"`      // 已安装版本（未安装为空）
	Status      string   `json:"status"`       // running / stopped / unknown
	Installable bool     `json:"installable"`  // 当前包管理器是否支持安装
}

// DetectPkgManager 检测远程服务器使用的包管理器。
func (m *Manager) DetectPkgManager(srv *host.Server) (string, error) {
	client, err := m.remote.Dial(srv)
	if err != nil {
		return "", err
	}
	defer client.Close()

	// 依次检测 apt / dnf / yum
	for _, pm := range []string{"apt", "dnf", "yum"} {
		out := m.remote.RunOne(client, "which "+pm)
		if strings.TrimSpace(out) != "" {
			return pm, nil
		}
	}
	return "", fmt.Errorf("no supported package manager found (apt/dnf/yum)")
}

// CheckStatus 检查单个软件包的安装状态。
func (m *Manager) CheckStatus(srv *host.Server, pkg *Package) *SoftwareStatus {
	status := &SoftwareStatus{
		Package: pkg,
		Status:  "unknown",
	}

	client, err := m.remote.Dial(srv)
	if err != nil {
		return status
	}
	defer client.Close()

	// 检测是否安装
	out := m.remote.RunOne(client, pkg.StatusCmd)
	if strings.TrimSpace(out) == "" {
		status.Installed = false
		// 检查是否可安装
		pm, err := m.DetectPkgManager(srv)
		if err == nil {
			_, status.Installable = pkg.InstallCmd[pm]
		}
		return status
	}

	status.Installed = true

	// 获取版本
	if pkg.VersionCmd != "" {
		verOut := m.remote.RunOne(client, pkg.VersionCmd)
		status.Version = strings.TrimSpace(verOut)
	}

	// 检测服务状态（仅对有服务管理的软件）
	if pkg.StartCmd != "" {
		// 尝试 systemctl is-active
		activeOut := m.remote.RunOne(client, "systemctl is-active "+pkg.ID+" 2>/dev/null || true")
		activeOut = strings.TrimSpace(activeOut)
		switch activeOut {
		case "active":
			status.Status = "running"
		case "inactive", "failed":
			status.Status = "stopped"
		default:
			status.Status = "unknown"
		}
	} else {
		status.Status = "none" // 如 Node.js 没有服务
	}

	return status
}

// Install 安装软件包。
func (m *Manager) Install(srv *host.Server, pkg *Package) error {
	pm, err := m.DetectPkgManager(srv)
	if err != nil {
		return err
	}

	cmds, ok := pkg.InstallCmd[pm]
	if !ok {
		return fmt.Errorf("package %s not installable with %s", pkg.ID, pm)
	}

	client, err := m.remote.Dial(srv)
	if err != nil {
		return err
	}
	defer client.Close()

	for _, cmd := range cmds {
		out := m.remote.RunOne(client, cmd)
		m.log.Infow("install cmd", "pkg", pkg.ID, "cmd", cmd, "out", out)
	}

	return nil
}

// Uninstall 卸载软件包。
func (m *Manager) Uninstall(srv *host.Server, pkg *Package) error {
	pm, err := m.DetectPkgManager(srv)
	if err != nil {
		return err
	}

	cmds, ok := pkg.UninstallCmd[pm]
	if !ok {
		return fmt.Errorf("package %s not uninstallable with %s", pkg.ID, pm)
	}

	client, err := m.remote.Dial(srv)
	if err != nil {
		return err
	}
	defer client.Close()

	for _, cmd := range cmds {
		out := m.remote.RunOne(client, cmd)
		m.log.Infow("uninstall cmd", "pkg", pkg.ID, "cmd", cmd, "out", out)
	}

	return nil
}

// ServiceControl 控制服务启停。
func (m *Manager) ServiceControl(srv *host.Server, pkg *Package, action string) error {
	client, err := m.remote.Dial(srv)
	if err != nil {
		return err
	}
	defer client.Close()

	var cmd string
	switch action {
	case "start":
		cmd = pkg.StartCmd
	case "stop":
		cmd = pkg.StopCmd
	case "restart":
		cmd = pkg.RestartCmd
	default:
		return fmt.Errorf("unknown action: %s", action)
	}

	if cmd == "" {
		return fmt.Errorf("service action %s not supported for %s", action, pkg.ID)
	}

	out := m.remote.RunOne(client, cmd)
	m.log.Infow("service control", "pkg", pkg.ID, "action", action, "out", out)
	return nil
}
