package api

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"github.com/smash-hq/opspanel/internal/ws"
)

// wsUpgrader 终端 WebSocket 升级器：与监控 WS 一致，校验 Origin 同源。
var wsUpgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
	CheckOrigin:     ws.CheckSameOrigin,
}

// terminalClientMsg 是前端发来的控制消息。
type terminalClientMsg struct {
	Type string `json:"type"` // "input" | "resize"
	Data string `json:"data"`
	Cols int    `json:"cols"`
	Rows int    `json:"rows"`
}

// handleTerminal: GET /api/v1/terminal?server=<id> 升级为 WebSocket，
// 双向桥接浏览器 xterm.js 与远端 SSH 交互式 shell。
// 协议：浏览器→服务端为 JSON 控制消息（input/resize）；服务端→浏览器为二进制原始输出，
// 会话结束时发送一条 JSON {"type":"exit"} 文本消息后关闭。
func (h *SoftwareHandler) handleTerminal(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		Fail(w, http.StatusMethodNotAllowed, 405, "method not allowed")
		return
	}
	serverID := r.URL.Query().Get("server")
	if serverID == "" {
		Fail(w, http.StatusBadRequest, 400, "server id required")
		return
	}
	srv, ok := h.store.Get(r.Context(), serverID)
	if !ok {
		Fail(w, http.StatusNotFound, 404, "server not found")
		return
	}
	if srv.Type == "local" {
		Fail(w, http.StatusBadRequest, 400, "本地节点暂不支持终端，请添加远程 SSH 节点")
		return
	}

	wsConn, err := wsUpgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer wsConn.Close()

	ts, err := h.remoteCollector.OpenTerminalSession(toRemoteServer(srv), 80, 24)
	if err != nil {
		h.log.Errorw("terminal open failed", "server", serverID, "error", err.Error())
		_ = wsConn.WriteMessage(websocket.TextMessage, mustJSON(map[string]string{"type": "exit", "error": err.Error()}))
		return
	}
	defer ts.Close()

	// 统一写锁：gorilla 不允许并发写
	var writeMu sync.Mutex
	writeMsg := func(mt int, data []byte) error {
		writeMu.Lock()
		defer writeMu.Unlock()
		return wsConn.WriteMessage(mt, data)
	}

	readDone := make(chan struct{})

	// 读泵：浏览器 → SSH stdin
	go func() {
		defer close(readDone)
		for {
			_, raw, err := wsConn.ReadMessage()
			if err != nil {
				return
			}
			var msg terminalClientMsg
			if err := json.Unmarshal(raw, &msg); err != nil {
				continue
			}
			switch msg.Type {
			case "input":
				if _, err := ts.Write([]byte(msg.Data)); err != nil {
					return
				}
			case "resize":
				if msg.Cols > 0 && msg.Rows > 0 {
					_ = ts.Resize(msg.Cols, msg.Rows)
				}
			}
		}
	}()

	// 保活：定期 ping，防止中间设备断开空闲连接
	pingStop := make(chan struct{})
	defer close(pingStop)
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				writeMu.Lock()
				_ = wsConn.WriteMessage(websocket.PingMessage, nil)
				writeMu.Unlock()
			case <-pingStop:
				return
			}
		}
	}()

	// 浏览器断开时（读泵退出）主动关闭 SSH 会话，唤醒阻塞的 ts.Read
	go func() {
		select {
		case <-readDone:
			ts.Close()
		case <-pingStop:
		}
	}()

	// 写泵：SSH stdout → 浏览器（二进制帧）
	buf := make([]byte, 8192)
	for {
		n, err := ts.Read(buf)
		if n > 0 {
			if werr := writeMsg(websocket.BinaryMessage, buf[:n]); werr != nil {
				break
			}
		}
		if err != nil {
			break
		}
	}

	// 会话结束：通知前端并等待读泵退出
	_ = writeMsg(websocket.TextMessage, mustJSON(map[string]string{"type": "exit"}))
	select {
	case <-readDone:
	case <-time.After(3 * time.Second):
	}
	_ = wsConn.Close()
	h.log.Infow("terminal session closed", "server", serverID)
}

// mustJSON 序列化为 JSON，失败时返回空对象字节（调用场景均为 map，不会失败）。
func mustJSON(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		return []byte("{}")
	}
	return b
}
