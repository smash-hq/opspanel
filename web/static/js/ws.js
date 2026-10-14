/**
 * ws.js — WebSocket 连接管理器
 *
 * 管理与服务器的 WebSocket 长连接，支持自动重连和消息分发。
 * 推送数据格式：{ type: "snapshot", server_id: "xxx", data: { ... } }
 */

/**
 * 创建 WebSocket 连接。
 * 返回一个控制器对象，供外部管理连接生命周期。
 *
 * @param {string} serverId - 目标服务器 ID
 * @param {Object} callbacks - 回调函数集合
 * @param {Function} callbacks.onSnapshot - 收到 snapshot 消息时 (data, serverId) => void
 * @param {Function} [callbacks.onOpen] - 连接建立时
 * @param {Function} [callbacks.onClose] - 连接断开时
 * @returns {Object} 控制器 { disconnect: () => void, isConnected: () => boolean }
 */
export function connectWS(serverId, callbacks) {
    let ws = null;
    let reconnectTimer = null;
    let closed = false;
    let attempts = 0; // 连续失败次数，用于重连退避

    const { onSnapshot, onOpen, onClose } = callbacks || {};
    // 重连退避序列（毫秒）：1s → 2s → 5s → 10s 封顶，避免服务下线时无限高频重连
    const BACKOFF_MS = [1000, 2000, 5000, 10000];

    function open() {
        if (!serverId || closed) return;
        const protocol = location.protocol === 'https:' ? 'wss' : 'ws';
        ws = new WebSocket(`${protocol}://${location.host}/ws?server=${encodeURIComponent(serverId)}`);

        ws.onopen = () => {
            attempts = 0;
            if (typeof onOpen === 'function') onOpen();
        };

        ws.onmessage = (event) => {
            try {
                const message = JSON.parse(event.data);
                if (message.type === 'snapshot' && message.data) {
                    if (!message.server_id || message.server_id === serverId) {
                        onSnapshot && onSnapshot(message.data, message.server_id);
                    }
                }
            } catch (err) {
                console.warn('[WS] message parse failed', err);
            }
        };

        ws.onclose = () => {
            if (typeof onClose === 'function') onClose();
            if (!closed) {
                const delay = BACKOFF_MS[Math.min(attempts, BACKOFF_MS.length - 1)];
                attempts++;
                reconnectTimer = setTimeout(open, delay);
            }
        };

        ws.onerror = () => {
            console.warn('[WS] connection error, closing');
            ws.close();
        };
    }

    open();

    return {
        /** 断开连接并停止重连。 */
        disconnect() {
            closed = true;
            if (reconnectTimer) clearTimeout(reconnectTimer);
            if (ws) { ws.onclose = null; ws.close(); ws = null; }
        },
        /** 检查连接是否活跃。 */
        isConnected() {
            return ws !== null && ws.readyState === WebSocket.OPEN;
        },
    };
}
