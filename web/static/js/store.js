/**
 * store.js — 响应式状态中心
 *
 * 集中管理所有应用状态、计算属性和业务操作。
 * 导出 setupStore() 工厂函数，供 app.js 在 Vue setup 中调用。
 */

// Vue 3 通过 CDN 全局加载，从 window 对象解构 API
// 注意：import 语句会被提升，所以将 Vue 解构放在 imports 之后
import { number, bounded, fmtBytes, fmtUptime, usageClass, statusText, serverAddress, showToast } from './utils.js';
import { fetchServers, createServer, updateServer, deleteServer as apiDeleteServer, loadServerInfo, testConnection, fetchSoftwareCatalog, fetchSoftwareStatus, installSoftware, uninstallSoftware, controlService, fetchSources, createSource, updateSource, deleteSource as apiDeleteSource, fetchSoftwareLog, fetchProcesses, killProcess, fetchProcessLog, fetchLanguages, createLanguage, updateLanguage, deleteLanguage as apiDeleteLanguage, detectLanguages, installLanguage } from './api.js';
import { connectWS } from './ws.js';

const { reactive, ref, computed, onMounted, onUnmounted, nextTick } = window.Vue;

/**
 * 创建空的表单模板。
 * @returns {Object} 空白表单对象
 */
function emptyForm() {
    return {
        id: '', name: '', type: 'remote', host: '', port: 22,
        username: '', auth_type: 'password', password: '', key_path: '',
    };
}

/**
 * 构建并返回完整的应用状态对象。
 * 在 Vue setup() 中调用此函数，将返回值暴露给模板。
 */
