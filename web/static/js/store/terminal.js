/**
 * terminal.js — Web 终端域
 *
 * 独立 SSH 会话（xterm + WebSocket），不占用监控 WS。
 */

const { ref, computed, watch, nextTick } = window.Vue;

export function setupTerminal(ctx) {
    const terminalMount = ref(null);          // xterm 挂载容器（模板 ref）
    const terminalStatus = ref('closed');     // connecting | connected | closed | error
    let term = null;                          // xterm 实例（非响应式，避免代理开销）
    let termFit = null;                       // FitAddon 实例
    let termWS = null;                        // 终端 WebSocket

    const terminalStatusText = computed(() => ({
        connecting: '连接中...',
        connected: '已连接',
        closed: '未连接',
        error: '连接失败',
    }[terminalStatus.value] || '未连接'));

    /** 点击终端 Tab：拦截本地节点，其余进入并初始化终端。 */
    function openTerminalTab() {
        if (!ctx.selectedServer.value) { ctx.toastNotify('请先选择服务器', true); return; }
        if (ctx.selectedServer.value === 'srv_local') { ctx.toastNotify('本地节点暂不支持终端', true); return; }
        ctx.activeTab.value = 'terminal';
        nextTick(() => initTerminal());
    }

    /** 初始化 xterm 并建立终端 WebSocket。 */
    function initTerminal() {
        if (!terminalMount.value || terminalStatus.value === 'connecting') return;
        destroyTerminal();
        terminalStatus.value = 'connecting';

        term = new window.Terminal({
            cursorBlink: true,
            fontSize: 14,
            fontFamily: '"SF Mono", Menlo, Monaco, Consolas, "Courier New", monospace',
            scrollback: 5000,
            theme: { background: '#0d1117', foreground: '#e6edf3', cursor: '#58a6ff', selectionBackground: 'rgba(56, 139, 253, 0.4)' },
        });
        termFit = new window.FitAddon.FitAddon();
        term.loadAddon(termFit);
        term.open(terminalMount.value);
        try { termFit.fit(); } catch (e) { /* 容器尚未布局时忽略 */ }

        const protocol = location.protocol === 'https:' ? 'wss' : 'ws';
        termWS = new WebSocket(`${protocol}://${location.host}/api/v1/terminal?server=${encodeURIComponent(ctx.selectedServer.value)}`);
        termWS.binaryType = 'arraybuffer';

        termWS.onopen = () => {
            terminalStatus.value = 'connected';
            term.focus();
            sendTerminalResize();
        };
        termWS.onmessage = (event) => {
            // 二进制帧 = SSH 原始输出
            if (event.data instanceof ArrayBuffer) {
                term && term.write(new Uint8Array(event.data));
                return;
            }
            // 文本帧 = 控制消息（exit）
            try {
                const msg = JSON.parse(event.data);
                if (msg.type === 'exit') {
                    term && term.write('\r\n\x1b[33m● ' + (msg.error ? '连接失败: ' + msg.error : '会话已结束') + '\x1b[0m\r\n');
                    terminalStatus.value = msg.error ? 'error' : 'closed';
                }
            } catch (e) { /* 忽略 */ }
        };
        termWS.onclose = () => {
            if (terminalStatus.value === 'connecting') {
                terminalStatus.value = 'error';
                term && term.write('\r\n\x1b[31m● 连接失败，请检查 SSH 配置\x1b[0m\r\n');
            } else if (terminalStatus.value === 'connected') {
                terminalStatus.value = 'closed';
                term && term.write('\r\n\x1b[31m● 连接已断开\x1b[0m\r\n');
            }
        };
        termWS.onerror = () => {
            if (terminalStatus.value === 'connecting') terminalStatus.value = 'error';
        };

        // 键盘输入 → SSH stdin
        term.onData(data => {
            if (termWS && termWS.readyState === WebSocket.OPEN) {
                termWS.send(JSON.stringify({ type: 'input', data }));
            }
        });
        // 容器尺寸变化 → 远端 PTY resize
        term.onResize(({ cols, rows }) => sendResizeMsg(cols, rows));
    }

    /** 按当前容器大小自适应并同步远端 PTY 尺寸。 */
    function sendTerminalResize() {
        if (!term || !termFit) return;
        try { termFit.fit(); } catch (e) { /* 忽略 */ }
        sendResizeMsg(term.cols, term.rows);
    }

    function sendResizeMsg(cols, rows) {
        if (termWS && termWS.readyState === WebSocket.OPEN) {
            termWS.send(JSON.stringify({ type: 'resize', cols, rows }));
        }
    }

    /** 重新连接终端。 */
    function reconnectTerminal() {
        nextTick(() => initTerminal());
    }

    /** 主动断开终端会话。 */
    function disconnectTerminal() {
        destroyTerminal();
        terminalStatus.value = 'closed';
    }

    /** 销毁终端资源（WS + xterm 实例）。 */
    function destroyTerminal() {
        if (termWS) {
            termWS.onclose = null;
            termWS.onmessage = null;
            try { termWS.close(); } catch (e) { /* 忽略 */ }
            termWS = null;
        }
        if (term) { term.dispose(); term = null; }
        termFit = null;
    }

    /** 窗口尺寸变化时重新自适应终端。 */
    function handleWindowResize() {
        if (ctx.activeTab.value === 'terminal' && terminalStatus.value === 'connected') sendTerminalResize();
    }

    // 切走终端 Tab 或切换服务器时断开会话，避免残留 SSH 连接
    watch(ctx.activeTab, tab => { if (tab !== 'terminal') disconnectTerminal(); });
    watch(ctx.selectedServer, () => disconnectTerminal());

    Object.assign(ctx, {
        terminalMount, terminalStatus, terminalStatusText,
        openTerminalTab, reconnectTerminal, disconnectTerminal, handleWindowResize,
    });
}
