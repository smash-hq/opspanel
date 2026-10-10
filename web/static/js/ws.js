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

    const { onSnapshot, onOpen, onClose } = callbacks || {};

    function open() {
        if (!serverId || closed) return;
        const protocol = location.protocol === 'https:' ? 'wss' : 'ws';
        ws = new WebSocket(`${protocol}://${location.host}/ws?server=${encodeURIComponent(serverId)}`);

        ws.onopen = () => {
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
            if (!closed) reconnectTimer = setTimeout(open, 3000);
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