export function setupStore() {
    // ─── 主题管理 ───
    const theme = ref(localStorage.getItem('smp_theme') || 'system');
    applyTheme(theme.value);

    function applyTheme(t) {
        const root = document.documentElement;
        if (t === 'dark') {
            root.dataset.theme = 'dark';
        } else if (t === 'light') {
            root.dataset.theme = 'light';
        } else {
            // 跟随系统
            const dark = window.matchMedia('(prefers-color-scheme: dark)').matches;
            root.dataset.theme = dark ? 'dark' : 'light';
        }
    }

    function setTheme(t) {
        theme.value = t;
        themeVal.value = t;
        localStorage.setItem('smp_theme', t);
        applyTheme(t);
        // 持久化到后端
        fetch('/api/v1/settings/', {
            method: 'PUT',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ theme: t }),
        }).catch(() => {});
    }

    // 监听系统主题变化
    window.matchMedia('(prefers-color-scheme: dark)').addEventListener('change', (e) => {
        if (theme.value === 'system') {
            document.documentElement.dataset.theme = e.matches ? 'dark' : 'light';
        }
    });

    // ─── 核心数据状态 ───
    const servers = ref([]);
    const selectedServer = ref(null);
    const serverStatus = reactive({ local: 'unknown' });
    const info = reactive({});

    // ─── UI 状态 ───
    const sidebarCollapsed = ref(false);
    const activeMenu = ref('main');           // "main" | "settings"
    const activeTab = ref('system');          // "system" | "software" | "cluster" | ...
    const configTab = ref('sources');         // "sources" | "packages" | "startup" | "favicon"
    const showDoc = ref(false);
    const themeVal = ref(theme.value);
    const settingsForm = reactive({
        panelName: '',
        adminEmail: '',
        pushInterval: 3,
        defaultSSHPort: 22,
        welcomeMessage: '',
    });
    const settingsSaving = ref(false);

    async function loadSettings() {
        try {
            const res = await fetch('/api/v1/settings/');
            const json = await res.json();
            if (json.code === 0 && json.data) {
                settingsForm.panelName = json.data.panel_name || '';
                settingsForm.adminEmail = json.data.admin_email || '';
                settingsForm.pushInterval = parseInt(json.data.push_interval) || 3;
                settingsForm.defaultSSHPort = parseInt(json.data.default_ssh_port) || 22;
                settingsForm.welcomeMessage = json.data.welcome_message || '';
            }
        } catch (err) {
            // 使用默认值
        }
    }

    async function saveSettings() {
        settingsSaving.value = true;
        try {
            await fetch('/api/v1/settings/', {
                method: 'PUT',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({
                    panel_name: settingsForm.panelName,
                    admin_email: settingsForm.adminEmail,
                    push_interval: settingsForm.pushInterval,
                    default_ssh_port: settingsForm.defaultSSHPort,
                    welcome_message: settingsForm.welcomeMessage,
                }),
            });
            toastNotify('设置已保存');
        } catch (err) {
            toastNotify('保存失败: ' + err.message, true);
        } finally {
            settingsSaving.value = false;
        }
    }
    const searchQuery = ref('');
    const activeFilter = ref('all');
    const loading = ref(false);
    const loadError = ref('');
    const wsConnected = ref(false);

    // ─── 表单状态 ───
    const formMode = ref(null);       // null | "add" | "edit"
    const formError = ref('');
    const saving = ref(false);
    const form = reactive(emptyForm());

    // ─── 连接测试状态 ───
    const testing = ref(false);
    const testResult = ref(null);     // { success, host?, error? } | null

    // ─── 软件管理状态 ───
    const softwareCatalog = ref([]);             // 软件目录
    const softwareStatuses = ref([]);            // 当前选中服务器的软件状态
    const softwareLoading = ref(false);          // 正在加载软件状态
    const softwareOperating = reactive({});      // { packageId: true } 正在安装/卸载中
    const softwareCategory = ref('all');         // 当前筛选分类

    // ─── 日志查看 ───
    const logModalVisible = ref(false);
    const logData = ref({ package: '', server: '', content: '' });
    const logLoading = ref(false);
    const logLines = ref(100);

    async function viewLog(packageId) {
        if (!selectedServer.value || selectedServer.value === 'srv_local') {
            toastNotify('请先选择远程服务器', true);
            return;
        }
        logModalVisible.value = true;
        logLoading.value = true;
        logData.value = { package: packageId, server: selectedServer.value, content: '' };
        try {
            const data = await fetchSoftwareLog(selectedServer.value, packageId, logLines.value);
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

    // ─── 软件包管理 ───
    const packageFormMode = ref(null);
    const packageForm = reactive({ id: '', name: '', category: 'tool', description: '', version: '', icon: '', install_cmd: '{}', uninstall_cmd: '{}', status_cmd: '', version_cmd: '', start_cmd: '', stop_cmd: '', restart_cmd: '' });
    const packageSaving = ref(false);
    const packageFormError = ref('');

    function openPackageForm(mode, pkg) {
        packageFormMode.value = mode;
        packageFormError.value = '';
        if (mode === 'edit' && pkg) {
            Object.assign(packageForm, {
                id: pkg.id, name: pkg.name, category: pkg.category,
                description: pkg.description || '', version: pkg.version || '', icon: pkg.icon || '',
                install_cmd: JSON.stringify(pkg.InstallCmd || {}, null, 2),
                uninstall_cmd: JSON.stringify(pkg.UninstallCmd || {}, null, 2),
                status_cmd: pkg.StatusCmd || '', version_cmd: pkg.VersionCmd || '',
                start_cmd: pkg.StartCmd || '', stop_cmd: pkg.StopCmd || '', restart_cmd: pkg.RestartCmd || '',
            });
        } else {
            Object.assign(packageForm, { id: '', name: '', category: 'tool', description: '', version: '', icon: '', install_cmd: '{}', uninstall_cmd: '{}', status_cmd: '', version_cmd: '', start_cmd: '', stop_cmd: '', restart_cmd: '' });
        }
    }

    function closePackageForm() {
        if (!packageSaving.value) packageFormMode.value = null;
    }

    async function savePackage() {
        packageFormError.value = '';
        if (!packageForm.name.trim()) { packageFormError.value = '请输入名称'; return; }
        if (!packageForm.id.trim()) { packageFormError.value = '请输入 ID'; return; }
        let installCmd, uninstallCmd;
        try {
            installCmd = JSON.parse(packageForm.install_cmd || '{}');
            uninstallCmd = JSON.parse(packageForm.uninstall_cmd || '{}');
        } catch (e) {
            packageFormError.value = '安装/卸载命令必须是有效的 JSON';
            return;
        }
        packageSaving.value = true;
        const payload = {
            id: packageForm.id, name: packageForm.name, category: packageForm.category,
            description: packageForm.description, version: packageForm.version, icon: packageForm.icon,
            install_cmd: installCmd, uninstall_cmd: uninstallCmd,
            status_cmd: packageForm.status_cmd, version_cmd: packageForm.version_cmd,
            start_cmd: packageForm.start_cmd, stop_cmd: packageForm.stop_cmd, restart_cmd: packageForm.restart_cmd,
        };
        try {
            if (packageFormMode.value === 'edit') {
                await updatePackage(packageForm.id, payload);
            } else {
                await createPackage(payload);
            }
            await loadSoftwareCatalog();
            software.LoadFromStore(swStore);
            packageFormMode.value = null;
            toastNotify(packageFormMode.value === 'edit' ? '软件包已更新' : '软件包已添加');
        } catch (err) {
            packageFormError.value = err.message;
        } finally {
            packageSaving.value = false;
        }
    }

    async function handleDeletePackage(id) {
        if (!confirm(`确定删除软件包「${id}」吗？`)) return;
        try {
            await deletePackage(id);
            await loadSoftwareCatalog();
            toastNotify('软件包已删除');
        } catch (err) {
            toastNotify(err.message, true);
        }
    }

    // ─── 编程语言环境 ───
    const languages = ref([]);
    const langFormMode = ref(null);
    const langForm = reactive({ id: '', name: '', category: 'runtime', description: '', icon: '', version_cmd: '', install_cmd: '', install_source: 'official' });
    const langSaving = ref(false);
    const langFormError = ref('');
    const detectedLanguages = ref([]);
    const langDetectLoading = ref(false);
    const langInstallLogVisible = ref(false);
    const langInstallLog = ref('');
    const langInstallName = ref('');
    const langInstallStatus = ref('');

    async function loadLanguages() {
        try {
            languages.value = await fetchLanguages();
        } catch (err) {
            toastNotify('语言列表加载失败', true);
        }
    }

    function openLangForm(mode, lang) {
        langFormMode.value = mode;
        langFormError.value = '';
        if (mode === 'edit' && lang) {
            Object.assign(langForm, {
                id: lang.id, name: lang.name, category: lang.category || 'runtime',
                description: lang.description || '', icon: lang.icon || '',
                version_cmd: lang.versionCmd || lang.version_cmd || '',
                install_cmd: lang.installCmd || lang.install_cmd || '',
            });
        } else {
            Object.assign(langForm, { id: '', name: '', category: 'runtime', description: '', icon: '', version_cmd: '', install_cmd: '' });
        }
    }

    function closeLangForm() {
        if (!langSaving.value) langFormMode.value = null;
    }

    // 根据选择的安装源自动生成安装脚本
    function onLangSourceChange() {
        const id = langForm.id.toLowerCase()
        const sources = {
            mirror_aliyun: {
                go: 'curl -fsSL https://mirrors.aliyun.com/golang/install.sh | bash',
                java: 'curl -fsSL https://mirrors.aliyun.com/opensource/JDK/install.sh | bash',
                python3: 'curl -fsSL https://mirrors.aliyun.com/pypi/get-pip.py | python3',
                nodejs: 'curl -fsSL https://mirrors.aliyun.com/nvm/install.sh | bash',
            },
            mirror_tsinghua: {
                go: 'curl -fsSL https://mirrors.tuna.tsinghua.edu.cn/golang/install.sh | bash',
                python3: 'pip3 install -i https://pypi.tuna.tsinghua.edu.cn/simple --upgrade pip',
                nodejs: 'curl -fsSL https://mirrors.tuna.tsinghua.edu.cn/nvm/install.sh | bash',
            },
            mirror_ustc: {
                go: 'curl -fsSL https://mirrors.ustc.edu.cn/golang/install.sh | bash',
                python3: 'pip3 install -i https://pypi.mirrors.ustc.edu.cn/simple --upgrade pip',
                nodejs: 'curl -fsSL https://mirrors.ustc.edu.cn/nvm/install.sh | bash',
            },
            sdkman: {
                java: 'curl -s "https://get.sdkman.io" | bash && source "$HOME/.sdkman/bin/sdkman-init.sh" && sdk install java 17.0.9-tem',
                kotlin: 'curl -s "https://get.sdkman.io" | bash && source "$HOME/.sdkman/bin/sdkman-init.sh" && sdk install kotlin',
                scala: 'curl -s "https://get.sdkman.io" | bash && source "$HOME/.sdkman/bin/sdkman-init.sh" && sdk install scala',
            }
        }
        const src = sources[langForm.install_source]
        if (src && src[id]) {
            langForm.install_cmd = src[id]
        }
    }

    async function saveLanguage() {
        langFormError.value = '';
        if (!langForm.name.trim()) { langFormError.value = '请输入名称'; return; }
        if (!langForm.id.trim()) { langFormError.value = '请输入 ID'; return; }
        langSaving.value = true;
        const payload = {
            id: langForm.id, name: langForm.name, category: langForm.category,
            description: langForm.description, icon: langForm.icon,
            version_cmd: langForm.version_cmd, install_cmd: langForm.install_cmd,
        };
        try {
            if (langFormMode.value === 'edit') {
                await updateLanguage(langForm.id, payload);
            } else {
                await createLanguage(payload);
            }
            await loadLanguages();
            langFormMode.value = null;
            toastNotify(langFormMode.value === 'edit' ? '语言已更新' : '语言已添加');
        } catch (err) {
            langFormError.value = err.message;
        } finally {
            langSaving.value = false;
        }
    }

    async function handleDeleteLanguage(id) {
        if (!confirm(`确定删除语言环境「${id}」吗？`)) return;
        try {
            await apiDeleteLanguage(id);
            await loadLanguages();
            toastNotify('语言已删除');
        } catch (err) {
            toastNotify(err.message, true);
        }
    }

    async function detectServerLanguages() {
        if (!selectedServer.value || selectedServer.value === 'srv_local') return;
        langDetectLoading.value = true;
        try {
            detectedLanguages.value = await detectLanguages(selectedServer.value);
        } catch (err) {
            toastNotify('检测失败: ' + err.message, true);
        } finally {
            langDetectLoading.value = false;
        }
    }

    async function installLang(languageId) {
        if (!selectedServer.value) return;
        // 查找语言名称
        const lang = languages.value.find(l => l.id === languageId);
        langInstallName.value = lang ? lang.name : languageId;
        langInstallLogVisible.value = true;
        langInstallLog.value = '正在连接服务器...\n';
        langInstallStatus.value = 'installing';
        try {
            await installLanguage(selectedServer.value, languageId);
            langInstallLog.value += '\n==> 安装命令已发送到服务器\n==> 请在服务器上查看实际安装进度\n';
            langInstallStatus.value = 'ok';
            toastNotify('安装命令已发送，请查看日志');
        } catch (err) {
            langInstallLog.value += '\n==> 错误: ' + err.message + '\n';
            langInstallStatus.value = 'error';
            toastNotify('安装失败: ' + err.message, true);
        }
    }

    function viewInstallLog() {
        langInstallLogVisible.value = true;
    }

    function refreshInstallLog() {
        // 重新检测已安装的语言
        detectServerLanguages();
    }

    // ─── 网站图标 ───
    const faviconTs = ref(Date.now());
    const hasCustomFavicon = ref(false);

    async function uploadFavicon(event) {
        const file = event.target.files[0];
        if (!file) return;
        if (file.size > 1024 * 1024) {
            toastNotify('图标文件过大（最大 1MB）', true);
            return;
        }
        try {
            const resp = await fetch('/api/v1/settings/favicon', {
                method: 'POST',
                body: file,
            });
            if (resp.ok) {
                faviconTs.value = Date.now();
                hasCustomFavicon.value = true;
                // 更新 head 中的 favicon
                document.querySelector('link[rel="icon"]').href = '/api/v1/settings/favicon?' + faviconTs.value;
                toastNotify('图标已更新');
            } else {
                toastNotify('上传失败', true);
            }
        } catch (err) {
            toastNotify('上传失败: ' + err.message, true);
        }
    }

    async function resetFavicon() {
        try {
            await fetch('/api/v1/settings/favicon', { method: 'DELETE' });
            faviconTs.value = Date.now();
            hasCustomFavicon.value = false;
            document.querySelector('link[rel="icon"]').href = '/favicon.svg';
            toastNotify('已恢复默认图标');
        } catch (err) {
            toastNotify('操作失败', true);
        }
    }

    // ─── 软件源管理状态 ───
    const sources = ref([]);
    const sourceFormMode = ref(null); // null | "add" | "edit"
    const sourceForm = reactive({ id: '', name: '', type: 'mirror', url: '', enabled: true, priority: 100 });
    const sourceSaving = ref(false);
    const sourceFormError = ref('');

    // ─── 进程管理状态 ───
    const processes = ref([]);
    const processLoading = ref(false);
    const processLogVisible = ref(false);
    const processLogData = ref({ pid: 0, content: '' });
    const processLogLoading = ref(false);

    async function loadProcesses(lines) {
        if (!selectedServer.value || selectedServer.value === 'srv_local') return;
        processLoading.value = true;
        try {
            processes.value = await fetchProcesses(selectedServer.value, lines || 50);
        } catch (err) {
            toastNotify('进程列表加载失败: ' + err.message, true);
        } finally {
            processLoading.value = false;
        }
    }

    async function handleKillProcess(pid, force) {
        if (!selectedServer.value) return;
        if (!confirm(`确定要终止进程 PID: ${pid} 吗？${force ? '（强制终止）' : ''}`)) return;
        try {
            await killProcess(selectedServer.value, pid, force);
            toastNotify(`进程 ${pid} 已终止`);
            loadProcesses();
        } catch (err) {
            toastNotify('终止失败: ' + err.message, true);
        }
    }

    async function viewProcessLog(pid) {
        if (!selectedServer.value) return;
        processLogVisible.value = true;
        processLogLoading.value = true;
        processLogData.value = { pid, content: '' };
        try {
            const data = await fetchProcessLog(selectedServer.value, pid, 50);
            processLogData.value.content = data.log || '暂无日志';
        } catch (err) {
            processLogData.value.content = '获取日志失败: ' + err.message;
        } finally {
            processLogLoading.value = false;
        }
    }

    function closeProcessLog() {
        processLogVisible.value = false;
    }

    // ─── Toast 状态 ───
    const toast = reactive({ visible: false, message: '', error: false });

    // ─── 内部运行时 ───
    let wsController = null;
    let toastTimer = null;

    // ─── 筛选标签 ───
    const filters = [
        { value: 'all', label: '全部' },
        { value: 'remote', label: '远程' },
        { value: 'online', label: '在线' },
    ];

    // ═══════════════════════════════════════════
    //  计算属性
    // ═══════════════════════════════════════════

    /** 当前选中的服务器对象。 */
    const currentServer = computed(() => {
        if (!selectedServer.value) return null;
        return servers.value.find(s => s.id === selectedServer.value) || null;
    });

    /** 当前服务器显示名称。 */
    const currentServerName = computed(() => currentServer.value?.name || '');

    /** 经过搜索和筛选后的服务器列表。 */
    const filteredServers = computed(() => {
        const query = searchQuery.value.trim().toLowerCase();
        return servers.value.filter(s => {
            const matchesQuery = !query || [s.name, s.host, s.username]
                .some(v => String(v || '').toLowerCase().includes(query));
            const matchesFilter = activeFilter.value === 'all'
                || activeFilter.value === s.type
                || (activeFilter.value === 'online' && serverStatus[s.id] === 'ok');
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
        if (connectionState.value === 'error') return '连接失败';
        if (connectionState.value === 'loading') return '连接中';
        return wsConnected.value ? '实时在线' : '已连接';
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

    // ═══════════════════════════════════════════
    //  工具方法（模板直接调用）
    // ═══════════════════════════════════════════

    /** 统计指定筛选标签的服务器数量。 */
    function filterCount(filter) {
        if (filter === 'all') return servers.value.length;
        if (filter === 'remote') return servers.value.filter(s => s.type === 'remote').length;
        return onlineCount.value;
    }

    /** 显示 Toast 并自动管理定时器。 */
    function toastNotify(message, error = false) {
        clearTimeout(toastTimer);
        toastTimer = showToast(toast, message, error);
    }

    // ═══════════════════════════════════════════
    //  服务器操作
    // ═══════════════════════════════════════════

    /** 加载服务器列表。 */
    async function loadServers() {
        try {
            servers.value = await fetchServers();
        } catch (err) {
            toastNotify('服务器列表加载失败', true);
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
            loadError.value = err.message || '获取服务器信息失败';
            serverStatus[serverId] = 'error';
        } finally {
            loading.value = false;
        }
    }

    /** 选中服务器：加载详情 + 切换 WebSocket + 软件状态。 */
    function selectServer(id) {
        selectedServer.value = id;
        activeMenu.value = 'main';
        activeTab.value = 'system';
        fetchServerInfo(id);
        reconnectWS();
        loadSoftwareStatus(id);
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

    // ═══════════════════════════════════════════
    //  WebSocket 管理
    // ═══════════════════════════════════════════

    function connectWebSocket() {
        if (!selectedServer.value) return;
        const id = selectedServer.value;
        wsController = connectWS(id, {
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
        if (wsController) wsController.disconnect();
        wsConnected.value = false;
        connectWebSocket();
    }

    // ═══════════════════════════════════════════
    //  表单操作
    // ═══════════════════════════════════════════

    /** 打开「添加服务器」表单。 */
    function openAdd() {
        formMode.value = 'add';
        formError.value = '';
        Object.assign(form, emptyForm());
    }

    /** 打开「编辑服务器」表单。 */
    function openEdit(server) {
        if (!server) return;
        formMode.value = 'edit';
        formError.value = '';
        Object.assign(form, {
            id: server.id, name: server.name, type: server.type,
            host: server.host || '', port: server.port || 22,
            username: server.username || '', auth_type: server.auth_type || 'password',
            password: '', key_path: server.key_path || '',
        });
    }

    /** 关闭表单。 */
    function closeForm() {
        if (!saving.value) formMode.value = null;
    }

    /** 校验表单并提交。 */
    async function saveServer() {
        formError.value = '';
        if (!form.name.trim()) { formError.value = '请输入服务器名称'; return; }
        if (form.type === 'remote' && (!form.host.trim() || !form.username.trim())) {
            formError.value = '远程服务器需要填写主机地址和用户名';
            return;
        }
        if (form.type === 'remote' && formMode.value === 'add' && form.auth_type === 'password' && !form.password) {
            formError.value = '请输入登录密码';
            return;
        }
        if (form.type === 'remote' && form.auth_type === 'key' && !form.key_path.trim()) {
            formError.value = '请输入私钥文件路径';
            return;
        }

        saving.value = true;
        const isEdit = formMode.value === 'edit';
        const payload = { name: form.name, type: form.type };
        if (form.type === 'remote') {
            Object.assign(payload, {
                host: form.host, port: form.port || 22,
                username: form.username, auth_type: form.auth_type,
            });
            if (form.auth_type === 'password' && form.password) payload.password = form.password;
            if (form.auth_type === 'key' && form.key_path) payload.key_path = form.key_path;
        }

        try {
            const saved = isEdit
                ? await updateServer(form.id, payload)
                : await createServer(payload);
            await loadServers();
            formMode.value = null;
            toastNotify(isEdit ? '服务器配置已更新' : '服务器已添加');
            if (!isEdit && saved?.id) selectServer(saved.id);
            else if (isEdit && selectedServer.value === form.id) refreshCurrent();
        } catch (err) {
            formError.value = err.message;
        } finally {
            saving.value = false;
        }
    }

    /** 删除服务器（带确认）。 */
    async function handleDeleteServer(id) {
        const server = servers.value.find(s => s.id === id);
        if (!server || !confirm(`确定删除「${server.name}」吗？删除后连接配置不可恢复。`)) return;
        try {
            await apiDeleteServer(id);
            if (selectedServer.value === id) {
                selectedServer.value = null;
                if (wsController) { wsController.disconnect(); wsController = null; }
            }
            await loadServers();
            toastNotify('服务器已删除');
        } catch (err) {
            toastNotify(err.message, true);
        }
    }

    // ═══════════════════════════════════════════
    //  连接测试
    // ═══════════════════════════════════════════

    /** 测试远程服务器连接（不保存）。在添加/编辑页面上调用。 */
    async function testServerConnection() {
        formError.value = '';
        testResult.value = null;

        // 基本校验
        if (!form.name.trim()) { formError.value = '请输入服务器名称'; return; }
        if (form.type === 'remote' && (!form.host.trim() || !form.username.trim())) {
            formError.value = '远程服务器需要填写主机地址和用户名';
            return;
        }
        if (form.type === 'remote' && form.auth_type === 'password' && !form.password) {
            formError.value = '请输入登录密码';
            return;
        }
        if (form.type === 'remote' && form.auth_type === 'key' && !form.key_path.trim()) {
            formError.value = '请输入私钥文件路径';
            return;
        }

        testing.value = true;
        const payload = { name: form.name, type: form.type };
        if (form.type === 'remote') {
            Object.assign(payload, {
                host: form.host, port: form.port || 22,
                username: form.username, auth_type: form.auth_type,
            });
            if (form.auth_type === 'password' && form.password) payload.password = form.password;
            if (form.auth_type === 'key' && form.key_path) payload.key_path = form.key_path;
        }

        try {
            const result = await testConnection(payload);
            testResult.value = result;
        } catch (err) {
            testResult.value = { success: false, error: err.message };
        } finally {
            testing.value = false;
        }
    }

    /** 清除测试结果（表单字段变更时调用）。 */
    function clearTestResult() {
        testResult.value = null;
    }

    // ═══════════════════════════════════════════
    //  软件管理
    // ═══════════════════════════════════════════

    /** 按分类筛选的软件目录。 */
    const filteredSoftware = computed(() => {
        if (softwareCategory.value === 'all') return softwareCatalog.value;
        return softwareCatalog.value.filter(p => p.category === softwareCategory.value);
    });

    /** 加载软件目录。 */
    async function loadSoftwareCatalog() {
        try {
            softwareCatalog.value = await fetchSoftwareCatalog();
        } catch (err) {
            toastNotify('软件目录加载失败', true);
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
            toastNotify('软件状态查询失败', true);
            softwareStatuses.value = [];
        } finally {
            softwareLoading.value = false;
        }
    }

    /** 安装软件。 */
    async function installPackage(packageId) {
        const serverId = selectedServer.value;
        if (!serverId || serverId === 'srv_local') { toastNotify('请先选择远程服务器', true); return; }
        softwareOperating[packageId] = true;
        try {
            await installSoftware(serverId, packageId);
            toastNotify('正在安装中，请稍后刷新查看状态');
        } catch (err) {
            toastNotify(err.message, true);
        } finally {
            softwareOperating[packageId] = false;
        }
    }

    /** 卸载软件。 */
    async function uninstallPackage(packageId) {
        const serverId = selectedServer.value;
        if (!serverId || serverId === 'srv_local') { toastNotify('请先选择远程服务器', true); return; }
        if (!confirm('确定要卸载此软件吗？数据可能会丢失。')) return;
        softwareOperating[packageId] = true;
        try {
            await uninstallSoftware(serverId, packageId);
            toastNotify('正在卸载中，请稍后刷新查看状态');
        } catch (err) {
            toastNotify(err.message, true);
        } finally {
            softwareOperating[packageId] = false;
        }
    }

    /** 控制服务启停。 */
    async function controlPackageService(packageId, action) {
        const serverId = selectedServer.value;
        if (!serverId || serverId === 'srv_local') { toastNotify('请先选择远程服务器', true); return; }
        try {
            await controlService(serverId, packageId, action);
            toastNotify(`服务${action === 'start' ? '启动' : action === 'stop' ? '停止' : '重启'}成功`);
            setTimeout(() => loadSoftwareStatus(serverId), 1000);
        } catch (err) {
            toastNotify(err.message, true);
        }
    }

    /** 获取软件包的安装状态。 */
    function getPackageStatus(packageId) {
        return softwareStatuses.value.find(s => s.package.id === packageId) || null;
    }

    // ═══════════════════════════════════════════
    //  软件源管理
    // ═══════════════════════════════════════════

    /** 加载软件源列表。 */
    async function loadSources() {
        try {
            sources.value = await fetchSources();
        } catch (err) {
            toastNotify('软件源列表加载失败', true);
        }
    }

    /** 打开软件源表单。 */
    function openSourceForm(mode, src) {
        sourceFormMode.value = mode;
        sourceFormError.value = '';
        if (mode === 'edit' && src) {
            Object.assign(sourceForm, {
                id: src.id, name: src.name, type: src.type,
                url: src.url, enabled: src.enabled, priority: src.priority,
            });
        } else {
            Object.assign(sourceForm, { id: '', name: '', type: 'mirror', url: '', enabled: true, priority: 100 });
        }
    }

    /** 关闭软件源表单。 */
    function closeSourceForm() {
        if (!sourceSaving.value) sourceFormMode.value = null;
    }

    /** 保存软件源。 */
    async function saveSource() {
        sourceFormError.value = '';
        if (!sourceForm.name.trim()) { sourceFormError.value = '请输入名称'; return; }
        if (!sourceForm.url.trim()) { sourceFormError.value = '请输入地址'; return; }
        if (!sourceForm.id.trim()) { sourceFormError.value = '请输入 ID'; return; }

        sourceSaving.value = true;
        const payload = {
            id: sourceForm.id,
            name: sourceForm.name,
            type: sourceForm.type,
            url: sourceForm.url,
            enabled: sourceForm.enabled,
            priority: sourceForm.priority,
        };

        try {
            if (sourceFormMode.value === 'edit') {
                await updateSource(sourceForm.id, payload);
                toastNotify('软件源已更新');
            } else {
                await createSource(payload);
                toastNotify('软件源已添加');
            }
            await loadSources();
            sourceFormMode.value = null;
        } catch (err) {
            sourceFormError.value = err.message;
        } finally {
            sourceSaving.value = false;
        }
    }

    /** 删除软件源。 */
    async function handleDeleteSource(id) {
        const src = sources.value.find(s => s.id === id);
        if (!src || !confirm(`确定删除软件源「${src.name}」吗？`)) return;
        try {
            await apiDeleteSource(id);
            await loadSources();
            toastNotify('软件源已删除');
        } catch (err) {
            toastNotify(err.message, true);
        }
    }

    // ═══════════════════════════════════════════
    //  生命周期
    // ═══════════════════════════════════════════

    onMounted(() => {
        loadServers();
        loadSoftwareCatalog();
        loadSources();
        loadSettings();
        loadLanguages();
        window.addEventListener('keydown', handleKeydown);
    });

    onUnmounted(() => {
        clearTimeout(toastTimer);
        if (wsController) wsController.disconnect();
        window.removeEventListener('keydown', handleKeydown);
    });

    function handleKeydown(event) {
        if (event.key === '/' && document.activeElement?.tagName !== 'INPUT') {
            event.preventDefault();
            document.querySelector('.search-box input')?.focus();
        }
    }

    // ═══════════════════════════════════════════
    //  导出给模板使用的全部状态和方法
    // ═══════════════════════════════════════════

    return {
        // 主题与图标
        theme, setTheme, themeVal, faviconTs, hasCustomFavicon, uploadFavicon, resetFavicon,
        // 设置
        settingsForm, settingsSaving, loadSettings, saveSettings,
        // 日志查看
        logModalVisible, logData, logLoading, logLines, viewLog, closeLogModal,
        // 数据状态
        servers, selectedServer, serverStatus, info,
        // UI 状态
        sidebarCollapsed, activeMenu, activeTab, configTab, themeVal, showDoc, searchQuery, activeFilter, filters, loading, loadError, wsConnected,
        // 表单状态
        formMode, formError, saving, form, toast,
        // 连接测试
        testing, testResult,
        // 计算属性
        currentServer, currentServerName, filteredServers, onlineCount, hasInfo, lastUpdate,
        connectionState, connectionLabel, diskCount, diskTotal, diskUsed, diskPercent, loadValue,
        // 工具方法
        number, bounded, fmtBytes, fmtUptime, usageClass, statusText, serverAddress,
        filterCount,
        // 操作
        selectServer, refreshCurrent, retryLoad,
        openAdd, openEdit, closeForm, saveServer,
        deleteServer: handleDeleteServer,
        testServerConnection, clearTestResult,
        // 软件管理
        softwareCatalog, softwareStatuses, softwareLoading, softwareOperating, softwareCategory,
        filteredSoftware,
        loadSoftwareCatalog, loadSoftwareStatus,
        installPackage, uninstallPackage, controlPackageService, getPackageStatus,
        // 软件源管理
        sources, sourceFormMode, sourceForm, sourceSaving, sourceFormError,
        openSourceForm, closeSourceForm, saveSource,
        deleteSource: handleDeleteSource,
        // 软件包管理
        packageFormMode, packageForm, packageSaving, packageFormError,
        openPackageForm, closePackageForm, savePackage,
        deletePackage: handleDeletePackage,
        // 进程管理
        processes, processLoading, processLogVisible, processLogData, processLogLoading,
        loadProcesses, killProcess: handleKillProcess, viewProcessLog, closeProcessLog,
        // 语言环境管理
        languages, langFormMode, langForm, langSaving, langFormError,
        detectedLanguages, langDetectLoading,
        langInstallLogVisible, langInstallLog, langInstallName, langInstallStatus,
        loadLanguages, openLangForm, closeLangForm, saveLanguage,
        deleteLanguage: handleDeleteLanguage, detectServerLanguages, installLang,
        viewInstallLog, refreshInstallLog,
        nextTick,
    };
}
