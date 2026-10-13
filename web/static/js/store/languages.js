/**
 * languages.js — 编程语言环境域
 *
 * 语言环境配置的增删改、服务器语言检测、安装/卸载与安装日志查看。
 */

import {
    fetchLanguages, createLanguage, updateLanguage, deleteLanguage as apiDeleteLanguage,
    detectLanguages, installLanguage, uninstallLanguage,
} from '../api/languages.js';
import { parseVersions } from '../utils.js';

const { ref, reactive, computed } = window.Vue;

export function setupLanguages(ctx) {
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
            ctx.toastNotify(langFormMode.value === 'edit' ? '语言已更新' : '语言已添加');
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
        if (!await ctx.confirmDialog(`确定要安装「${langName}」吗？安装脚本将在服务器上执行，可能修改系统环境。`, { title: '安装语言环境', confirmText: '安装', danger: false })) return;
        langInstallName.value = langName;
        langInstallLogVisible.value = true;
        langInstallLog.value = '正在连接服务器...\n';
        langInstallStatus.value = 'installing';
        try {
            await installLanguage(ctx.selectedServer.value, languageId, version);
            langInstallLog.value += '\n==> 安装命令已发送到服务器\n==> 请在服务器上查看实际安装进度\n';
            langInstallStatus.value = 'ok';
            ctx.toastNotify('安装命令已发送，请查看日志');
        } catch (err) {
            langInstallLog.value += '\n==> 错误: ' + err.message + '\n';
            langInstallStatus.value = 'error';
            ctx.toastNotify('安装失败: ' + err.message, true);
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
        if (!ctx.selectedServer.value) return;
        const lang = languages.value.find(l => l.id === languageId);
        const langName = lang ? lang.name : languageId;
        if (!await ctx.confirmDialog(`确定要卸载「${langName}」吗？此操作不可恢复。`, { title: '卸载语言环境', confirmText: '卸载' })) return;
        langInstallName.value = langName;
        langInstallLogVisible.value = true;
        langInstallLog.value = '正在连接服务器...\n';
        langInstallStatus.value = 'installing';
        try {
            await uninstallLanguage(ctx.selectedServer.value, languageId);
            langInstallLog.value += '\n==> 卸载命令已发送到服务器\n==> 请在服务器上查看实际卸载进度\n';
            langInstallStatus.value = 'ok';
            ctx.toastNotify('卸载命令已发送，请查看日志');
        } catch (err) {
            langInstallLog.value += '\n==> 错误: ' + err.message + '\n';
            langInstallStatus.value = 'error';
            ctx.toastNotify('卸载失败: ' + err.message, true);
        }
    }

    Object.assign(ctx, {
        languages, langFormMode, langForm, langSaving, langFormError,
        detectedLanguages, langDetectLoading, langSearch, detectedLanguagesFiltered,
        langInstallLogVisible, langInstallLog, langInstallName, langInstallStatus,
        loadLanguages, openLangForm, closeLangForm, onLangSourceChange, saveLanguage,
        deleteLanguage: handleDeleteLanguage,
        detectServerLanguages, installLang, uninstallLang,
        viewInstallLog, refreshInstallLog,
    });
}
