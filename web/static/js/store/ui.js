/**
 * ui.js — 全局 UI 域
 *
 * 布局状态（侧栏/菜单/Tab）、系统设置、品牌信息、全局确认框、
 * 配置详情弹窗与 Toast 通知。
 */
import { t, setLocale } from '../i18n/index.js';


import { showToast } from '../utils.js';
import { getSettings, updateSettings, uploadFaviconFile, resetFaviconFile } from '../api/settings.js';

const { ref, reactive, computed } = window.Vue;

export function setupUi(ctx) {
    // ─── 布局状态 ───
    const sidebarCollapsed = ref(false);
    const activeMenu = ref('main');           // "main" | "config" | "notifycenter"
    const activeTab = ref('system');          // "system" | "software" | "cluster" | ...
    const configTab = ref('sources');         // "sources" | "packages" | "startup" | "favicon"
    const showDoc = ref(false);
    const searchQuery = ref('');
    const activeFilter = ref('all');

    // ─── 系统设置 ───
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
            const json = await getSettings();
            if (json.code === 0 && json.data) {
                settingsForm.panelName = json.data.panel_name || '';
                settingsForm.adminEmail = json.data.admin_email || '';
                settingsForm.pushInterval = parseInt(json.data.push_interval) || 3;
                settingsForm.defaultSSHPort = parseInt(json.data.default_ssh_port) || 22;
                settingsForm.welcomeMessage = json.data.welcome_message || '';
                hasCustomFavicon.value = json.data.has_custom_favicon === '1';
                if (json.data.language) setLocale(json.data.language, { persist: false });
            }
        } catch (err) {
            // 使用默认值
        }
        applyBrand();
    }

    // 将启动信息应用到界面（标题 / favicon），品牌名通过 computed 响应式更新
    function applyBrand() {
        document.title = t('{p1} · 服务器管理', { p1: brandName.value });
        const link = document.querySelector('link[rel="icon"]');
        if (link) link.href = '/api/v1/settings/favicon?' + Date.now();
    }

    const brandName = computed(() => settingsForm.panelName || 'OpsPanel');
    const welcomeMessage = computed(() => settingsForm.welcomeMessage);

    async function saveSettings() {
        settingsSaving.value = true;
        try {
            const json = await updateSettings({
                panel_name: settingsForm.panelName,
                admin_email: settingsForm.adminEmail,
                push_interval: settingsForm.pushInterval,
                default_ssh_port: settingsForm.defaultSSHPort,
                welcome_message: settingsForm.welcomeMessage,
            });
            if (json.code !== 0) throw new Error(json.message || t('未知错误'));
            applyBrand();
            ctx.toastNotify(t('设置已保存'));
        } catch (err) {
            ctx.toastNotify(t('保存失败: ') + err.message, true);
        } finally {
            settingsSaving.value = false;
        }
    }

    // ─── 网站图标 ───
    const faviconTs = ref(Date.now());
    const hasCustomFavicon = ref(false);

    async function uploadFavicon(event) {
        const file = event.target.files[0];
        if (!file) return;
        if (file.size > 1024 * 1024) {
            ctx.toastNotify(t('图标文件过大（最大 1MB）'), true);
            return;
        }
        try {
            const json = await uploadFaviconFile(file);
            if (json.code !== 0) throw new Error(json.message || t('上传失败'));
            faviconTs.value = Date.now();
            hasCustomFavicon.value = true;
            // 更新 head 中的 favicon
            document.querySelector('link[rel="icon"]').href = '/api/v1/settings/favicon?' + faviconTs.value;
            ctx.toastNotify(t('图标已更新'));
        } catch (err) {
            ctx.toastNotify(t('上传失败: ') + err.message, true);
        }
    }

    async function resetFavicon() {
        try {
            const json = await resetFaviconFile();
            if (json.code !== 0) throw new Error(json.message || t('操作失败'));
            faviconTs.value = Date.now();
            hasCustomFavicon.value = false;
            document.querySelector('link[rel="icon"]').href = '/favicon.svg';
            ctx.toastNotify(t('已恢复默认图标'));
        } catch (err) {
            ctx.toastNotify(t('操作失败: ') + err.message, true);
        }
    }

    // ─── 全局确认框 ───
    const confirmState = reactive({
        visible: false, title: t('操作确认'), message: '',
        confirmText: t('确定'), danger: true, _resolve: null,
    });

    /** 友好的二次确认，替代原生 confirm：返回 Promise<boolean>。 */
    function confirmDialog(message, opts = {}) {
        return new Promise(resolve => {
            Object.assign(confirmState, {
                visible: true,
                title: opts.title || t('操作确认'),
                message,
                confirmText: opts.confirmText || t('确定'),
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

    // ─── Toast 状态 ───
    const toast = reactive({ visible: false, message: '', error: false });

    /** 显示 Toast 并自动管理定时器。 */
    function toastNotify(message, error = false) {
        clearTimeout(ctx.toastTimer);
        ctx.toastTimer = showToast(toast, message, error);
    }

    // ─── 筛选标签（computed 保证切换语言后重新翻译）───
    const filters = computed(() => [
        { value: 'all', label: t('全部') },
        { value: 'remote', label: t('远程') },
        { value: 'online', label: t('在线') },
    ]);

    // 切换语言时刷新浏览器标签页标题
    window.addEventListener('opspanel:langchange', applyBrand);

    Object.assign(ctx, {
        sidebarCollapsed, activeMenu, activeTab, configTab, showDoc, searchQuery, activeFilter, filters,
        settingsForm, settingsSaving, loadSettings, saveSettings,
        brandName, welcomeMessage,
        faviconTs, hasCustomFavicon, uploadFavicon, resetFavicon,
        confirmState, confirmDialog, okConfirm, cancelConfirm,
        configDetailVisible, configDetailType, configDetailData,
        openConfigDetail, closeConfigDetail,
        configDetailVersions, configDetailInstall, configDetailUninstall, configDetailEmpty,
        toast, toastNotify,
    });
}
