// Package ws 实现 WebSocket 连接管理与实时数据广播。
package ws

import (
	"encoding/json"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"github.com/smash-hq/server-management/internal/host"
	"github.com/smash-hq/server-management/internal/logger"
	"github.com/smash-hq/server-management/internal/system"
)

// websocket 协议升级器：拒绝与当前 Host 不同源的浏览器请求，防跨站 WebSocket 劫持
var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 4096,
	CheckOrigin:     CheckSameOrigin,
}

// checkSameOrigin 校验 Origin 与请求 Host 同源；非浏览器客户端（无 Origin 头）放行。
// 导出供终端等其他 WebSocket 升级器复用。
func CheckSameOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	u, err := url.Parse(origin)
	if err != nil {
		return false
	}
	return u.Host == r.Host
}

// client 代表一条 WebSocket 连接。
type client struct {
	hub      *Hub
	conn     *websocket.Conn
	send     chan []byte
	serverID string // 该客户端订阅的服务器 ID
}

// Hub 维护所有活跃连接并按服务器分发消息。
type Hub struct {
	log        *logger.Logger
	clients    map[*client]bool
	broadcast  chan *serverSnapshot
	register   chan *client
	unregister chan *client
	mu         sync.RWMutex
	done       chan struct{} // 关闭信号，Run 退出后 readPump 据此放弃注销
	closeOnce  sync.Once
}

// serverSnapshot 是带服务器标识的快照消息。
type serverSnapshot struct {
	serverID string
	info     *system.Info
}

// NewHub 创建 WebSocket 管理中心。
func NewHub(log *logger.Logger) *Hub {
	return &Hub{
		log:        log.Named("ws"),
		clients:    make(map[*client]bool),
		broadcast:  make(chan *serverSnapshot, 64),
		register:   make(chan *client),
		unregister: make(chan *client),
		done:       make(chan struct{}),
	}
}

// Run 启动 Hub 事件循环，直到 Close 被调用。
func (h *Hub) Run() {
	for {
		select {
		case <-h.done:
			return

		case c := <-h.register:
			h.mu.Lock()
			h.clients[c] = true
			h.mu.Unlock()
			h.log.Infow("ws client registered", "remote", c.conn.RemoteAddr().String(), "server", c.serverID)

		case c := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[c]; ok {
				delete(h.clients, c)
				close(c.send)
			}
			h.mu.Unlock()
			h.log.Infow("ws client unregistered", "remote", c.conn.RemoteAddr().String())

		case snap := <-h.broadcast:
			data, err := json.Marshal(map[string]any{
				"type":      "snapshot",
				"server_id": snap.serverID,
				"data":      snap.info,
			})
			if err != nil {
				h.log.Errorw("marshal snapshot failed", "error", err)
				continue
			}

			// 读锁下仅发送；缓冲已满的死连接收集后升级写锁统一清理，
			// 避免在 RLock 下修改 map 造成数据竞争
			var dead []*client
			h.mu.RLock()
			for c := range h.clients {
				if c.serverID != snap.serverID {
					continue
				}
				select {
				case c.send <- data:
				default:
					dead = append(dead, c)
				}
			}
			h.mu.RUnlock()

			if len(dead) > 0 {
				h.mu.Lock()
				for _, c := range dead {
					if h.clients[c] {
						delete(h.clients, c)
						close(c.send)
					}
				}
				h.mu.Unlock()
			}
		}
	}
}

// Close 优雅关闭 Hub：停止事件循环并断开全部客户端。
// 每个客户端的 send 关闭后，writePump 会先发送 WebSocket Close 帧再关闭连接，
// 浏览器端因此能感知正常关闭而非 TCP RST。
func (h *Hub) Close() {
	h.closeOnce.Do(func() { close(h.done) })

	h.mu.Lock()
	defer h.mu.Unlock()
	for c := range h.clients {
		delete(h.clients, c)
		close(c.send)
	}
}

// Broadcast 向订阅了指定服务器的所有客户端推送快照。
// Hub 已关闭时静默丢弃（避免阻塞推送循环）。
func (h *Hub) Broadcast(info *system.Info, serverID string) {
	select {
	case h.broadcast <- &serverSnapshot{serverID: serverID, info: info}:
	case <-h.done:
	}
}

// ClientCount 返回当前连接数。
func (h *Hub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

// ServeWS 将 HTTP 请求升级为 WebSocket 并启动读写协程。
// 客户端可通过 ?server=<id> 指定订阅的服务器，默认 "local"。
func (h *Hub) ServeWS(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.log.Errorw("ws upgrade failed", "error", err)
		return
	}

	// 从查询参数获取订阅的服务器 ID，默认本机
	serverID := r.URL.Query().Get("server")
	if serverID == "" {
		serverID = host.LocalServerID
	}

	c := &client{
		hub:      h,
		conn:     conn,
		send:     make(chan []byte, 16),
		serverID: serverID,
	}
	// Hub 已关闭（服务正在停止）时放弃注册并断开，避免 goroutine 永久阻塞
	select {
	case h.register <- c:
	case <-h.done:
		conn.Close()
		return
	}

	go c.writePump()
	go c.readPump()
}

// writePump 将 send channel 中的数据写入 WebSocket。
func (c *client) writePump() {
	ticker := time.NewTicker(30 * time.Second) // 心跳
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case msg, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// readPump 读取消耗客户端消息（保持连接活跃，检测断连）。
// 支持客户端通过 JSON 消息切换订阅服务器：{"action": "switch", "server": "<id>"}
func (c *client) readPump() {
	defer func() {
		// Hub 已关闭时事件循环不再消费 unregister，放弃注入避免 goroutine 泄漏
		select {
		case c.hub.unregister <- c:
		case <-c.hub.done:
		}
		c.conn.Close()
	}()

	c.conn.SetReadLimit(512)
	c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		_, data, err := c.conn.ReadMessage()
		if err != nil {
			break
		}

		// 尝试解析为切换订阅命令
		var msg struct {
			Action string `json:"action"`
			Server string `json:"server"`
		}
		if json.Unmarshal(data, &msg) == nil && msg.Action == "switch" {
			c.hub.mu.Lock()
			c.serverID = msg.Server
			if c.serverID == "" {
				c.serverID = host.LocalServerID
			}
			c.hub.mu.Unlock()
			c.hub.log.Infow("ws client switched server", "server", msg.Server)
		}
	}
}
