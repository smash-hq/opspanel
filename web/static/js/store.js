/**
 * store.js — 响应式状态中心
 *
 * 集中管理所有应用状态、计算属性和业务操作。
 * 导出 setupStore() 工厂函数，供 app.js 在 Vue setup 中调用。
 */

// Vue 3 通过 CDN 全局加载，从 window 对象解构 API
// 注意：import 语句会被提升，所以将 Vue 解构放在 imports 之后
import { number, bounded, fmtBytes, fmtUptime, usageClass, statusText, serverAddress, showToast } from './utils.js';
import { fetchServers, createServer, updateServer, deleteServer as apiDeleteServer, loadServerInfo, testConnection, fetchSoftwareCatalog, fetchSoftwareStatus, installSoftware, uninstallSoftware, controlService, fetchSources, createSource, updateSource, deleteSource as apiDeleteSource, fetchSoftwareLog, fetchProcesses, killProcess, fetchProcessLog, fetchLanguages, createLanguage, updateLanguage, deleteLanguage as apiDeleteLanguage, detectLanguages, installLanguage, fetchFiles, fetchFileInfo, fetchFilePreview, downloadFile, checkDocker, fetchDockerImages, pullDockerImage, removeDockerImage, fetchDockerContainers, dockerContainerAction as apiDockerContainerAction, removeDockerContainer, dockerExec, fetchDockerLogs, fetchPackages, createPackage, updatePackage, deletePackage as apiDeletePackage, batchDeleteDockerImages, batchDockerContainerAction, fetchDockerSources, createDockerSource, updateDockerSource, deleteDockerSource as apiDeleteDockerSource, fetchServerMirror, applyServerMirror, uninstallLanguage, login as apiLogin, logout as apiLogout, fetchMe, changePassword as apiChangePassword } from './api.js';
import { connectWS } from './ws.js';

