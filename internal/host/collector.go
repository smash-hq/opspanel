package host

import (
	"context"
	"fmt"

	"github.com/smash-hq/opspanel/internal/logger"
	"github.com/smash-hq/opspanel/internal/remote"
	"github.com/smash-hq/opspanel/internal/system"
)

// Collector 根据服务器类型调度本地或远程采集。
type Collector struct {
	log    *logger.Logger
	local  *system.Collector
	remote *RemoteCollector
	store  *Store
}

// NewCollector 创建统一采集器。
func NewCollector(log *logger.Logger, store *Store) *Collector {
	return &Collector{
		log:    log.Named("collector"),
		local:  system.NewCollector(),
		remote: NewRemoteCollector(log),
		store:  store,
	}
}

// Snapshot 采集指定服务器的系统快照。
// serverID 为空或 srv_local 时采集本机。
func (c *Collector) Snapshot(ctx context.Context, serverID string) (*system.Info, error) {
	if serverID == "" || serverID == LocalServerID {
		return c.local.Snapshot(ctx)
	}

	srv, ok := c.store.Get(ctx, serverID)
	if !ok {
		return nil, fmt.Errorf("server not found: %s", serverID)
	}

	switch srv.Type {
	case "remote":
		return c.remote.Snapshot(ctx, toRemoteServer(srv))
	case "local":
		return c.local.Snapshot(ctx)
	default:
		// 未知类型不能静默用本机数据冒充目标主机，直接报错
		return nil, fmt.Errorf("未知的服务器类型 %q（server %s）", srv.Type, srv.ID)
	}
}

// toRemoteServer 将 storage.Server 转换为 remote.Server。
func toRemoteServer(srv *Server) *remote.Server {
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
