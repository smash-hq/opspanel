/**
 * mirrors.js — 镜像源管理域
 *
 * Docker 镜像源（Registry）的增删改与默认源设置，
 * 以及将镜像源应用到远程服务器。
 */

import {
    fetchDockerSources, createDockerSource, updateDockerSource,
    deleteDockerSource as apiDeleteDockerSource, setDefaultDockerSource,
    fetchServerMirror, applyServerMirror,
} from '../api/mirrors.js';

const { ref, reactive } = window.Vue;

export function setupMirrors(ctx) {
    // ─── Docker 镜像源管理 ───
    const dockerSources = ref([]);
    const serverMirror = ref({ mirror: '', registry: '' });
    const serverMirrorLoading = ref(false);
    const dockerSourceFormVisible = ref(false);
    const dockerSourceForm = reactive({ id: '', name: '', url: '', enabled: true });
    const dockerSourceSaving = ref(false);

    async function loadDockerSources() {
        try {
            dockerSources.value = await fetchDockerSources();
        } catch (err) {
            ctx.toastNotify('镜像源列表加载失败', true);
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
            ctx.toastNotify('名称和地址不能为空', true);
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
            ctx.toastNotify(isEdit ? '镜像源已更新' : '镜像源已添加');
        } catch (err) {
            ctx.toastNotify('保存失败: ' + err.message, true);
        } finally {
            dockerSourceSaving.value = false;
        }
    }

    async function removeDockerSource(id) {
        if (!await ctx.confirmDialog('确定删除此镜像源吗？', { title: '删除镜像源' })) return;
        try {
            await apiDeleteDockerSource(id);
            await loadDockerSources();
            ctx.toastNotify('镜像源已删除');
        } catch (err) {
            ctx.toastNotify('删除失败: ' + err.message, true);
        }
    }

    async function setDefaultSource(id) {
        try {
            await setDefaultDockerSource(id);
            await loadDockerSources();
            ctx.toastNotify('已设为默认镜像源');
        } catch (err) {
            ctx.toastNotify('设置失败: ' + err.message, true);
        }
    }

    // ─── 服务器镜像源 ───
    async function loadServerMirror() {
        if (!ctx.selectedServer.value || ctx.selectedServer.value === 'srv_local') return;
        serverMirrorLoading.value = true;
        try {
            const data = await fetchServerMirror(ctx.selectedServer.value);
            serverMirror.value = data;
        } catch (err) {
            // 静默失败
        } finally {
            serverMirrorLoading.value = false;
        }
    }

    async function applyMirrorToServer(mirror) {
        if (!ctx.selectedServer.value) return;
        if (!await ctx.confirmDialog(`确定要将镜像源「${mirror}」应用到当前服务器吗？应用后 Docker 将会重启。`, { title: '应用镜像源', confirmText: '应用' })) return;
        try {
            await applyServerMirror(ctx.selectedServer.value, mirror);
            serverMirror.value.mirror = mirror;
            ctx.toastNotify('镜像源已应用到服务器，Docker 正在重启');
        } catch (err) {
            ctx.toastNotify('应用失败: ' + err.message, true);
        }
    }

    function onMirrorSelectModeChange() {
        ctx.selectedMirror.value = '';
    }

    Object.assign(ctx, {
        dockerSources, serverMirror, serverMirrorLoading,
        dockerSourceFormVisible, dockerSourceForm, dockerSourceSaving,
        loadDockerSources, openDockerSourceForm, closeDockerSourceForm, saveDockerSource,
        removeDockerSource, setDefaultSource,
        loadServerMirror, applyMirrorToServer, onMirrorSelectModeChange,
    });
}
