// Package software 提供软件包管理与安装能力。
package software

import (
	"context"
	"fmt"
	"strings"

	"github.com/smash-hq/opspanel/internal/logger"
	"github.com/smash-hq/opspanel/internal/remote"
	"github.com/smash-hq/opspanel/internal/storage"
)

// toRemoteServer 将 storage.Server 转换为 remote.Server。
func toRemoteServer(srv *storage.Server) *remote.Server {
	if srv == nil {
		return nil
	}
	return &remote.Server{
		ID:       srv.ID,
		Name:     srv.Name,
		Type:     srv.Type,
		Host:     srv.Host,
		Port:     srv.Port,
		Username: srv.Username,
		AuthType: srv.AuthType,
		Password: srv.Password,
		KeyPath:  srv.KeyPath,
	}
}

// Manager 管理远程服务器上的软件包。
type Manager struct {
	log    *logger.Logger
	remote *remote.RemoteCollector
}

// NewManager 创建软件管理器。
func NewManager(log *logger.Logger) *Manager {
	return &Manager{
		log:    log.Named("software"),
		remote: remote.NewRemoteCollector(log),
	}
}

// SoftwareStatus 表示单个软件的安装状态。
type SoftwareStatus struct {
	Package     *Package `json:"package"`
	Installed   bool     `json:"installed"`
	Version     string   `json:"version"`
	Status      string   `json:"status"`
	Installable bool     `json:"installable"`
}

// DetectPkgManager 检测远程服务器使用的包管理器。
func (m *Manager) DetectPkgManager(ctx context.Context, srv *storage.Server) (string, error) {
	client, err := m.remote.Acquire(toRemoteServer(srv))
	if err != nil {
		return "", err
	}
	defer m.remote.Release(srv.ID)

	for _, pm := range []string{"apt", "dnf", "yum"} {
		out := m.remote.RunOne(ctx, client, "which "+pm)
		if strings.TrimSpace(out) != "" {
			return pm, nil
		}
	}
	return "", fmt.Errorf("no supported package manager found (apt/dnf/yum)")
}

// CheckStatus 检查单个软件包的安装状态。
func (m *Manager) CheckStatus(ctx context.Context, srv *storage.Server, pkg *Package) *SoftwareStatus {
	status := &SoftwareStatus{
		Package: pkg,
		Status:  "unknown",
	}

	client, err := m.remote.Acquire(toRemoteServer(srv))
	if err != nil {
		return status
	}
	defer m.remote.Release(srv.ID)

	out := m.remote.RunOne(ctx, client, pkg.StatusCmd)
	if strings.TrimSpace(out) == "" {
		status.Installed = false
		pm, err := m.DetectPkgManager(ctx, srv)
		if err == nil {
			_, status.Installable = pkg.InstallCmd[pm]
		}
		return status
	}

	status.Installed = true

	if pkg.VersionCmd != "" {
		verOut := m.remote.RunOne(ctx, client, pkg.VersionCmd)
		status.Version = strings.TrimSpace(verOut)
	}

	if pkg.StartCmd != "" {
		activeOut := m.remote.RunOne(ctx, client, "systemctl is-active "+pkg.ID+" 2>/dev/null || true")
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
		status.Status = "none"
	}

	return status
}

// Install 安装软件包。version 为空则不替换版本占位符。
func (m *Manager) Install(ctx context.Context, srv *storage.Server, pkg *Package, version string) error {
	// 安装是长操作，套 30 分钟超时（调用方已有更紧的 deadline 时以调用方为准）
	ctx, cancel := context.WithTimeout(ctx, remote.LongOpTimeout)
	defer cancel()

	pm, err := m.DetectPkgManager(ctx, srv)
	if err != nil {
		return err
	}

	cmds, ok := pkg.InstallCmd[pm]
	if !ok {
		return fmt.Errorf("package %s not installable with %s", pkg.ID, pm)
	}

	client, err := m.remote.Acquire(toRemoteServer(srv))
	if err != nil {
		return err
	}
	defer m.remote.Release(srv.ID)

	for _, cmd := range cmds {
		if version != "" {
			cmd = strings.ReplaceAll(cmd, "{{VERSION}}", version)
		}
		out := m.remote.RunWithSudo(ctx, client, cmd)
		m.log.Infow("install cmd", "pkg", pkg.ID, "cmd", cmd, "version", version, "out", out)
	}

	return nil
}

// Uninstall 卸载软件包。
func (m *Manager) Uninstall(ctx context.Context, srv *storage.Server, pkg *Package) error {
	// 卸载是长操作，套 30 分钟超时
	ctx, cancel := context.WithTimeout(ctx, remote.LongOpTimeout)
	defer cancel()

	pm, err := m.DetectPkgManager(ctx, srv)
	if err != nil {
		return err
	}

	cmds, ok := pkg.UninstallCmd[pm]
	if !ok {
		return fmt.Errorf("package %s not uninstallable with %s", pkg.ID, pm)
	}

	client, err := m.remote.Acquire(toRemoteServer(srv))
	if err != nil {
		return err
	}
	defer m.remote.Release(srv.ID)

	for _, cmd := range cmds {
		out := m.remote.RunWithSudo(ctx, client, cmd)
		m.log.Infow("uninstall cmd", "pkg", pkg.ID, "cmd", cmd, "out", out)
	}

	return nil
}

// ServiceControl 控制服务启停。
func (m *Manager) ServiceControl(ctx context.Context, srv *storage.Server, pkg *Package, action string) error {
	client, err := m.remote.Acquire(toRemoteServer(srv))
	if err != nil {
		return err
	}
	defer m.remote.Release(srv.ID)

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

	out := m.remote.RunWithSudo(ctx, client, cmd)
	m.log.Infow("service control", "pkg", pkg.ID, "action", action, "out", out)
	return nil
}

// GetLog 获取软件运行日志。
func (m *Manager) GetLog(ctx context.Context, srv *storage.Server, service string, lines int) (string, error) {
	return m.remote.GetLog(ctx, toRemoteServer(srv), service, lines)
}
