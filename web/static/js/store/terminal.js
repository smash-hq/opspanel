/**
 * terminal.js — Web 终端域
 *
 * 独立 SSH 会话（xterm + WebSocket），不占用监控 WS。
 */
import { t } from '../i18n/index.js';


const { ref, computed, watch, nextTick } = window.Vue;

// 终端状态文案映射（存中文源键，读取时经 t() 渲染期翻译；服务器终端与容器终端共用）
const TERMINAL_STATUS_TEXT = {
    connecting: '连接中...',
    connected: '已连接',
    closed: '未连接',
    error: '连接失败',
};

// xterm 外观配置（两处终端实例共用）
const XTERM_OPTIONS = {
    cursorBlink: true,
    fontSize: 14,
    fontFamily: '"SF Mono", Menlo, Monaco, Consolas, "Courier New", monospace',
    scrollback: 5000,
    theme: { background: '#0d1117', foreground: '#e6edf3', cursor: '#58a6ff', selectionBackground: 'rgba(56, 139, 253, 0.4)' },
};

export function setupTerminal(ctx) {
    const terminalMount = ref(null);          // xterm 挂载容器（模板 ref）
    const terminalStatus = ref('closed');     // connecting | connected | closed | error
    let term = null;                          // xterm 实例（非响应式，避免代理开销）
    let termFit = null;                       // FitAddon 实例
    let termWS = null;                        // 终端 WebSocket

    const terminalStatusText = computed(() => t(TERMINAL_STATUS_TEXT[terminalStatus.value] || '未连接'));

    // 常用命令面板（分类菜单 + 搜索 + 点击注入终端）；computed 保证切换语言后重新翻译
    const commandGroups = computed(() => [
        {
            id: 'status', name: t('系统状态'),
            commands: [
                { c: 'top', d: t('实时进程与负载') },
                { c: 'htop', d: t('增强版进程监控') },
                { c: 'uptime', d: t('运行时长与负载') },
                { c: 'free -h', d: t('内存使用') },
                { c: 'vmstat 1 5', d: t('虚拟内存与 CPU 概览') },
                { c: 'uname -a', d: t('内核版本') },
                { c: 'cat /etc/os-release', d: t('系统发行版') },
                { c: 'w', d: t('当前登录用户') },
                { c: 'last | head -20', d: t('最近登录记录') },
            ],
        },
        {
            id: 'time', name: t('时间与时区'),
            commands: [
                { c: 'date', d: t('系统时间') },
                { c: "date '+%Y-%m-%d %H:%M:%S %Z'", d: t('格式化时间与时区') },
                { c: 'date +%s', d: t('Unix 时间戳') },
                { c: 'date -d @<timestamp>', d: t('时间戳转日期') },
                { c: 'timedatectl', d: t('时间/时区/NTP 同步状态') },
                { c: 'timedatectl list-timezones', d: t('可用时区列表') },
                { c: 'timedatectl set-timezone Asia/Shanghai', d: t('设置时区为上海') },
                { c: 'hwclock --show', d: t('硬件时钟 (RTC)') },
                { c: 'chronyc sources -v', d: t('NTP 同步源 (chrony)') },
                { c: 'ntpq -p', d: t('NTP 对端状态 (经典)') },
                { c: 'cal', d: t('本月日历') },
            ],
        },
        {
            id: 'cpu', name: 'CPU',
            commands: [
                { c: 'lscpu', d: t('CPU 架构与核心信息') },
                { c: 'grep "model name" /proc/cpuinfo | head -1', d: t('CPU 型号') },
                { c: 'lscpu -e', d: t('CPU 拓扑明细 (核心/插槽/NUMA)') },
                { c: 'grep MHz /proc/cpuinfo', d: t('各核心当前频率') },
                { c: 'mpstat -P ALL 1 3', d: t('各核心利用率 (sysstat)') },
                { c: 'pidstat -u 1 3', d: t('进程级 CPU 占用统计') },
                { c: 'numactl --hardware', d: t('NUMA 拓扑') },
                { c: 'taskset -pc <pid>', d: t('进程 CPU 亲和性') },
                { c: 'sensors', d: t('CPU 温度传感器') },
            ],
        },
        {
            id: 'hardware', name: t('硬件信息'),
            commands: [
                { c: 'lsusb', d: t('USB 设备') },
                { c: 'lspci | grep -iE "vga|3d|npu|co-processor"', d: t('显卡/NPU 设备') },
                { c: 'lspci -tv', d: t('PCI 拓扑树') },
                { c: 'dmidecode -t memory | grep -E "Size|Speed"', d: t('内存条规格') },
                { c: 'dmidecode -t bios', d: t('BIOS 信息') },
                { c: 'dmesg | tail -50', d: t('内核最近消息') },
            ],
        },
        {
            id: 'disk', name: t('磁盘与存储'),
            commands: [
                { c: 'lsblk', d: t('磁盘与分区') },
                { c: 'df -h', d: t('磁盘空间使用') },
                { c: 'df -i', d: t('inode 使用情况') },
                { c: 'fdisk -l', d: t('磁盘分区表') },
                { c: 'blkid', d: t('分区 UUID 与文件系统') },
                { c: 'mount | column -t', d: t('已挂载文件系统') },
                { c: 'mount <dev> <dir>', d: t('挂载设备') },
                { c: 'umount <dir>', d: t('卸载挂载点') },
                { c: 'du -sh <dir>', d: t('目录总占用') },
            ],
        },
        {
            id: 'network', name: t('网络'),
            commands: [
                { c: 'ip a', d: t('网卡与 IP') },
                { c: 'ip r', d: t('路由表') },
                { c: 'ss -tlnp', d: t('监听端口') },
                { c: 'ss -s', d: t('连接统计摘要') },
                { c: 'netstat -tlnp', d: t('监听端口(经典)') },
                { c: 'ping -c 4 <host>', d: t('连通性测试') },
                { c: 'traceroute <host>', d: t('路由跟踪') },
                { c: 'nslookup <domain>', d: t('域名解析') },
                { c: 'curl -I <url>', d: t('HTTP 探测') },
                { c: 'wget <url>', d: t('下载文件') },
                { c: 'arp -a', d: t('ARP 表') },
                { c: 'tcpdump -i any -nn port <port>', d: t('按端口抓包') },
            ],
        },
        {
            id: 'service', name: t('systemd 服务'),
            commands: [
                { c: 'systemctl status <name>', d: t('服务状态') },
                { c: 'systemctl start <name>', d: t('启动服务') },
                { c: 'systemctl restart <name>', d: t('重启服务') },
                { c: 'systemctl stop <name>', d: t('停止服务') },
                { c: 'systemctl cat <name>', d: t('查看 unit 文件') },
                { c: 'systemctl --failed', d: t('失败的服务') },
                { c: 'systemctl list-unit-files --type=service', d: t('全部服务单元') },
                { c: 'systemctl list-timers', d: t('定时器列表') },
                { c: 'journalctl -u <name> -n 100', d: t('服务最近日志') },
                { c: 'journalctl -f -u <name>', d: t('服务实时日志') },
                { c: 'journalctl -p err -n 50', d: t('错误级别日志') },
            ],
        },
        {
            id: 'process', name: t('进程'),
            commands: [
                { c: 'ps aux | grep <kw>', d: t('按名查进程') },
                { c: 'ps -ef --sort=-%mem | head -15', d: t('内存占用 Top15') },
                { c: 'ps -ef --sort=-%cpu | head -15', d: t('CPU 占用 Top15') },
                { c: 'top -b -n 1 | head -30', d: t('进程快照') },
                { c: 'lsof -i:<port>', d: t('端口占用进程') },
                { c: 'kill -15 <pid>', d: t('优雅结束进程') },
                { c: 'kill -9 <pid>', d: t('强制结束进程') },
                { c: 'pkill -f <kw>', d: t('按名结束进程') },
                { c: 'strace -cp <pid>', d: t('跟踪进程系统调用') },
            ],
        },
        {
            id: 'file', name: t('文件与目录'),
            commands: [
                { c: 'ls -lah', d: t('目录详情') },
                { c: 'pwd', d: t('当前路径') },
                { c: 'stat <file>', d: t('文件元信息') },
                { c: 'head -20 <file>', d: t('文件开头 20 行') },
                { c: 'tail -100 <file>', d: t('文件末尾 100 行') },
                { c: 'tail -f <file>', d: t('实时追踪文件') },
                { c: 'wc -l <file>', d: t('统计行数') },
                { c: 'find <dir> -name "<kw>"', d: t('按名查找文件') },
                { c: 'find <dir> -size +100M', d: t('查找大文件(>100M)') },
                { c: 'find <dir> -mtime -1', d: t('最近 1 天修改的文件') },
                { c: 'grep -rn "<kw>" <dir>', d: t('递归搜索文件内容') },
                { c: 'md5sum <file>', d: t('计算 MD5 校验') },
            ],
        },
        {
            id: 'archive', name: t('压缩与传输'),
            commands: [
                { c: 'tar -czf <name>.tar.gz <dir>', d: t('gzip 打包目录') },
                { c: 'tar -xzf <file>.tar.gz', d: t('解压 tar.gz') },
                { c: 'tar -tf <file>.tar.gz', d: t('查看包内容') },
                { c: 'zip -r <name>.zip <dir>', d: t('zip 打包目录') },
                { c: 'unzip <file>.zip -d <dir>', d: t('解压 zip') },
                { c: 'rsync -avz <src> <dst>', d: t('增量同步目录') },
                { c: 'scp <file> <user>@<host>:<dir>', d: t('远程拷贝文件') },
            ],
        },
        {
            id: 'perm', name: t('用户与权限'),
            commands: [
                { c: 'whoami', d: t('当前用户') },
                { c: 'id <user>', d: t('用户 UID 与组') },
                { c: 'sudo -l', d: t('sudo 权限列表') },
                { c: 'useradd <name>', d: t('创建用户') },
                { c: 'userdel <name>', d: t('删除用户') },
                { c: 'passwd <user>', d: t('修改密码') },
                { c: 'su - <user>', d: t('切换用户') },
                { c: 'chmod +x <file>', d: t('添加执行权限') },
                { c: 'chmod 755 <file>', d: t('设置 755 权限') },
                { c: 'chown <user>:<group> <file>', d: t('修改属主属组') },
            ],
        },
        {
            id: 'pkg', name: t('软件包'),
            commands: [
                { c: 'yum install <pkg>', d: t('yum 安装') },
                { c: 'yum remove <pkg>', d: t('yum 卸载') },
                { c: 'yum info <pkg>', d: t('查看包信息') },
                { c: 'yum list installed | grep <kw>', d: t('已装包搜索') },
                { c: 'rpm -qa | grep <kw>', d: t('rpm 包搜索') },
                { c: 'apt update', d: t('apt 更新索引') },
                { c: 'apt install <pkg>', d: t('apt 安装') },
                { c: 'apt list --installed | grep <kw>', d: t('已装包搜索(apt)') },
                { c: 'dpkg -l | grep <kw>', d: t('deb 包搜索') },
            ],
        },
        {
            id: 'gpu', name: 'GPU', subgroups: [
                {
                    id: 'gpu-probe', name: t('通用探测'),
                    commands: [
                        { c: 'for c in nvidia-smi npu-smi cnmon rocm-smi mthreads-gmi ixsmi mx-smi amd-smi xpu-smi; do command -v $c >/dev/null && echo "$c 已安装"; done', d: t('探测已装 GPU/NPU 管理工具') },
                        { c: 'lspci | grep -iE "nvidia|vga|3d|display|accel|co-processor"', d: t('PCI 显卡/加速设备') },
                        { c: 'ls /dev | grep -iE "nvidia|davinci|mlu|musa|maca"', d: t('GPU 设备节点') },
                        { c: 'fuser -v /dev/nvidia*', d: t('查看 GPU 占用进程') },
                    ],
                },
                {
                    id: 'gpu-nvidia', name: 'NVIDIA',
                    commands: [
                        { c: 'nvidia-smi', d: t('GPU 总览(利用率/显存/温度)') },
                        { c: 'nvidia-smi -l 1', d: t('每秒刷新实时监控') },
                        { c: 'nvidia-smi -q -d TEMPERATURE', d: t('温度详情') },
                        { c: 'nvidia-smi -q -d POWER', d: t('功耗详情') },
                        { c: 'nvidia-smi --query-gpu=index,name,temperature.gpu,utilization.gpu,memory.used,memory.total --format=csv', d: t('自定义字段汇总') },
                    ],
                },
                {
                    id: 'gpu-hygon', name: t('海光 DCU'),
                    commands: [
                        { c: 'rocm-smi', d: t('DCU 总览') },
                        { c: 'hy-smi', d: t('海光自研工具总览') },
                        { c: 'rocm-smi --showtemp', d: t('DCU 温度') },
                        { c: 'rocm-smi --showuse --showmemuse', d: t('利用率与显存占用') },
                    ],
                },
                {
                    id: 'gpu-moore', name: t('摩尔线程'),
                    commands: [
                        { c: 'mthreads-gmi', d: t('GPU 总览') },
                        { c: 'mthreads-gmi -q', d: t('详细信息(含温度功耗)') },
                        { c: 'watch -n1 mthreads-gmi', d: t('实时监控') },
                    ],
                },
                {
                    id: 'gpu-iluvatar', name: t('天数智芯'),
                    commands: [
                        { c: 'ixsmi', d: t('GPU 总览') },
                        { c: 'ixsmi -q', d: t('详细信息(含温度功耗)') },
                        { c: 'watch -n1 ixsmi', d: t('实时监控') },
                    ],
                },
                {
                    id: 'gpu-metax', name: t('沐曦'),
                    commands: [
                        { c: 'mx-smi', d: t('GPU 总览') },
                        { c: 'watch -n1 mx-smi', d: t('实时监控') },
                    ],
                },
                {
                    id: 'gpu-amd', name: 'AMD',
                    commands: [
                        { c: 'amd-smi', d: t('GPU 总览') },
                        { c: 'amd-smi metric', d: t('实时指标(温度/功耗/利用率)') },
                    ],
                },
                {
                    id: 'gpu-intel', name: 'Intel',
                    commands: [
                        { c: 'xpu-smi discovery', d: t('GPU 设备发现') },
                        { c: 'xpu-smi stats -d 0', d: t('设备 0 实时统计') },
                    ],
                },
            ],
        },
        {
            id: 'npu', name: 'NPU', subgroups: [
                {
                    id: 'npu-ascend', name: t('华为昇腾'),
                    commands: [
                        { c: 'npu-smi info', d: t('NPU 总览') },
                        { c: 'npu-smi info -l', d: t('NPU 设备列表') },
                        { c: 'npu-smi info -t board -i 0', d: t('卡板信息') },
                        { c: 'npu-smi info -t usages -i 0', d: t('利用率与显存') },
                        { c: 'npu-smi info -t temp -i 0', d: t('NPU 温度') },
                        { c: 'npu-smi info -t power -i 0', d: t('NPU 功耗') },
                        { c: 'watch -n1 npu-smi info', d: t('实时监控') },
                    ],
                },
                {
                    id: 'npu-cambricon', name: t('寒武纪 MLU'),
                    commands: [
                        { c: 'cnmon', d: t('NPU 总览') },
                        { c: 'cnmon info', d: t('详细信息(含温度功耗)') },
                        { c: 'cnmon info -c 0', d: t('指定卡详情') },
                        { c: 'watch -n1 cnmon', d: t('实时监控') },
                    ],
                },
            ],
        },
    ]);
    const cmdSearch = ref('');
    const cmdGroupOpen = ref({});          // 分组折叠状态，默认全部收起

    // 按关键字过滤（匹配命令或说明），支持二级分组；搜索时父子全部展开
    const visibleCmdGroups = computed(() => {
        const kw = cmdSearch.value.trim().toLowerCase();
        const match = x => x.c.toLowerCase().includes(kw) || x.d.toLowerCase().includes(kw);
        const filterCmds = list => (kw ? list.filter(match) : list);
        return commandGroups.value
            .map(g => {
                const open = kw ? true : cmdGroupOpen.value[g.id] === true;
                if (g.subgroups) {
                    const subgroups = g.subgroups
                        .map(s => ({
                            ...s,
                            commands: filterCmds(s.commands),
                            open: kw ? true : cmdGroupOpen.value[s.id] === true,
                        }))
                        .filter(s => s.commands.length);
                    return { ...g, commands: [], subgroups, open };
                }
                return { ...g, commands: filterCmds(g.commands), open };
            })
            .filter(g => (g.subgroups ? g.subgroups.length : g.commands.length));
    });

    function toggleCmdGroup(id) {
        cmdGroupOpen.value[id] = cmdGroupOpen.value[id] !== true;
    }

    /** 将命令注入终端：含 <占位符> 的只输入不回车，等用户补全；否则直接执行。 */
    function sendCommand(cmd) {
        if (terminalStatus.value !== 'connected' || !termWS || termWS.readyState !== WebSocket.OPEN) {
            ctx.toastNotify(t('终端未连接'), true);
            return;
        }
        const needsArgs = /<[^>]+>/.test(cmd);
        termWS.send(JSON.stringify({ type: 'input', data: cmd + (needsArgs ? '' : '\r') }));
        term && term.focus();
    }

    /** 点击终端 Tab：拦截本地节点，其余进入并初始化终端。 */
    function openTerminalTab() {
        if (!ctx.selectedServer.value) { ctx.toastNotify(t('请先选择服务器'), true); return; }
        if (ctx.selectedServer.value === 'srv_local') { ctx.toastNotify(t('本地节点暂不支持终端'), true); return; }
        ctx.activeTab.value = 'terminal';
        nextTick(() => initTerminal());
    }

    /** 创建 xterm 实例并挂载、自适应初始尺寸（服务器终端与容器终端共用）。 */
    function createXterm(mount) {
        const t = new window.Terminal(XTERM_OPTIONS);
        const fit = new window.FitAddon.FitAddon();
        t.loadAddon(fit);
        t.open(mount);
        try { fit.fit(); } catch (e) { /* 容器尚未布局时忽略 */ }
        return { term: t, fit };
    }

    /** 绑定终端 WebSocket 事件与 xterm 输入/尺寸联动（两处终端共用）。 */
    function attachWSHandlers(ws, t, statusRef, doResize) {
        ws.binaryType = 'arraybuffer';
        ws.onopen = () => {
            statusRef.value = 'connected';
            t.focus();
            doResize();
        };
        ws.onmessage = (event) => {
            // 二进制帧 = 远端原始输出
            if (event.data instanceof ArrayBuffer) {
                t.write(new Uint8Array(event.data));
                return;
            }
            // 文本帧 = 控制消息（exit）
            try {
                const msg = JSON.parse(event.data);
                if (msg.type === 'exit') {
                    t.write('\r\n\x1b[33m● ' + (msg.error ? t('连接失败: ') + msg.error : t('会话已结束')) + '\x1b[0m\r\n');
                    statusRef.value = msg.error ? 'error' : 'closed';
                }
            } catch (e) { /* 忽略 */ }
        };
        ws.onclose = () => {
            if (statusRef.value === 'connecting') {
                statusRef.value = 'error';
                t.write('\r\n\x1b[31m● ' + t('连接失败，请检查 SSH 配置') + '\x1b[0m\r\n');
            } else if (statusRef.value === 'connected') {
                statusRef.value = 'closed';
                t.write('\r\n\x1b[31m● ' + t('连接已断开') + '\x1b[0m\r\n');
            }
        };
        ws.onerror = () => {
            if (statusRef.value === 'connecting') statusRef.value = 'error';
        };

        // 键盘输入 → 远端 stdin
        t.onData(data => {
            if (ws.readyState === WebSocket.OPEN) {
                ws.send(JSON.stringify({ type: 'input', data }));
            }
        });
        // 终端尺寸变化 → 远端 PTY resize
        t.onResize(({ cols, rows }) => {
            if (ws.readyState === WebSocket.OPEN) {
                ws.send(JSON.stringify({ type: 'resize', cols, rows }));
            }
        });
    }

    /** 初始化 xterm 并建立终端 WebSocket。 */
    function initTerminal() {
        if (!terminalMount.value || terminalStatus.value === 'connecting') return;
        destroyTerminal();
        terminalStatus.value = 'connecting';

        const created = createXterm(terminalMount.value);
        term = created.term;
        termFit = created.fit;

        const protocol = location.protocol === 'https:' ? 'wss' : 'ws';
        termWS = new WebSocket(`${protocol}://${location.host}/api/v1/terminal?server=${encodeURIComponent(ctx.selectedServer.value)}`);
        attachWSHandlers(termWS, term, terminalStatus, sendTerminalResize);
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

    // ─── 容器终端（docker exec -it，弹窗内独立 xterm + WS 会话）───
    const dockerTermMount = ref(null);        // 弹窗内 xterm 挂载容器（模板 ref）
    const dockerTermVisible = ref(false);
    const dockerTermContainerId = ref('');
    const dockerTermStatus = ref('closed');   // connecting | connected | closed | error
    const dockerTermStatusText = computed(() => TERMINAL_STATUS_TEXT[dockerTermStatus.value] || t('未连接'));
    let dockerTerm = null;                    // 弹窗内 xterm 实例（非响应式）
    let dockerTermFit = null;
    let dockerTermWS = null;                  // 容器终端 WebSocket
    let dockerTermResizeHandler = null;       // 弹窗期间挂载的 window resize 监听

    /** 点击容器行「接入终端」：本地节点拦截，其余打开弹窗并初始化终端。 */
    function openContainerTerminal(containerId) {
        if (!ctx.selectedServer.value) { ctx.toastNotify(t('请先选择服务器'), true); return; }
        if (ctx.selectedServer.value === 'srv_local') { ctx.toastNotify(t('本地节点暂不支持终端接入'), true); return; }
        dockerTermContainerId.value = containerId;
        dockerTermVisible.value = true;
        nextTick(() => initContainerTerminal());
    }

    /** 初始化容器终端 xterm 并建立 WebSocket（URL 带 container 参数，后端走 docker exec -it）。 */
    function initContainerTerminal() {
        if (!dockerTermMount.value || dockerTermStatus.value === 'connecting') return;
        destroyContainerTerminal();
        dockerTermStatus.value = 'connecting';

        const created = createXterm(dockerTermMount.value);
        dockerTerm = created.term;
        dockerTermFit = created.fit;

        const protocol = location.protocol === 'https:' ? 'wss' : 'ws';
        const url = `${protocol}://${location.host}/api/v1/terminal?server=${encodeURIComponent(ctx.selectedServer.value)}&container=${encodeURIComponent(dockerTermContainerId.value)}`;
        dockerTermWS = new WebSocket(url);
        attachWSHandlers(dockerTermWS, dockerTerm, dockerTermStatus, sendContainerTermResize);

        dockerTermResizeHandler = () => sendContainerTermResize();
        window.addEventListener('resize', dockerTermResizeHandler);
    }

    /** 容器终端自适应并同步远端 PTY 尺寸。 */
    function sendContainerTermResize() {
        if (!dockerTerm || !dockerTermFit) return;
        try { dockerTermFit.fit(); } catch (e) { /* 忽略 */ }
        if (dockerTermWS && dockerTermWS.readyState === WebSocket.OPEN) {
            dockerTermWS.send(JSON.stringify({ type: 'resize', cols: dockerTerm.cols, rows: dockerTerm.rows }));
        }
    }

    /** 关闭容器终端弹窗并释放会话资源。 */
    function closeContainerTerminal() {
        destroyContainerTerminal();
        dockerTermStatus.value = 'closed';
        dockerTermVisible.value = false;
    }

    /** 销毁容器终端资源（WS + xterm 实例 + resize 监听）。 */
    function destroyContainerTerminal() {
        if (dockerTermResizeHandler) {
            window.removeEventListener('resize', dockerTermResizeHandler);
            dockerTermResizeHandler = null;
        }
        if (dockerTermWS) {
            dockerTermWS.onclose = null;
            dockerTermWS.onmessage = null;
            try { dockerTermWS.close(); } catch (e) { /* 忽略 */ }
            dockerTermWS = null;
        }
        if (dockerTerm) { dockerTerm.dispose(); dockerTerm = null; }
        dockerTermFit = null;
    }

    /** 窗口尺寸变化时重新自适应终端。 */
    function handleWindowResize() {
        if (ctx.activeTab.value === 'terminal' && terminalStatus.value === 'connected') sendTerminalResize();
    }

    // 切走终端 Tab 或切换服务器时断开会话，避免残留 SSH 连接；切换服务器同时关闭容器终端弹窗
    watch(ctx.activeTab, tab => { if (tab !== 'terminal') disconnectTerminal(); });
    watch(ctx.selectedServer, () => { disconnectTerminal(); if (dockerTermVisible.value) closeContainerTerminal(); });

    Object.assign(ctx, {
        terminalMount, terminalStatus, terminalStatusText,
        cmdSearch, visibleCmdGroups, toggleCmdGroup, sendCommand,
        openTerminalTab, reconnectTerminal, disconnectTerminal, handleWindowResize,
        dockerTermMount, dockerTermVisible, dockerTermContainerId,
        dockerTermStatus, dockerTermStatusText,
        openContainerTerminal, closeContainerTerminal,
    });
}
