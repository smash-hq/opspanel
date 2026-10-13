/**
 * docker.js — Docker 管理域
 *
 * 环境检测、镜像/容器管理、容器命令执行与日志、批量操作，
 * 以及应用镜像源时的选择状态。
 */

import {
    checkDocker, fetchDockerImages, pullDockerImage, removeDockerImage,
    fetchDockerContainers, dockerContainerAction as apiDockerContainerAction,
    removeDockerContainer, dockerExec, fetchDockerLogs,
    batchDeleteDockerImages, batchDockerContainerAction,
} from '../api/docker.js';

const { ref, reactive } = window.Vue;

export function setupDocker(ctx) {
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
    // 批量操作
    const selectedImages = ref([]);
    const selectedContainers = ref([]);
    const batchActionLoading = ref(false);

    async function checkDockerEnv() {
        if (!ctx.selectedServer.value) {
            dockerAvailable.value = false;
            dockerTabEnabled.value = false;
            return;
        }
        // 本机与远程节点均检测（本机需运行用户有 docker 权限，未安装/无权限时自动降级隐藏）
        try {
            const data = await checkDocker(ctx.selectedServer.value);
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
            ctx.activeTab.value = 'docker';
            checkDockerEnv();
            loadDockerImages();
            loadDockerContainers(true);
            ctx.loadServerMirror();
            return;
        }
        // 未检测到环境（灰色）时允许手动重检：服务器可能是后来才安装的 Docker
        checkDockerEnv().then(() => {
            if (dockerTabEnabled.value) ctx.toastNotify('检测到 Docker 环境，已启用管理');
        });
    }

    async function loadDockerImages() {
        if (!ctx.selectedServer.value) return;
        dockerLoading.value = true;
        try {
            dockerImages.value = await fetchDockerImages(ctx.selectedServer.value);
        } catch (err) {
            ctx.toastNotify('镜像列表加载失败: ' + err.message, true);
        } finally {
            dockerLoading.value = false;
        }
    }

    async function loadDockerContainers(all) {
        if (!ctx.selectedServer.value || dockerLoading.value) return;
        dockerLoading.value = true;
        try {
            dockerContainers.value = await fetchDockerContainers(ctx.selectedServer.value, all);
        } catch (err) {
            ctx.toastNotify('容器列表加载失败: ' + err.message, true);
        } finally {
            dockerLoading.value = false;
        }
    }

    async function dockerPullImage(image) {
        if (!ctx.selectedServer.value) return;
        try {
            await pullDockerImage(ctx.selectedServer.value, image);
            ctx.toastNotify('正在拉取镜像: ' + image);
            setTimeout(() => loadDockerImages(), 5000);
        } catch (err) {
            ctx.toastNotify('拉取失败: ' + err.message, true);
        }
    }

    async function dockerRemoveImage(id) {
        if (!ctx.selectedServer.value) return;
        if (!await ctx.confirmDialog(`确定删除镜像 ${id} 吗？`, { title: '删除镜像' })) return;
        try {
            await removeDockerImage(ctx.selectedServer.value, id);
            ctx.toastNotify('镜像已删除');
            loadDockerImages();
        } catch (err) {
            ctx.toastNotify('删除失败: ' + err.message, true);
        }
    }

    async function dockerContainerAction(id, action) {
        if (!ctx.selectedServer.value) return;
        try {
            await apiDockerContainerAction(ctx.selectedServer.value, id, action);
            ctx.toastNotify('操作成功: ' + action);
            loadDockerContainers(true);
        } catch (err) {
            ctx.toastNotify('操作失败: ' + err.message, true);
        }
    }

    async function dockerRemoveContainer(id) {
        if (!ctx.selectedServer.value) return;
        if (!await ctx.confirmDialog(`确定删除容器 ${id} 吗？`, { title: '删除容器' })) return;
        try {
            await removeDockerContainer(ctx.selectedServer.value, id);
            ctx.toastNotify('容器已删除');
            loadDockerContainers(true);
        } catch (err) {
            ctx.toastNotify('删除失败: ' + err.message, true);
        }
    }

    async function dockerExecCommand(containerId, command) {
        if (!ctx.selectedServer.value) return;
        dockerExecVisible.value = true;
        dockerExecData.containerId = containerId;
        dockerExecData.command = command;
        dockerExecData.output = '执行中...';
        try {
            const data = await dockerExec(ctx.selectedServer.value, containerId, command);
            dockerExecData.output = data.output || '无输出';
        } catch (err) {
            dockerExecData.output = '执行失败: ' + err.message;
        }
    }

    async function dockerViewLogs(containerId) {
        if (!ctx.selectedServer.value) return;
        dockerLogsVisible.value = true;
        dockerLogsData.containerId = containerId;
        dockerLogsData.logs = '加载中...';
        try {
            const data = await fetchDockerLogs(ctx.selectedServer.value, containerId, 100);
            dockerLogsData.logs = data.logs || '暂无日志';
        } catch (err) {
            dockerLogsData.logs = '获取日志失败: ' + err.message;
        }
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
        if (!selectedImages.value.length) { ctx.toastNotify('请选择要删除的镜像', true); return; }
        if (!await ctx.confirmDialog(`确定删除选中的 ${selectedImages.value.length} 个镜像吗？`, { title: '批量删除镜像', confirmText: '删除' })) return;
        batchActionLoading.value = true;
        try {
            const result = await batchDeleteDockerImages(ctx.selectedServer.value, selectedImages.value);
            ctx.toastNotify(`已删除 ${result.deleted?.length || 0} 个镜像${result.errors?.length ? ', ' + result.errors.length + ' 个失败' : ''}`);
            selectedImages.value = [];
            loadDockerImages();
        } catch (err) {
            ctx.toastNotify('批量删除失败: ' + err.message, true);
        } finally {
            batchActionLoading.value = false;
        }
    }

    async function batchContainerAction(action) {
        if (!selectedContainers.value.length) { ctx.toastNotify('请选择要操作的容器', true); return; }
        const actionName = action === 'stop' ? '停止' : action === 'start' ? '启动' : action === 'restart' ? '重启' : action === 'rm' ? '删除' : action;
        if (!await ctx.confirmDialog(`确定对选中的 ${selectedContainers.value.length} 个容器执行「${actionName}」操作吗？`, { title: `批量${actionName}` })) return;
        batchActionLoading.value = true;
        try {
            const result = await batchDockerContainerAction(ctx.selectedServer.value, selectedContainers.value, action);
            ctx.toastNotify(`已处理 ${result.done?.length || 0} 个容器${result.errors?.length ? ', ' + result.errors.length + ' 个失败' : ''}`);
            selectedContainers.value = [];
            loadDockerContainers(true);
        } catch (err) {
            ctx.toastNotify('批量操作失败: ' + err.message, true);
        } finally {
            batchActionLoading.value = false;
        }
    }

    Object.assign(ctx, {
        dockerAvailable, dockerVersion, dockerTabEnabled,
        dockerImages, dockerContainers, dockerLoading,
        dockerExecVisible, dockerExecData, dockerLogsVisible, dockerLogsData,
        dockerSubTab, dockerPullInput, dockerShowAll, selectedMirror, mirrorSelectMode,
        selectedImages, selectedContainers, batchActionLoading,
        checkDockerEnv, onDockerTabClick, loadDockerImages, loadDockerContainers,
        dockerPullImage, dockerRemoveImage, dockerContainerAction,
        dockerRemoveContainer, dockerExecCommand, dockerViewLogs,
        toggleImageSelection, toggleContainerSelection, batchDeleteImages, batchContainerAction,
    });
}
