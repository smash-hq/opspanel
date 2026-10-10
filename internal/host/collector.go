package host

import (
	"fmt"

	"github.com/smash-hq/server-management/internal/logger"
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
// serverID 为空时默认采集本机。
func (c *Collector) Snapshot(serverID string) (*system.Info, error) {
	if serverID == "" || serverID == "local" {
		return c.local.Snapshot()
	}

	srv, ok := c.store.Get(serverID)
	if !ok {
		return nil, fmt.Errorf("server not found: %s", serverID)
	}

	switch srv.Type {
	case "remote":
		return c.remote.Snapshot(srv)
	default:
		// 未知类型回退到本机
		return c.local.Snapshot()
	}
}