const { reactive, ref, computed, watch, onMounted, onUnmounted, nextTick } = window.Vue;

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

    // ─── 登录认证 ───
    const authed = ref(false);
    const authChecked = ref(false);              // 是否完成登录态探测
    const currentUser = ref('');
    const mustChangePassword = ref(false);       // 首次登录（仍为默认密码）强制改密
    const loginForm = reactive({ username: 'admin', password: 'admin123' }); // 默认账密预填写
    const loginError = ref('');
    const loginLoading = ref(false);
    const pwForm = reactive({ oldPassword: '', newPassword: '', confirmPassword: '' });
    const pwError = ref('');
    const pwSaving = ref(false);

    /** 探测登录态：已登录直接进入应用，否则展示登录页。 */
    async function initAuth() {
        try {
            const me = await fetchMe();
            enterApp(me.username, me.must_change_password);
        } catch {
            authed.value = false;
        } finally {
            authChecked.value = true;
        }
    }

    /** 登录成功后进入应用并加载业务数据。 */
    function enterApp(username, mustChange) {
        currentUser.value = username;
        mustChangePassword.value = !!mustChange;
        authed.value = true;
        loadAppData();
    }

    /** 登录后加载全部业务数据（未登录前不发任何业务请求）。 */
    function loadAppData() {
        loadServers();
        loadSoftwareCatalog();
        loadSources();
        loadSettings();
        loadLanguages();
    }

    /** 提交登录。 */
    async function handleLogin() {
        loginError.value = '';
        if (!loginForm.username || !loginForm.password) {
            loginError.value = '请输入用户名和密码';
            return;
        }
        loginLoading.value = true;
        try {
            const data = await apiLogin(loginForm.username, loginForm.password);
            enterApp(data.username, data.must_change_password);
            if (data.must_change_password) toastNotify('首次登录，请先修改默认密码');
        } catch (err) {
            loginError.value = err.message;
        } finally {
            loginLoading.value = false;
        }
    }

    /** 退出登录，回到登录页（重置为默认账密预填写）。 */
    async function handleLogout() {
        try { await apiLogout(); } catch { /* 会话可能已失效，忽略 */ }
        leaveApp();
        loginForm.username = 'admin';
        loginForm.password = 'admin123';
    }

    /** 会话失效（401）时回到登录页。 */
    function handleUnauthorized() {
        if (!authed.value) return;
        leaveApp();
        toastNotify('登录已过期，请重新登录', true);
    }

    /** 清理本地登录态并断开 WebSocket。 */
    function leaveApp() {
        authed.value = false;
        mustChangePassword.value = false;
        currentUser.value = '';
        if (wsController) wsController.disconnect();
    }

    /** 提交修改密码（首登强制改密共用）。 */
    async function handleChangePassword() {
        pwError.value = '';
        if (!pwForm.oldPassword) { pwError.value = '请输入当前密码'; return; }
        if (pwForm.newPassword.length < 8) { pwError.value = '新密码长度至少 8 位'; return; }
        if (pwForm.newPassword !== pwForm.confirmPassword) { pwError.value = '两次输入的新密码不一致'; return; }
        pwSaving.value = true;
        try {
            await apiChangePassword(pwForm.oldPassword, pwForm.newPassword);
            mustChangePassword.value = false;
            pwForm.oldPassword = '';
            pwForm.newPassword = '';
            pwForm.confirmPassword = '';
            toastNotify('密码已修改，后续登录请使用新密码');
        } catch (err) {
            pwError.value = err.message;
        } finally {
            pwSaving.value = false;
        }
    }

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
        applyBrand();
    }

    // 将启动信息应用到界面（标题 / favicon），品牌名通过 computed 响应式更新
    function applyBrand() {
        document.title = `${brandName.value} · 服务器管理`;
        const link = document.querySelector('link[rel="icon"]');
        if (link) link.href = '/api/v1/settings/favicon?' + Date.now();
    }

    const brandName = computed(() => settingsForm.panelName || 'Server Plane');
    const brandMark = computed(() => settingsForm.panelName ? settingsForm.panelName.slice(0, 2).toUpperCase() : 'SP');
    const welcomeMessage = computed(() => settingsForm.welcomeMessage);

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
            applyBrand();
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

    // ─── 全局确认框 ───
    const confirmState = reactive({
        visible: false, title: '操作确认', message: '',
        confirmText: '确定', danger: true, _resolve: null,
    });

    /** 友好的二次确认，替代原生 confirm：返回 Promise<boolean>。 */
    function confirmDialog(message, opts = {}) {
        return new Promise(resolve => {
            Object.assign(confirmState, {
                visible: true,
                title: opts.title || '操作确认',
                message,
                confirmText: opts.confirmText || '确定',
                danger: opts.danger !== false,
                _resolve: resolve,
            });
        });
    }

    function okConfirm() {
        confirmState.visible = false;
        confirmState._resolve && confirmState._resolve(true);
    }

    function cancelConfirm() {
        confirmState.visible = false;
        confirmState._resolve && confirmState._resolve(false);
    }

    /** 解析逗号/空格分隔的版本串为数组（去空、去重）。 */
    function parseVersions(str) {
        const seen = new Set();
        return (str || '').split(/[,，\s]+/).map(s => s.trim()).filter(Boolean).filter(v => !seen.has(v) && seen.add(v));
    }

    // ─── 软件包管理 ───
    const packageFormMode = ref(null);
    const packageForm = reactive({ id: '', name: '', category: 'tool', description: '', version: '', versions: '', icon: '', install_cmd: '{}', uninstall_cmd: '{}', status_cmd: '', version_cmd: '', start_cmd: '', stop_cmd: '', restart_cmd: '' });
    const packageSaving = ref(false);
    const packageFormError = ref('');

    function openPackageForm(mode, pkg) {
        packageFormMode.value = mode;
        packageFormError.value = '';
        if (mode === 'edit' && pkg) {
            Object.assign(packageForm, {
                id: pkg.id, name: pkg.name, category: pkg.category,
                description: pkg.description || '', version: pkg.version || '',
                versions: (pkg.versions || []).join(', '), icon: pkg.icon || '',
                install_cmd: JSON.stringify(pkg.InstallCmd || {}, null, 2),
                uninstall_cmd: JSON.stringify(pkg.UninstallCmd || {}, null, 2),
                status_cmd: pkg.StatusCmd || '', version_cmd: pkg.VersionCmd || '',
                start_cmd: pkg.StartCmd || '', stop_cmd: pkg.StopCmd || '', restart_cmd: pkg.RestartCmd || '',
            });
        } else {
            Object.assign(packageForm, { id: '', name: '', category: 'tool', description: '', version: '', versions: '', icon: '', install_cmd: '{}', uninstall_cmd: '{}', status_cmd: '', version_cmd: '', start_cmd: '', stop_cmd: '', restart_cmd: '' });
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
            description: packageForm.description, version: packageForm.version,
            versions: parseVersions(packageForm.versions), icon: packageForm.icon,
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
        if (!await confirmDialog(`确定删除软件包「${id}」吗？`, { title: '删除软件包' })) return;
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
    const langForm = reactive({ id: '', name: '', category: 'runtime', description: '', icon: '', versions: '', version_cmd: '', install_cmd: '', install_source: 'official' });
    const langSaving = ref(false);
    const langFormError = ref('');
    const detectedLanguages = ref([]);
    const langDetectLoading = ref(false);
    const langSearch = ref('');
    const langInstallLogVisible = ref(false);
    const langInstallLog = ref('');
    const langInstallName = ref('');
    const langInstallStatus = ref('');

    /** 模糊匹配后的未安装语言。 */
    const detectedLanguagesFiltered = computed(() => {
        const uninstalled = detectedLanguages.value.filter(l => l.status !== 'installed');
        const q = langSearch.value.trim().toLowerCase();
        if (!q) return uninstalled;
        return uninstalled.filter(l =>
            l.name.toLowerCase().includes(q) ||
            (l.id || '').toLowerCase().includes(q)
        );
    });

    // ─── 文件管理 ───
    const files = ref([]);
    const currentFilePath = ref('/');
    const filesLoading = ref(false);
    const fileSearchQuery = ref('');
    const fileSudoWarning = ref(false);
    const filePreviewVisible = ref(false);
    const filePreviewData = reactive({ path: '', content: '', size: 0 });
    const filePreviewLoading = ref(false);
    const fileDetailVisible = ref(false);
    const fileDetailData = reactive({ name: '', path: '', is_dir: false, size: 0, mode: '', mod_time: 0 });

    async function loadFiles(path) {
        if (!selectedServer.value || selectedServer.value === 'srv_local') return;
        filesLoading.value = true;
        try {
            const targetPath = path || currentFilePath.value;
            const result = await fetchFiles(selectedServer.value, targetPath);
            files.value = result.files || [];
            fileSudoWarning.value = !result.sudo_used;
            if (path) currentFilePath.value = path;
        } catch (err) {
            toastNotify('文件列表加载失败: ' + err.message, true);
            fileSudoWarning.value = false;
        } finally {
            filesLoading.value = false;
        }
    }

    function navigateToPath(path) {
        loadFiles(path);
    }

    async function openFilePreview(file) {
        if (!selectedServer.value || file.is_dir) return;
        filePreviewVisible.value = true;
        filePreviewLoading.value = true;
        filePreviewData.path = file.path;
        filePreviewData.content = '';
        try {
            const data = await fetchFilePreview(selectedServer.value, file.path, 100);
            filePreviewData.content = data.content || '空文件';
            filePreviewData.size = data.content ? data.content.length : 0;
        } catch (err) {
            filePreviewData.content = '读取失败: ' + err.message;
        } finally {
            filePreviewLoading.value = false;
        }
    }

    function closeFilePreview() {
        filePreviewVisible.value = false;
    }

    async function openFileDetail(file) {
        if (!selectedServer.value) return;
        fileDetailVisible.value = true;
        fileDetailData.name = file.name;
        fileDetailData.path = file.path;
        fileDetailData.is_dir = file.is_dir;
        fileDetailData.size = file.size;
        fileDetailData.mode = file.mode;
        fileDetailData.mod_time = file.mod_time;
        try {
            const info = await fetchFileInfo(selectedServer.value, file.path);
            Object.assign(fileDetailData, info);
        } catch (err) {
            // 使用已有数据
        }
    }

    function closeFileDetail() {
        fileDetailVisible.value = false;
    }

    async function handleDownloadFile(file) {
        if (!selectedServer.value) return;
        try {
            await downloadFile(selectedServer.value, file.path);
            toastNotify('下载已开始');
        } catch (err) {
            toastNotify('下载失败: ' + err.message, true);
        }
    }

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
                versions: (lang.versions || []).join(', '),
                version_cmd: lang.versionCmd || lang.version_cmd || '',
                install_cmd: lang.installCmd || lang.install_cmd || '',
            });
        } else {
            Object.assign(langForm, { id: '', name: '', category: 'runtime', description: '', icon: '', versions: '', version_cmd: '', install_cmd: '' });
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
            versions: parseVersions(langForm.versions),
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
        if (!await confirmDialog(`确定删除语言环境「${id}」吗？`, { title: '删除语言环境' })) return;
        try {
            await apiDeleteLanguage(id);
            await loadLanguages();
            toastNotify('语言已删除');
        } catch (err) {
            toastNotify(err.message, true);
        }
    }

    // ─── 配置详情弹窗（软件包 / 语言环境通用） ───
    const configDetailVisible = ref(false);
    const configDetailType = ref('pkg');
    const configDetailData = ref({});

    function openConfigDetail(type, item) {
        configDetailType.value = type;
        configDetailData.value = item || {};
        configDetailVisible.value = true;
    }

    function closeConfigDetail() {
        configDetailVisible.value = false;
    }

    const configDetailVersions = computed(() => configDetailData.value.versions || []);
    const configDetailInstall = computed(() => Object.entries(configDetailData.value.install_cmd || {}));
    const configDetailUninstall = computed(() => Object.entries(configDetailData.value.uninstall_cmd || {}));
    const configDetailEmpty = computed(() => {
        const d = configDetailData.value;
        if (!d || !d.id) return true;
        const hasLifecycle = d.status_cmd || d.start_cmd || d.stop_cmd || d.restart_cmd;
        if (configDetailType.value === 'pkg') {
            return !d.version_cmd && !Object.keys(d.install_cmd || {}).length && !Object.keys(d.uninstall_cmd || {}).length && !hasLifecycle;
        }
        return !d.version_cmd && !d.install_cmd && !d.uninstall_cmd;
    });

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

    async function installLang(languageId, version) {
        if (!selectedServer.value) return;
        // 查找语言名称
        const lang = languages.value.find(l => l.id === languageId);
        const langName = lang ? lang.name : languageId;
        if (!await confirmDialog(`确定要安装「${langName}」吗？安装脚本将在服务器上执行，可能修改系统环境。`, { title: '安装语言环境', confirmText: '安装', danger: false })) return;
        langInstallName.value = langName;
        langInstallLogVisible.value = true;
        langInstallLog.value = '正在连接服务器...\n';
        langInstallStatus.value = 'installing';
        try {
            await installLanguage(selectedServer.value, languageId, version);
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

    async function uninstallLang(languageId) {
        if (!selectedServer.value) return;
        const lang = languages.value.find(l => l.id === languageId);
        const langName = lang ? lang.name : languageId;
        if (!await confirmDialog(`确定要卸载「${langName}」吗？此操作不可恢复。`, { title: '卸载语言环境', confirmText: '卸载' })) return;
        langInstallName.value = langName;
        langInstallLogVisible.value = true;
        langInstallLog.value = '正在连接服务器...\n';
        langInstallStatus.value = 'installing';
        try {
            await uninstallLanguage(selectedServer.value, languageId);
            langInstallLog.value += '\n==> 卸载命令已发送到服务器\n==> 请在服务器上查看实际卸载进度\n';
            langInstallStatus.value = 'ok';
            toastNotify('卸载命令已发送，请查看日志');
        } catch (err) {
            langInstallLog.value += '\n==> 错误: ' + err.message + '\n';
            langInstallStatus.value = 'error';
            toastNotify('卸载失败: ' + err.message, true);
        }
    }

    // ─── Docker 管理 ───
    const dockerAvailable = ref(false);
    const dockerVersion = ref('');
    const dockerTabEnabled = ref(false);
    const dockerImages = ref([]);
    const dockerContainers = ref([]);
    const dockerLoading = ref(false);
    const dockerExecVisible = ref(false);
    const dockerExecData = reactive({ containerId: '', command: '', output: '' });
    const dockerLogsVisible = ref(false);
    const dockerLogsData = reactive({ containerId: '', logs: '' });
    const dockerSubTab = ref('images');
    const dockerPullInput = ref('');
    const dockerShowAll = ref(true);
    const selectedMirror = ref('');
    const mirrorSelectMode = ref('preset'); // "preset" | "custom"
    // Docker 镜像源管理
    const dockerSources = ref([]);
    const serverMirror = ref({ mirror: '', registry: '' });
    const serverMirrorLoading = ref(false);
    const dockerSourceFormVisible = ref(false);
    const dockerSourceForm = reactive({ id: '', name: '', url: '', enabled: true });
    const dockerSourceSaving = ref(false);
    // 批量操作
    const selectedImages = ref([]);
    const selectedContainers = ref([]);
    const batchActionLoading = ref(false);

    async function checkDockerEnv() {
        if (!selectedServer.value || selectedServer.value === 'srv_local') {
            dockerAvailable.value = false;
            dockerTabEnabled.value = false;
            return;
        }
        try {
            const data = await checkDocker(selectedServer.value);
            dockerAvailable.value = data.available;
            dockerVersion.value = data.version || '';
            dockerTabEnabled.value = data.available;
        } catch {
            dockerAvailable.value = false;
            dockerTabEnabled.value = false;
        }
    }

    /** 点击 Docker Tab：已启用则进入并刷新；灰色状态点击则重新检测环境。 */
    function onDockerTabClick() {
        if (dockerTabEnabled.value) {
            activeTab.value = 'docker';
            checkDockerEnv();
            loadDockerImages();
            loadDockerContainers(true);
            loadServerMirror();
            return;
        }
        // 未检测到环境（灰色）时允许手动重检：服务器可能是后来才安装的 Docker
        checkDockerEnv().then(() => {
            if (dockerTabEnabled.value) toastNotify('检测到 Docker 环境，已启用管理');
        });
    }

    async function loadDockerImages() {
        if (!selectedServer.value) return;
        dockerLoading.value = true;
        try {
            dockerImages.value = await fetchDockerImages(selectedServer.value);
        } catch (err) {
            toastNotify('镜像列表加载失败: ' + err.message, true);
        } finally {
            dockerLoading.value = false;
        }
    }

    async function loadDockerContainers(all) {
        if (!selectedServer.value || dockerLoading.value) return;
        dockerLoading.value = true;
        try {
            dockerContainers.value = await fetchDockerContainers(selectedServer.value, all);
        } catch (err) {
            toastNotify('容器列表加载失败: ' + err.message, true);
        } finally {
            dockerLoading.value = false;
        }
    }

    async function dockerPullImage(image) {
        if (!selectedServer.value) return;
        try {
            await pullDockerImage(selectedServer.value, image);
            toastNotify('正在拉取镜像: ' + image);
            setTimeout(() => loadDockerImages(), 5000);
        } catch (err) {
            toastNotify('拉取失败: ' + err.message, true);
        }
    }

    async function dockerRemoveImage(id) {
        if (!selectedServer.value) return;
        if (!await confirmDialog(`确定删除镜像 ${id} 吗？`, { title: '删除镜像' })) return;
        try {
            await removeDockerImage(selectedServer.value, id);
            toastNotify('镜像已删除');
            loadDockerImages();
        } catch (err) {
            toastNotify('删除失败: ' + err.message, true);
        }
    }

    async function dockerContainerAction(id, action) {
        if (!selectedServer.value) return;
        try {
            await apiDockerContainerAction(selectedServer.value, id, action);
            toastNotify('操作成功: ' + action);
            loadDockerContainers(true);
        } catch (err) {
            toastNotify('操作失败: ' + err.message, true);
        }
    }

    async function dockerRemoveContainer(id) {
        if (!selectedServer.value) return;
        if (!await confirmDialog(`确定删除容器 ${id} 吗？`, { title: '删除容器' })) return;
        try {
            await removeDockerContainer(selectedServer.value, id);
            toastNotify('容器已删除');
            loadDockerContainers(true);
        } catch (err) {
            toastNotify('删除失败: ' + err.message, true);
        }
    }

    async function dockerExecCommand(containerId, command) {
        if (!selectedServer.value) return;
        dockerExecVisible.value = true;
        dockerExecData.containerId = containerId;
        dockerExecData.command = command;
        dockerExecData.output = '执行中...';
        try {
            const data = await dockerExec(selectedServer.value, containerId, command);
            dockerExecData.output = data.output || '无输出';
        } catch (err) {
            dockerExecData.output = '执行失败: ' + err.message;
        }
    }

    async function dockerViewLogs(containerId) {
        if (!selectedServer.value) return;
        dockerLogsVisible.value = true;
        dockerLogsData.containerId = containerId;
        dockerLogsData.logs = '加载中...';
        try {
            const data = await fetchDockerLogs(selectedServer.value, containerId, 100);
            dockerLogsData.logs = data.logs || '暂无日志';
        } catch (err) {
            dockerLogsData.logs = '获取日志失败: ' + err.message;
        }
    }

    // ─── Docker 镜像源管理 ───
    async function loadDockerSources() {
        try {
            dockerSources.value = await fetchDockerSources();
        } catch (err) {
            toastNotify('镜像源列表加载失败', true);
        }
    }

    function openDockerSourceForm(src) {
        dockerSourceFormVisible.value = true;
        if (src) {
            Object.assign(dockerSourceForm, { id: src.id, name: src.name, url: src.url, enabled: src.enabled });
        } else {
            Object.assign(dockerSourceForm, { id: '', name: '', url: '', enabled: true });
        }
    }

    function closeDockerSourceForm() {
        dockerSourceFormVisible.value = false;
    }

    async function saveDockerSource() {
        if (!dockerSourceForm.name.trim() || !dockerSourceForm.url.trim()) {
            toastNotify('名称和地址不能为空', true);
            return;
        }
        const isEdit = !!dockerSourceForm.id;
        dockerSourceSaving.value = true;
        try {
            if (isEdit) {
                await updateDockerSource(dockerSourceForm.id, dockerSourceForm);
            } else {
                await createDockerSource({ ...dockerSourceForm, id: 'src_' + Date.now() });
            }
            await loadDockerSources();
            dockerSourceFormVisible.value = false;
            toastNotify(isEdit ? '镜像源已更新' : '镜像源已添加');
        } catch (err) {
            toastNotify('保存失败: ' + err.message, true);
        } finally {
            dockerSourceSaving.value = false;
        }
    }

    async function removeDockerSource(id) {
        if (!await confirmDialog('确定删除此镜像源吗？', { title: '删除镜像源' })) return;
        try {
            await deleteDockerSource(id);
            await loadDockerSources();
            toastNotify('镜像源已删除');
        } catch (err) {
            toastNotify('删除失败: ' + err.message, true);
        }
    }

    async function setDefaultSource(id) {
        try {
            await setDefaultDockerSource(id);
            await loadDockerSources();
            toastNotify('已设为默认镜像源');
        } catch (err) {
            toastNotify('设置失败: ' + err.message, true);
        }
    }

    // ─── 服务器镜像源 ───
    async function loadServerMirror() {
        if (!selectedServer.value || selectedServer.value === 'srv_local') return;
        serverMirrorLoading.value = true;
        try {
            const data = await fetchServerMirror(selectedServer.value);
            serverMirror.value = data;
        } catch (err) {
            // 静默失败
        } finally {
            serverMirrorLoading.value = false;
        }
    }

    async function applyMirrorToServer(mirror) {
        if (!selectedServer.value) return;
        if (!await confirmDialog(`确定要将镜像源「${mirror}」应用到当前服务器吗？应用后 Docker 将会重启。`, { title: '应用镜像源', confirmText: '应用' })) return;
        try {
            await applyServerMirror(selectedServer.value, mirror);
            serverMirror.value.mirror = mirror;
            toastNotify('镜像源已应用到服务器，Docker 正在重启');
        } catch (err) {
            toastNotify('应用失败: ' + err.message, true);
        }
    }

    function onMirrorSelectModeChange() {
        selectedMirror.value = '';
    }

    // ─── Docker 批量操作 ───
    function toggleImageSelection(id) {
        const idx = selectedImages.value.indexOf(id);
        if (idx >= 0) selectedImages.value.splice(idx, 1);
        else selectedImages.value.push(id);
    }

    function toggleContainerSelection(id) {
        const idx = selectedContainers.value.indexOf(id);
        if (idx >= 0) selectedContainers.value.splice(idx, 1);
        else selectedContainers.value.push(id);
    }

    async function batchDeleteImages() {
        if (!selectedImages.value.length) { toastNotify('请选择要删除的镜像', true); return; }
        if (!await confirmDialog(`确定删除选中的 ${selectedImages.value.length} 个镜像吗？`, { title: '批量删除镜像', confirmText: '删除' })) return;
        batchActionLoading.value = true;
        try {
            const result = await batchDeleteDockerImages(selectedServer.value, selectedImages.value);
            toastNotify(`已删除 ${result.deleted?.length || 0} 个镜像${result.errors?.length ? ', ' + result.errors.length + ' 个失败' : ''}`);
            selectedImages.value = [];
            loadDockerImages();
        } catch (err) {
            toastNotify('批量删除失败: ' + err.message, true);
        } finally {
            batchActionLoading.value = false;
        }
    }

    async function batchContainerAction(action) {
        if (!selectedContainers.value.length) { toastNotify('请选择要操作的容器', true); return; }
        const actionName = action === 'stop' ? '停止' : action === 'start' ? '启动' : action === 'restart' ? '重启' : action === 'rm' ? '删除' : action;
        if (!await confirmDialog(`确定对选中的 ${selectedContainers.value.length} 个容器执行「${actionName}」操作吗？`, { title: `批量${actionName}` })) return;
        batchActionLoading.value = true;
        try {
            const result = await batchDockerContainerAction(selectedServer.value, selectedContainers.value, action);
            toastNotify(`已处理 ${result.done?.length || 0} 个容器${result.errors?.length ? ', ' + result.errors.length + ' 个失败' : ''}`);
            selectedContainers.value = [];
            loadDockerContainers(true);
        } catch (err) {
            toastNotify('批量操作失败: ' + err.message, true);
        } finally {
            batchActionLoading.value = false;
        }
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
        if (!await confirmDialog(`确定要终止进程 PID: ${pid} 吗？${force ? '（强制终止）' : ''}`, { title: '终止进程', confirmText: '终止' })) return;
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

    // ─── Web 终端（独立 SSH 会话，不占用监控 WS） ───
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
        if (!selectedServer.value) { toastNotify('请先选择服务器', true); return; }
        if (selectedServer.value === 'srv_local') { toastNotify('本地节点暂不支持终端', true); return; }
        activeTab.value = 'terminal';
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
        termWS = new WebSocket(`${protocol}://${location.host}/api/v1/terminal?server=${encodeURIComponent(selectedServer.value)}`);
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
        if (activeTab.value === 'terminal' && terminalStatus.value === 'connected') sendTerminalResize();
    }

    // 切走终端 Tab 或切换服务器时断开会话，避免残留 SSH 连接
    watch(activeTab, tab => { if (tab !== 'terminal') disconnectTerminal(); });
    watch(selectedServer, () => disconnectTerminal());

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
        checkDockerEnv();
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
        form.port = settingsForm.defaultSSHPort || 22;
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
        if (!server || !await confirmDialog(`确定删除「${server.name}」吗？删除后连接配置不可恢复。`, { title: '删除服务器', confirmText: '删除' })) return;
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

    /** 搜索关键字（可安装软件）。 */
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

    // 文件管理辅助函数
    const filteredFiles = computed(() => {
        if (!fileSearchQuery.value) return files.value;
        const q = fileSearchQuery.value.toLowerCase();
        return files.value.filter(f => f.name.toLowerCase().includes(q));
    });

    // 目录在前、组内按名称排序（类似 ls -l 的默认展示）
    const sortedEntries = computed(() =>
        [...filteredFiles.value].sort((a, b) =>
            (Number(b.is_dir) - Number(a.is_dir)) || a.name.localeCompare(b.name)
        )
    );

    function getFileIcon(name) {
        const ext = name.split('.').pop()?.toLowerCase();
        const icons = {
            js: '📜', ts: '📜', py: '🐍', go: '🐹', java: '☕', rb: '💎',
            php: '🐘', rs: '🦀', c: '⚙️', cpp: '⚙️', h: '⚙️',
            json: '📋', yaml: '📋', yml: '📋', toml: '📋',
            md: '📝', txt: '📄', log: '📄', csv: '📊',
            html: '🌐', css: '🎨', xml: '📋',
            sh: '⌨️', bash: '⌨️', zsh: '⌨️',
            tar: '📦', gz: '📦', zip: '📦', rar: '📦',
            jpg: '🖼️', png: '🖼️', gif: '🖼️', svg: '🖼️',
            mp4: '🎬', mp3: '🎵', wav: '🎵',
            exe: '⚡', bin: '⚡', deb: '📦', rpm: '📦',
        };
        return icons[ext] || '📄';
    }

    function formatFileSize(bytes) {
        if (bytes === 0) return '0 B';
        const units = ['B', 'KB', 'MB', 'GB', 'TB'];
        const i = Math.floor(Math.log(bytes) / Math.log(1024));
        return (bytes / Math.pow(1024, i)).toFixed(1) + ' ' + units[i];
    }

    /** Unix 时间戳 → 本地时间字符串（文件修改时间）。 */
    function formatModTime(ts) {
        if (!ts) return '—';
        return new Date(ts * 1000).toLocaleString('zh-CN', { hour12: false });
    }

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
    async function installPackage(packageId, version) {
        const serverId = selectedServer.value;
        if (!serverId || serverId === 'srv_local') { toastNotify('请先选择远程服务器', true); return; }
        softwareOperating[packageId] = true;
        try {
            await installSoftware(serverId, packageId, version);
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
        if (!await confirmDialog('确定要卸载此软件吗？卸载后数据可能会丢失。', { title: '卸载软件', confirmText: '卸载' })) return;
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
        if (!src || !await confirmDialog(`确定删除软件源「${src.name}」吗？`, { title: '删除软件源' })) return;
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
        initAuth();
        window.addEventListener('keydown', handleKeydown);
        window.addEventListener('resize', handleWindowResize);
        window.addEventListener('smp:unauthorized', handleUnauthorized);
    });

    // 根据活跃 Tab 暂停/恢复系统快照推送（减少不必要的 SSH 连接）
    watch(activeTab, (tab) => {
        const paused = tab !== 'system';
        fetch('/api/v1/settings/push', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ paused }),
        }).catch(() => {});
    });

    onUnmounted(() => {
        clearTimeout(toastTimer);
        if (wsController) wsController.disconnect();
        disconnectTerminal();
        window.removeEventListener('keydown', handleKeydown);
        window.removeEventListener('resize', handleWindowResize);
        window.removeEventListener('smp:unauthorized', handleUnauthorized);
        document.removeEventListener('click', handleDocClickForCombo);
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
        // 登录认证
        authed, authChecked, currentUser, mustChangePassword,
        loginForm, loginError, loginLoading,
        pwForm, pwError, pwSaving,
        handleLogin, handleLogout, handleChangePassword,
        // 设置
        settingsForm, settingsSaving, loadSettings, saveSettings,
        brandName, brandMark, welcomeMessage,
        // 日志查看
        logModalVisible, logData, logLoading, logLines, viewLog, closeLogModal,
        // 数据状态
        servers, selectedServer, serverStatus, info,
        // UI 状态
        sidebarCollapsed, activeMenu, activeTab, configTab, themeVal, showDoc, searchQuery, activeFilter, filters, loading, loadError, wsConnected,
        // 全局确认框
        confirmState, okConfirm, cancelConfirm,
        // 版本下拉组合框
        versionComboOpen, toggleVersionCombo, pickVersion,
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
        softwareCatalog, softwareStatuses, softwareLoading, softwareOperating,
        softwareSearch, selectedVersions,
        availableSoftware, availableSoftwareFiltered,
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
        // 配置详情弹窗（软件包 / 语言环境通用）
        configDetailVisible, configDetailType, configDetailData,
        openConfigDetail, closeConfigDetail,
        configDetailVersions, configDetailInstall, configDetailUninstall, configDetailEmpty,
        // 进程管理
        processes, processLoading, processLogVisible, processLogData, processLogLoading,
        loadProcesses, killProcess: handleKillProcess, viewProcessLog, closeProcessLog,
        // 语言环境管理
        languages, langFormMode, langForm, langSaving, langFormError,
        detectedLanguages, langDetectLoading, langSearch, detectedLanguagesFiltered,
        langInstallLogVisible, langInstallLog, langInstallName, langInstallStatus,
        loadLanguages, openLangForm, closeLangForm, saveLanguage,
        deleteLanguage: handleDeleteLanguage, detectServerLanguages, installLang, uninstallLang,
        viewInstallLog, refreshInstallLog,
        // 文件管理
        files, currentFilePath, filesLoading, fileSearchQuery, fileSudoWarning,
        filePreviewVisible, filePreviewData, filePreviewLoading,
        fileDetailVisible, fileDetailData,
        loadFiles, navigateToPath, openFilePreview, closeFilePreview,
        openFileDetail, closeFileDetail, handleDownloadFile,
        filteredFiles, sortedEntries, getFileIcon, formatFileSize, formatModTime,
        // Web 终端
        terminalMount, terminalStatus, terminalStatusText,
        openTerminalTab, disconnectTerminal, reconnectTerminal,
        // Docker 管理
        dockerAvailable, dockerVersion, dockerTabEnabled,
        dockerImages, dockerContainers, dockerLoading,
        dockerExecVisible, dockerExecData, dockerLogsVisible, dockerLogsData,
        dockerSubTab, dockerPullInput, dockerShowAll, selectedMirror, mirrorSelectMode,
        dockerSources, dockerSourceFormVisible, dockerSourceForm, dockerSourceSaving,
        selectedImages, selectedContainers, batchActionLoading,
        checkDockerEnv, onDockerTabClick, loadDockerImages, loadDockerContainers,
        dockerPullImage, dockerRemoveImage, dockerContainerAction,
        dockerRemoveContainer, dockerExecCommand, dockerViewLogs,
        loadDockerSources, openDockerSourceForm, closeDockerSourceForm, saveDockerSource,
        removeDockerSource, setDefaultSource,
        serverMirror, serverMirrorLoading,
        loadServerMirror, applyMirrorToServer, onMirrorSelectModeChange,
        toggleImageSelection, toggleContainerSelection, batchDeleteImages, batchContainerAction,
        nextTick,
    };
}
