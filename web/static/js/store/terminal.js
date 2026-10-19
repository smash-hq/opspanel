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

    // ─── 常用命令面板（分类菜单 + 搜索 + 点击注入终端）───
    const commandGroups = ref([
        {
            id: 'status', name: '系统状态',
            commands: [
                { c: 'top', d: '实时进程与负载' },
                { c: 'htop', d: '增强版进程监控' },
                { c: 'uptime', d: '运行时长与负载' },
                { c: 'free -h', d: '内存使用' },
                { c: 'vmstat 1 5', d: '虚拟内存与 CPU 概览' },
                { c: 'uname -a', d: '内核版本' },
                { c: 'cat /etc/os-release', d: '系统发行版' },
                { c: 'w', d: '当前登录用户' },
                { c: 'last | head -20', d: '最近登录记录' },
                { c: 'date', d: '系统时间' },
            ],
        },
        {
            id: 'disk', name: '磁盘与存储',
            commands: [
                { c: 'lsblk', d: '磁盘与分区' },
                { c: 'df -h', d: '磁盘空间使用' },
                { c: 'df -i', d: 'inode 使用情况' },
                { c: 'fdisk -l', d: '磁盘分区表' },
                { c: 'blkid', d: '分区 UUID 与文件系统' },
                { c: 'mount | column -t', d: '已挂载文件系统' },
                { c: 'mount <dev> <dir>', d: '挂载设备' },
                { c: 'umount <dir>', d: '卸载挂载点' },
                { c: 'du -sh <dir>', d: '目录总占用' },
            ],
        },
        {
            id: 'network', name: '网络',
            commands: [
                { c: 'ip a', d: '网卡与 IP' },
                { c: 'ip r', d: '路由表' },
                { c: 'ss -tlnp', d: '监听端口' },
                { c: 'ss -s', d: '连接统计摘要' },
                { c: 'netstat -tlnp', d: '监听端口(经典)' },
                { c: 'ping -c 4 <host>', d: '连通性测试' },
                { c: 'traceroute <host>', d: '路由跟踪' },
                { c: 'nslookup <domain>', d: '域名解析' },
                { c: 'curl -I <url>', d: 'HTTP 探测' },
                { c: 'wget <url>', d: '下载文件' },
                { c: 'arp -a', d: 'ARP 表' },
                { c: 'tcpdump -i any -nn port <port>', d: '按端口抓包' },
            ],
        },
        {
            id: 'service', name: 'systemd 服务',
            commands: [
                { c: 'systemctl status <name>', d: '服务状态' },
                { c: 'systemctl start <name>', d: '启动服务' },
                { c: 'systemctl restart <name>', d: '重启服务' },
                { c: 'systemctl stop <name>', d: '停止服务' },
                { c: 'systemctl cat <name>', d: '查看 unit 文件' },
                { c: 'systemctl --failed', d: '失败的服务' },
                { c: 'systemctl list-unit-files --type=service', d: '全部服务单元' },
                { c: 'systemctl list-timers', d: '定时器列表' },
                { c: 'journalctl -u <name> -n 100', d: '服务最近日志' },
                { c: 'journalctl -f -u <name>', d: '服务实时日志' },
                { c: 'journalctl -p err -n 50', d: '错误级别日志' },
            ],
        },
        {
            id: 'process', name: '进程',
            commands: [
                { c: 'ps aux | grep <kw>', d: '按名查进程' },
                { c: 'ps -ef --sort=-%mem | head -15', d: '内存占用 Top15' },
                { c: 'ps -ef --sort=-%cpu | head -15', d: 'CPU 占用 Top15' },
                { c: 'top -b -n 1 | head -30', d: '进程快照' },
                { c: 'lsof -i:<port>', d: '端口占用进程' },
                { c: 'kill -15 <pid>', d: '优雅结束进程' },
                { c: 'kill -9 <pid>', d: '强制结束进程' },
                { c: 'pkill -f <kw>', d: '按名结束进程' },
                { c: 'strace -cp <pid>', d: '跟踪进程系统调用' },
            ],
        },
        {
            id: 'file', name: '文件与目录',
            commands: [
                { c: 'ls -lah', d: '目录详情' },
                { c: 'pwd', d: '当前路径' },
                { c: 'stat <file>', d: '文件元信息' },
                { c: 'head -20 <file>', d: '文件开头 20 行' },
                { c: 'tail -100 <file>', d: '文件末尾 100 行' },
                { c: 'tail -f <file>', d: '实时追踪文件' },
                { c: 'wc -l <file>', d: '统计行数' },
                { c: 'find <dir> -name "<kw>"', d: '按名查找文件' },
                { c: 'find <dir> -size +100M', d: '查找大文件(>100M)' },
                { c: 'find <dir> -mtime -1', d: '最近 1 天修改的文件' },
                { c: 'grep -rn "<kw>" <dir>', d: '递归搜索文件内容' },
                { c: 'md5sum <file>', d: '计算 MD5 校验' },
            ],
        },
        {
            id: 'archive', name: '压缩与传输',
            commands: [
                { c: 'tar -czf <name>.tar.gz <dir>', d: 'gzip 打包目录' },
                { c: 'tar -xzf <file>.tar.gz', d: '解压 tar.gz' },
                { c: 'tar -tf <file>.tar.gz', d: '查看包内容' },
                { c: 'zip -r <name>.zip <dir>', d: 'zip 打包目录' },
                { c: 'unzip <file>.zip -d <dir>', d: '解压 zip' },
                { c: 'rsync -avz <src> <dst>', d: '增量同步目录' },
                { c: 'scp <file> <user>@<host>:<dir>', d: '远程拷贝文件' },
            ],
        },
        {
            id: 'perm', name: '用户与权限',
            commands: [
                { c: 'whoami', d: '当前用户' },
                { c: 'id <user>', d: '用户 UID 与组' },
                { c: 'sudo -l', d: 'sudo 权限列表' },
                { c: 'useradd <name>', d: '创建用户' },
                { c: 'userdel <name>', d: '删除用户' },
                { c: 'passwd <user>', d: '修改密码' },
                { c: 'su - <user>', d: '切换用户' },
                { c: 'chmod +x <file>', d: '添加执行权限' },
                { c: 'chmod 755 <file>', d: '设置 755 权限' },
                { c: 'chown <user>:<group> <file>', d: '修改属主属组' },
            ],
        },
        {
            id: 'hardware', name: '硬件与 GPU',
            commands: [
                { c: 'lscpu', d: 'CPU 信息' },
                { c: 'lsusb', d: 'USB 设备' },
                { c: 'lspci | grep -iE "vga|3d"', d: '显卡设备' },
                { c: 'nvidia-smi', d: 'GPU 状态' },
                { c: 'watch -n 1 nvidia-smi', d: 'GPU 状态持续刷新' },
                { c: 'nvidia-smi --query-gpu=name,memory.used,memory.total --format=csv', d: 'GPU 显存 CSV' },
                { c: 'dmesg | tail -50', d: '内核最近消息' },
                { c: 'sensors', d: '温度传感器' },
                { c: 'dmidecode -t memory | grep -E "Size|Speed"', d: '内存条规格' },
            ],
        },
        {
            id: 'pkg', name: '软件包',
            commands: [
                { c: 'yum install <pkg>', d: 'yum 安装' },
                { c: 'yum remove <pkg>', d: 'yum 卸载' },
                { c: 'yum info <pkg>', d: '查看包信息' },
                { c: 'yum list installed | grep <kw>', d: '已装包搜索' },
                { c: 'rpm -qa | grep <kw>', d: 'rpm 包搜索' },
                { c: 'apt update', d: 'apt 更新索引' },
                { c: 'apt install <pkg>', d: 'apt 安装' },
                { c: 'apt list --installed | grep <kw>', d: '已装包搜索(apt)' },
                { c: 'dpkg -l | grep <kw>', d: 'deb 包搜索' },
            ],
        },
    ]);
    const cmdSearch = ref('');
    const cmdGroupOpen = ref({});          // 分组折叠状态，默认全部收起

    // 按关键字过滤（匹配命令或说明）；搜索时忽略折叠状态全部展开
    const visibleCmdGroups = computed(() => {
        const kw = cmdSearch.value.trim().toLowerCase();
        return commandGroups.value
            .map(g => {
                const commands = kw
                    ? g.commands.filter(x => x.c.toLowerCase().includes(kw) || x.d.toLowerCase().includes(kw))
                    : g.commands;
                const open = kw ? true : cmdGroupOpen.value[g.id] === true;
                return { ...g, commands, open };
            })
            .filter(g => g.commands.length);
    });

    function toggleCmdGroup(id) {
        cmdGroupOpen.value[id] = cmdGroupOpen.value[id] !== true;
    }

    /** 将命令注入终端：含 <占位符> 的只输入不回车，等用户补全；否则直接执行。 */
    function sendCommand(cmd) {
        if (terminalStatus.value !== 'connected' || !termWS || termWS.readyState !== WebSocket.OPEN) {
            ctx.toastNotify('终端未连接', true);
            return;
        }
        const needsArgs = /<[^>]+>/.test(cmd);
        termWS.send(JSON.stringify({ type: 'input', data: cmd + (needsArgs ? '' : '\r') }));
        term && term.focus();
    }

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
        cmdSearch, visibleCmdGroups, toggleCmdGroup, sendCommand,
        openTerminalTab, reconnectTerminal, disconnectTerminal, handleWindowResize,
    });
}
