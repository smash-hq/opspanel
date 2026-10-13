/**
 * software.js — 软件管理域
 *
 * 软件目录与安装状态、安装/卸载/服务控制、运行日志查看，
 * 以及版本下拉组合框的交互。
 */

import {
    fetchSoftwareCatalog, fetchSoftwareStatus, installSoftware,
    uninstallSoftware, controlService, fetchSoftwareLog,
} from '../api/software.js';

const { ref, reactive, computed } = window.Vue;

export function setupSoftware(ctx) {
    // ─── 软件管理状态 ───
    const softwareCatalog = ref([]);             // 软件目录
    const softwareStatuses = ref([]);            // 当前选中服务器的软件状态
    const softwareLoading = ref(false);          // 正在加载软件状态
    const softwareOperating = reactive({});      // { packageId: true } 正在安装/卸载中

    // ─── 日志查看 ───
    const logModalVisible = ref(false);
    const logData = ref({ package: '', server: '', content: '' });
    const logLoading = ref(false);
    const logLines = ref(100);

    // ─── 搜索与版本下拉 ───
    const softwareSearch = ref('');

    /** 每个软件/语言选择的安装版本 { id: version }。 */
    const selectedVersions = reactive({});

    /** 当前展开版本下拉的卡片 id（'' 表示全部收起）。 */
    const versionComboOpen = ref('');

    /** 展开/收起版本下拉。 */
    function toggleVersionCombo(id) {
        versionComboOpen.value = versionComboOpen.value === id ? '' : id;
    }

    /** 选中预置版本并收起下拉。 */
    function pickVersion(id, v) {
        selectedVersions[id] = v;
        versionComboOpen.value = '';
    }

    /** 点击组合框外部时收起所有版本下拉。 */
    function handleDocClickForCombo(e) {
        if (versionComboOpen.value && !e.target.closest('.sw-combo')) {
            versionComboOpen.value = '';
        }
    }
    document.addEventListener('click', handleDocClickForCombo);

    /** 未安装的软件目录。 */
    const availableSoftware = computed(() => {
        const installedIds = new Set(
            softwareStatuses.value.filter(s => s.installed).map(s => s.package.id)
        );
        return softwareCatalog.value.filter(p => !installedIds.has(p.id));
    });

    /** 模糊匹配后的可安装软件。 */
    const availableSoftwareFiltered = computed(() => {
        const q = softwareSearch.value.trim().toLowerCase();
        if (!q) return availableSoftware.value;
        return availableSoftware.value.filter(p =>
            p.name.toLowerCase().includes(q) ||
            (p.description || '').toLowerCase().includes(q) ||
            p.id.toLowerCase().includes(q)
        );
    });

    /** 加载软件目录。 */
    async function loadSoftwareCatalog() {
        try {
            softwareCatalog.value = await fetchSoftwareCatalog();
        } catch (err) {
            ctx.toastNotify('软件目录加载失败', true);
        }
    }

    /** 加载指定服务器的软件状态。 */
    async function loadSoftwareStatus(serverId) {
        if (!serverId) {
            softwareStatuses.value = [];
            return;
        }
        softwareLoading.value = true;
        try {
            const statuses = await fetchSoftwareStatus(serverId);
            softwareStatuses.value = statuses;
        } catch (err) {
            ctx.toastNotify('软件状态查询失败', true);
            softwareStatuses.value = [];
        } finally {
            softwareLoading.value = false;
        }
    }

    /** 安装软件。 */
    async function installPackage(packageId, version) {
        const serverId = ctx.selectedServer.value;
        if (!serverId || serverId === 'srv_local') { ctx.toastNotify('请先选择远程服务器', true); return; }
        softwareOperating[packageId] = true;
        try {
            await installSoftware(serverId, packageId, version);
            ctx.toastNotify('正在安装中，请稍后刷新查看状态');
        } catch (err) {
            ctx.toastNotify(err.message, true);
        } finally {
            softwareOperating[packageId] = false;
        }
    }

    /** 卸载软件。 */
    async function uninstallPackage(packageId) {
        const serverId = ctx.selectedServer.value;
        if (!serverId || serverId === 'srv_local') { ctx.toastNotify('请先选择远程服务器', true); return; }
        if (!await ctx.confirmDialog('确定要卸载此软件吗？卸载后数据可能会丢失。', { title: '卸载软件', confirmText: '卸载' })) return;
        softwareOperating[packageId] = true;
        try {
            await uninstallSoftware(serverId, packageId);
            ctx.toastNotify('正在卸载中，请稍后刷新查看状态');
        } catch (err) {
            ctx.toastNotify(err.message, true);
        } finally {
            softwareOperating[packageId] = false;
        }
    }

    /** 控制服务启停。 */
    async function controlPackageService(packageId, action) {
        const serverId = ctx.selectedServer.value;
        if (!serverId || serverId === 'srv_local') { ctx.toastNotify('请先选择远程服务器', true); return; }
        try {
            await controlService(serverId, packageId, action);
            ctx.toastNotify(`服务${action === 'start' ? '启动' : action === 'stop' ? '停止' : '重启'}成功`);
            setTimeout(() => loadSoftwareStatus(serverId), 1000);
        } catch (err) {
            ctx.toastNotify(err.message, true);
        }
    }

    /** 获取软件包的安装状态。 */
    function getPackageStatus(packageId) {
        return softwareStatuses.value.find(s => s.package.id === packageId) || null;
    }

    async function viewLog(packageId) {
        if (!ctx.selectedServer.value || ctx.selectedServer.value === 'srv_local') {
            ctx.toastNotify('请先选择远程服务器', true);
            return;
        }
        logModalVisible.value = true;
        logLoading.value = true;
        logData.value = { package: packageId, server: ctx.selectedServer.value, content: '' };
        try {
            const data = await fetchSoftwareLog(ctx.selectedServer.value, packageId, logLines.value);
            logData.value.content = data.log || '暂无日志';
        } catch (err) {
            logData.value.content = '获取日志失败: ' + err.message;
        } finally {
            logLoading.value = false;
        }
    }

    function closeLogModal() {
        logModalVisible.value = false;
    }

    Object.assign(ctx, {
        softwareCatalog, softwareStatuses, softwareLoading, softwareOperating,
        softwareSearch, selectedVersions,
        versionComboOpen, toggleVersionCombo, pickVersion, handleDocClickForCombo,
        availableSoftware, availableSoftwareFiltered,
        logModalVisible, logData, logLoading, logLines, viewLog, closeLogModal,
        loadSoftwareCatalog, loadSoftwareStatus,
        installPackage, uninstallPackage, controlPackageService, getPackageStatus,
    });
}
