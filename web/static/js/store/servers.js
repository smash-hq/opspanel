/**
 * servers.js — 服务器监控域
 *
 * 服务器列表、选中与系统快照详情、监控 WebSocket 生命周期，
 * 以及列表筛选相关的计算属性。
 */
import { t } from '../i18n/index.js';


import { fetchServers, loadServerInfo } from '../api/servers.js';
import { connectWS } from '../ws.js';
import { fmtBytes, number } from '../utils.js';

const { ref, reactive, computed } = window.Vue;

export function setupServers(ctx) {
    // ─── 核心数据状态 ───
    const servers = ref([]);
    const selectedServer = ref(null);
    const serverStatus = reactive({ local: 'unknown' });
    const info = reactive({});
    const loading = ref(false);
    const loadError = ref('');
    const wsConnected = ref(false);

    /** 当前选中的服务器对象。 */
    const currentServer = computed(() => {
        if (!selectedServer.value) return null;
        return servers.value.find(s => s.id === selectedServer.value) || null;
    });

    /** 当前服务器显示名称。 */
    const currentServerName = computed(() => currentServer.value?.name || '');

    /** 经过搜索和筛选后的服务器列表。 */
    const filteredServers = computed(() => {
        const query = ctx.searchQuery.value.trim().toLowerCase();
        return servers.value.filter(s => {
            const matchesQuery = !query || [s.name, s.host, s.username]
                .some(v => String(v || '').toLowerCase().includes(query));
            const matchesFilter = ctx.activeFilter.value === 'all'
                || ctx.activeFilter.value === s.type
                || (ctx.activeFilter.value === 'online' && serverStatus[s.id] === 'ok');
            return matchesQuery && matchesFilter;
        });
    });

    /** 在线服务器数量。 */
    const onlineCount = computed(() => servers.value.filter(s => serverStatus[s.id] === 'ok').length);

    /** 是否有详情数据。 */
    const hasInfo = computed(() => Boolean(info.timestamp));

    /** 详情数据最后更新时间。 */
    const lastUpdate = computed(() => info.timestamp ? new Date(info.timestamp).toLocaleTimeString('zh-CN') : '');

    /** 连接状态：error | loading | online。 */
    const connectionState = computed(() => {
        if (loadError.value && !hasInfo.value) return 'error';
        if (loading.value) return 'loading';
        return 'online';
    });

    /** 连接状态显示文本。 */
    const connectionLabel = computed(() => {
        if (connectionState.value === 'error') return t('连接失败');
        if (connectionState.value === 'loading') return t('连接中');
        return wsConnected.value ? t('实时在线') : t('已连接');
    });

    /** 磁盘挂载点数量。 */
    const diskCount = computed(() => (info.disks || []).length);

    /** 磁盘总容量（可读）。 */
    const diskTotal = computed(() => fmtBytes((info.disks || []).reduce((s, d) => s + (Number(d.total) || 0), 0)));

    /** 磁盘已用容量（可读）。 */
    const diskUsed = computed(() => fmtBytes((info.disks || []).reduce((s, d) => s + (Number(d.used) || 0), 0)));

    /** 磁盘总使用率。 */
    const diskPercent = computed(() => {
        const disks = info.disks || [];
        const total = disks.reduce((s, d) => s + (Number(d.total) || 0), 0);
        const used = disks.reduce((s, d) => s + (Number(d.used) || 0), 0);
        return total ? number(used / total * 100) : '0.0';
    });

    /** 系统 1 分钟负载。 */
    const loadValue = computed(() => info.load?.load1 ? Number(info.load.load1).toFixed(2) : '—');

    /** 统计指定筛选标签的服务器数量。 */
    function filterCount(filter) {
        if (filter === 'all') return servers.value.length;
        if (filter === 'remote') return servers.value.filter(s => s.type === 'remote').length;
        return onlineCount.value;
    }

    /** 加载服务器列表。 */
    async function loadServers() {
        try {
            servers.value = await fetchServers();
        } catch (err) {
            ctx.toastNotify(t('服务器列表加载失败'), true);
        }
    }

    /** 加载指定服务器的系统详情。 */
    async function fetchServerInfo(serverId) {
        loading.value = true;
        loadError.value = '';
        serverStatus[serverId] = 'loading';
        Object.keys(info).forEach(key => delete info[key]);
        try {
            const data = await loadServerInfo(serverId);
            Object.assign(info, data);
            serverStatus[serverId] = 'ok';
        } catch (err) {
            loadError.value = err.message || t('获取服务器信息失败');
            serverStatus[serverId] = 'error';
        } finally {
            loading.value = false;
        }
    }

    /** 选中服务器：加载详情 + 切换 WebSocket + 软件状态。 */
    function selectServer(id) {
        selectedServer.value = id;
        ctx.activeMenu.value = 'main';
        ctx.activeTab.value = 'system';
        fetchServerInfo(id);
        reconnectWS();
        ctx.loadSoftwareStatus(id);
        ctx.checkDockerEnv();
    }

    /** 刷新当前选中服务器的数据。 */
    function refreshCurrent() {
        if (selectedServer.value) fetchServerInfo(selectedServer.value);
    }

    /** 重试连接（刷新数据 + 重连 WebSocket）。 */
    function retryLoad() {
        refreshCurrent();
        reconnectWS();
    }

    // ─── WebSocket 管理 ───

    function connectWebSocket() {
        if (!selectedServer.value) return;
        const id = selectedServer.value;
        ctx.wsController = connectWS(id, {
            onOpen: () => { wsConnected.value = true; },
            onClose: () => { wsConnected.value = false; },
            onSnapshot: (data, serverId) => {
                if (!serverId || serverId === selectedServer.value) {
                    Object.assign(info, data);
                    serverStatus[selectedServer.value] = 'ok';
                }
            },
        });
    }

    function reconnectWS() {
        if (ctx.wsController) ctx.wsController.disconnect();
        wsConnected.value = false;
        connectWebSocket();
    }

    Object.assign(ctx, {
        servers, selectedServer, serverStatus, info,
        loading, loadError, wsConnected,
        currentServer, currentServerName, filteredServers, onlineCount, hasInfo, lastUpdate,
        connectionState, connectionLabel, diskCount, diskTotal, diskUsed, diskPercent, loadValue,
        filterCount,
        loadServers, fetchServerInfo, selectServer, refreshCurrent, retryLoad,
    });
}
