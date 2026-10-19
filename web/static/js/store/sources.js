/**
 * sources.js — 软件源管理域
 *
 * 上游软件源（apt/yum 镜像配置）的增删改。
 */
import { t } from '../i18n/index.js';


import { fetchSources, createSource, updateSource, deleteSource as apiDeleteSource } from '../api/sources.js';

const { ref, reactive } = window.Vue;

export function setupSources(ctx) {
    // ─── 软件源管理状态 ───
    const sources = ref([]);
    const sourceFormMode = ref(null); // null | "add" | "edit"
    const sourceForm = reactive({ id: '', name: '', type: 'mirror', url: '', enabled: true, priority: 100 });
    const sourceSaving = ref(false);
    const sourceFormError = ref('');

    /** 加载软件源列表。 */
    async function loadSources() {
        try {
            sources.value = await fetchSources();
        } catch (err) {
            ctx.toastNotify(t('软件源列表加载失败'), true);
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
        if (!sourceForm.name.trim()) { sourceFormError.value = t('请输入名称'); return; }
        if (!sourceForm.url.trim()) { sourceFormError.value = t('请输入地址'); return; }
        if (!sourceForm.id.trim()) { sourceFormError.value = t('请输入 ID'); return; }

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
                ctx.toastNotify(t('软件源已更新'));
            } else {
                await createSource(payload);
                ctx.toastNotify(t('软件源已添加'));
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
        if (!src || !await ctx.confirmDialog(t('确定删除软件源「{p1}」吗？', { p1: src.name }), { title: t('删除软件源') })) return;
        try {
            await apiDeleteSource(id);
            await loadSources();
            ctx.toastNotify(t('软件源已删除'));
        } catch (err) {
            ctx.toastNotify(err.message, true);
        }
    }

    Object.assign(ctx, {
        sources, sourceFormMode, sourceForm, sourceSaving, sourceFormError,
        loadSources, openSourceForm, closeSourceForm, saveSource,
        deleteSource: handleDeleteSource, handleDeleteSource,
    });
}
