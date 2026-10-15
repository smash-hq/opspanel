// SSH 连接池：连接按需建立、活跃保活、空闲超时优雅关闭、断线自动重建。
// 同一进程内所有 RemoteCollector 实例共享同一个池。
package remote

import (
	"sync"
	"time"

	"github.com/smash-hq/opspanel/internal/logger"
)

// 连接池默认参数。
const (
	defaultIdleTimeout    = 5 * time.Minute  // 空闲多久后优雅关闭
	defaultKeepaliveEvery = 30 * time.Second // 保活探测间隔
	defaultReapInterval   = 30 * time.Second // 空闲回收巡检间隔
)

// sharedPool 全局共享连接池（进程内唯一）。
var (
	sharedPoolOnce sync.Once
	sharedPool     *ConnPool
)

// getSharedPool 懒初始化共享连接池。
func getSharedPool(log *logger.Logger) *ConnPool {
	sharedPoolOnce.Do(func() {
		sharedPool = NewConnPool(log)
	})
	return sharedPool
}

// CloseAllSSH 优雅关闭全部池化 SSH 连接并停止后台巡检（服务关停时调用）。
func CloseAllSSH() {
	if sharedPool != nil {
		sharedPool.CloseAll()
	}
}

// connEntry 池中单个服务器持有的连接条目。
type connEntry struct {
	conn     *sshConn
	lastUsed time.Time
}

// ConnPool 按服务器 ID 复用 SSH 连接：
//   - Acquire 时池中无可用连接则重新建立（断线/被关闭的连接自动重连）
//   - 活跃连接由保活协程定期发送 keepalive 维持，失败即标记断开并移出池
//   - 空闲超过 idleTimeout 的连接由后台巡检优雅关闭
//   - CloseConn/CloseAll 供配置变更与服务关停时显式优雅关闭
type ConnPool struct {
	log         *logger.Logger
	mu          sync.Mutex
	entries     map[string]*connEntry
	idleTimeout time.Duration
	stopCh      chan struct{}
	stopOnce    sync.Once
}

// NewConnPool 创建连接池并启动后台巡检。
func NewConnPool(log *logger.Logger) *ConnPool {
	p := &ConnPool{
		log:         log.Named("sshpool"),
		entries:     make(map[string]*connEntry),
		idleTimeout: defaultIdleTimeout,
		stopCh:      make(chan struct{}),
	}
	go p.reaper()
	return p
}

// Acquire 获取指定服务器的可用连接；池中不存在或已断线时重新建立。
func (p *ConnPool) Acquire(srv *Server) (*sshConn, error) {
	p.mu.Lock()
	if e := p.entries[srv.ID]; e != nil && e.conn.usable() {
		e.lastUsed = time.Now()
		p.mu.Unlock()
		return e.conn, nil
	}
	delete(p.entries, srv.ID)
	p.mu.Unlock()

	// 不持锁拨号，避免阻塞其他服务器的获取
	conn, err := newSSHConn(srv)
	if err != nil {
		return nil, err
	}

	p.mu.Lock()
	// 并发拨号时后建立者覆盖前者并关闭旧连接，避免泄漏
	if old := p.entries[srv.ID]; old != nil {
		_ = old.conn.Close()
	}
	p.entries[srv.ID] = &connEntry{conn: conn, lastUsed: time.Now()}
	p.mu.Unlock()

	// 启动保活：连接断开时自动移出池
	conn.startKeepalive(defaultKeepaliveEvery, func() { p.evict(srv.ID, conn) })

	p.log.Infow("ssh connection established", "server", srv.ID, "host", srv.Host, "port", srv.Port)
	return conn, nil
}

// Release 归还连接：仅刷新使用时间，不关闭。
func (p *ConnPool) Release(serverID string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if e := p.entries[serverID]; e != nil {
		e.lastUsed = time.Now()
	}
}

// CloseConn 优雅关闭指定服务器的连接（更新/删除服务器配置时调用）。幂等。
func (p *ConnPool) CloseConn(serverID string) {
	p.mu.Lock()
	e := p.entries[serverID]
	delete(p.entries, serverID)
	p.mu.Unlock()

	if e != nil {
		_ = e.conn.Close()
		p.log.Infow("ssh connection closed", "server", serverID)
	}
}

// CloseAll 优雅关闭全部连接并停止后台巡检（服务关停时调用）。
func (p *ConnPool) CloseAll() {
	p.stopOnce.Do(func() { close(p.stopCh) })

	p.mu.Lock()
	entries := p.entries
	p.entries = make(map[string]*connEntry)
	p.mu.Unlock()

	for id, e := range entries {
		_ = e.conn.Close()
		p.log.Infow("ssh connection closed", "server", id)
	}
}

// evict 将指定连接移出池并关闭（仅当池中仍是该连接时生效）。
func (p *ConnPool) evict(serverID string, conn *sshConn) {
	p.mu.Lock()
	e := p.entries[serverID]
	if e != nil && e.conn == conn {
		delete(p.entries, serverID)
	} else {
		e = nil
	}
	p.mu.Unlock()

	if e != nil {
		_ = conn.Close()
		p.log.Infow("ssh connection evicted (dead)", "server", serverID)
	}
}

// reaper 后台巡检：优雅关闭空闲超时的连接。
func (p *ConnPool) reaper() {
	ticker := time.NewTicker(defaultReapInterval)
	defer ticker.Stop()

	for {
		select {
		case <-p.stopCh:
			return
		case <-ticker.C:
		}

		now := time.Now()
		type idle struct {
			id string
			e  *connEntry
		}
		var expired []idle
		p.mu.Lock()
		for id, e := range p.entries {
			if now.Sub(e.lastUsed) >= p.idleTimeout {
				expired = append(expired, idle{id, e})
				delete(p.entries, id)
			}
		}
		p.mu.Unlock()

		for _, v := range expired {
			_ = v.e.conn.Close()
			p.log.Infow("ssh connection closed (idle)", "server", v.id)
		}
	}
}

// startKeepalive 启动保活协程：定期发送 keepalive 请求维持连接与 NAT 映射，
// 失败则标记断开并回调通知池移除，下次需要时自动重建。
func (c *sshConn) startKeepalive(interval time.Duration, onDead func()) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-c.stopKeepalive:
				return
			case <-ticker.C:
			}

			_, _, err := c.client.SendRequest("keepalive@openssh.com", true, nil)
			if err != nil {
				c.broken.Store(true)
				onDead()
				return
			}
		}
	}()
}
