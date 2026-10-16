/**
 * languages.js — 编程语言环境域
 *
 * 语言环境配置的增删改、服务器语言检测、安装/卸载与安装日志查看。
 */

import {
    fetchLanguages, createLanguage, updateLanguage, deleteLanguage as apiDeleteLanguage,
    detectLanguages, installLanguage, uninstallLanguage, fetchLanguageJob,
} from '../api/languages.js';
import { parseVersions } from '../utils.js';

const { ref, reactive, computed, nextTick } = window.Vue;

export function setupLanguages(ctx) {
    // ─── 编程语言环境 ───
    const languages = ref([]);
    const langFormMode = ref(null);
    const langForm = reactive({ id: '', name: '', category: 'runtime', description: '', icon: '', versions: '', version_cmd: '', install_cmd: '', uninstall_cmd: '' });
    const langSaving = ref(false);
    const langFormError = ref('');
    const detectedLanguages = ref([]);
    const langDetectLoading = ref(false);
    const langInstallLogVisible = ref(false);
    const langInstallLog = ref('');
    const langInstallName = ref('');
    const langInstallStatus = ref('');
    const langInstallAction = ref('install'); // install | uninstall
    const langLogViewer = ref(null);          // 弹窗日志容器，用于自动滚动

    // ─── 任务进度轮询：安装/卸载在服务端异步执行，这里实时拉取日志 ───
    let langJobTimer = null;
    let langJobOffset = 0;
    let langJobTarget = null; // { serverId, languageId, action }

    /** 状态栏文案：区分安装与卸载。 */
    const langStatusText = computed(() => {
        const verb = langInstallAction.value === 'uninstall' ? '卸载' : '安装';
        if (langInstallStatus.value === 'installing') return `${verb}中...`;
        if (langInstallStatus.value === 'ok') return `${verb}完成`;
        if (langInstallStatus.value === 'error') return `${verb}失败`;
        return '';
    });

    function appendLangLog(text) {
        langInstallLog.value += text;
        nextTick(() => {
            const el = langLogViewer.value;
            if (el) el.scrollTop = el.scrollHeight;
        });
    }

    function stopLangJobPoll() {
        if (langJobTimer) { clearInterval(langJobTimer); langJobTimer = null; }
    }

    function startLangJobPoll(serverId, languageId, action) {
        stopLangJobPoll();
        langJobOffset = 0;
        langJobTarget = { serverId, languageId, action };
        pollLangJob();
        langJobTimer = setInterval(pollLangJob, 1500);
    }

    async function pollLangJob() {
        if (!langJobTarget) return;
        const { serverId, languageId, action } = langJobTarget;
        try {
            const data = await fetchLanguageJob(serverId, languageId, action);
            // 增量追加新产生的日志
            if (typeof data.log === 'string' && data.log.length > langJobOffset) {
                appendLangLog(data.log.slice(langJobOffset));
                langJobOffset = data.log.length;
            }
            if (data.status === 'success') {
                langInstallStatus.value = 'ok';
                stopLangJobPoll();
                ctx.toastNotify(langInstallAction.value === 'uninstall' ? '卸载完成' : '安装完成');
                detectServerLanguages(); // 完成后重新检测已安装语言
            } else if (data.status === 'error') {
                langInstallStatus.value = 'error';
                stopLangJobPoll();
                ctx.toastNotify((langInstallAction.value === 'uninstall' ? '卸载' : '安装') + '失败，详见日志', true);
                detectServerLanguages();
            } else if (data.status === 'none') {
                // 服务端无此任务（如面板重启），标记失败并停止
                appendLangLog('\n==> 任务不存在（面板可能已重启），请重试\n');
                langInstallStatus.value = 'error';
                stopLangJobPoll();
            }
        } catch (err) {
            // 单次查询失败不打断轮询，等待下一次
        }
    }

    /** 未安装的语言（可安装列表）。 */
    const uninstalledLanguages = computed(() =>
        detectedLanguages.value.filter(l => l.status !== 'installed')
    );

    async function loadLanguages() {
        try {
            languages.value = await fetchLanguages();
        } catch (err) {
            ctx.toastNotify('语言列表加载失败', true);
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
                uninstall_cmd: lang.uninstallCmd || lang.uninstall_cmd || '',
            });
        } else {
            Object.assign(langForm, { id: '', name: '', category: 'runtime', description: '', icon: '', versions: '', version_cmd: '', install_cmd: '', uninstall_cmd: '' });
        }
    }

    function closeLangForm() {
        if (!langSaving.value) langFormMode.value = null;
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
            uninstall_cmd: langForm.uninstall_cmd,
        };
        try {
            const isEdit = langFormMode.value === 'edit';
            if (isEdit) {
                await updateLanguage(langForm.id, payload);
            } else {
                await createLanguage(payload);
            }
            await loadLanguages();
            langFormMode.value = null;
            ctx.toastNotify(isEdit ? '语言已更新' : '语言已添加');
        } catch (err) {
            langFormError.value = err.message;
        } finally {
            langSaving.value = false;
        }
    }

    async function handleDeleteLanguage(id) {
        if (!await ctx.confirmDialog(`确定删除语言环境「${id}」吗？`, { title: '删除语言环境' })) return;
        try {
            await apiDeleteLanguage(id);
            await loadLanguages();
            ctx.toastNotify('语言已删除');
        } catch (err) {
            ctx.toastNotify(err.message, true);
        }
    }

    async function detectServerLanguages() {
        if (!ctx.selectedServer.value) return; // 本机与远程节点均支持
        langDetectLoading.value = true;
        try {
            detectedLanguages.value = await detectLanguages(ctx.selectedServer.value);
        } catch (err) {
            ctx.toastNotify('检测失败: ' + err.message, true);
        } finally {
            langDetectLoading.value = false;
        }
    }

    async function installLang(languageId, version) {
        if (!ctx.selectedServer.value) return;
        // 查找语言名称
        const lang = languages.value.find(l => l.id === languageId);
        const langName = lang ? lang.name : languageId;
        const serverId = ctx.selectedServer.value;
        if (!await ctx.confirmDialog(`确定要安装「${langName}」吗？安装脚本将在服务器上执行，可能修改系统环境。`, { title: '安装语言环境', confirmText: '安装', danger: false })) return;
        langInstallName.value = langName;
        langInstallAction.value = 'install';
        langInstallStatus.value = 'installing';
        langInstallLog.value = '正在提交安装任务...\n';
        langInstallLogVisible.value = true;
        try {
            await installLanguage(serverId, languageId, version);
            // 提交成功后轮询任务进度，实时展示安装输出
            startLangJobPoll(serverId, languageId, 'install');
        } catch (err) {
            langInstallLog.value += '\n==> 提交失败: ' + err.message + '\n';
            langInstallStatus.value = 'error';
            ctx.toastNotify('安装失败: ' + err.message, true);
        }
    }

    function refreshInstallLog() {
        // 立即拉取一次进度，并重新检测已安装的语言
        pollLangJob();
        detectServerLanguages();
    }

    async function uninstallLang(languageId) {
        if (!ctx.selectedServer.value) return;
        const lang = languages.value.find(l => l.id === languageId);
        const langName = lang ? lang.name : languageId;
        const serverId = ctx.selectedServer.value;
        if (!await ctx.confirmDialog(`确定要卸载「${langName}」吗？此操作不可恢复。`, { title: '卸载语言环境', confirmText: '卸载' })) return;
        langInstallName.value = langName;
        langInstallAction.value = 'uninstall';
        langInstallStatus.value = 'installing';
        langInstallLog.value = '正在提交卸载任务...\n';
        langInstallLogVisible.value = true;
        try {
            await uninstallLanguage(serverId, languageId);
            // 提交成功后轮询任务进度，实时展示卸载输出
            startLangJobPoll(serverId, languageId, 'uninstall');
        } catch (err) {
            langInstallLog.value += '\n==> 提交失败: ' + err.message + '\n';
            langInstallStatus.value = 'error';
            ctx.toastNotify('卸载失败: ' + err.message, true);
        }
    }

    Object.assign(ctx, {
        languages, langFormMode, langForm, langSaving, langFormError,
        detectedLanguages, langDetectLoading, uninstalledLanguages,
        langInstallLogVisible, langInstallLog, langInstallName, langInstallStatus,
        langInstallAction, langStatusText, langLogViewer,
        loadLanguages, openLangForm, closeLangForm, saveLanguage,
        deleteLanguage: handleDeleteLanguage,
        detectServerLanguages, installLang, uninstallLang,
        refreshInstallLog,
    });
}
