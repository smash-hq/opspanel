package host

import (
	"fmt"

	"github.com/smash-hq/server-management/internal/logger"
	"github.com/smash-hq/server-management/internal/remote"
	"github.com/smash-hq/server-management/internal/system"
)

// Collector 根据服务器类型调度本地或远程采集。
type Collector struct {
	log      *logger.Logger
	local    *system.Collector
	remote   *RemoteCollector
	store    *Store
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
func (c *Collector) Snapshot(serverID string) (*system.Info, error) {
	if serverID == "" || serverID == LocalServerID {
		return c.local.Snapshot()
	}

	srv, ok := c.store.Get(serverID)
	if !ok {
		return nil, fmt.Errorf("server not found: %s", serverID)
	}

	switch srv.Type {
	case "remote":
		return c.remote.Snapshot(toRemoteServer(srv))
	default:
		return c.local.Snapshot()
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
