/**
 * docker.js — Docker 管理域
 *
 * 环境检测、镜像/容器管理、容器命令执行与日志、批量操作，
 * 以及应用镜像源时的选择状态。
 */
import { t } from '../i18n/index.js';


import {
    checkDocker, fetchDockerImages, pullDockerImage, removeDockerImage,
    runDockerContainer,
    fetchDockerContainers, dockerContainerAction as apiDockerContainerAction,
    removeDockerContainer, dockerExec, fetchDockerLogs,
    fetchDockerContainerFiles, fetchDockerFilePreview, downloadDockerFile,
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
    // 一键启动容器
    const dockerRunVisible = ref(false);
    const dockerRunLoading = ref(false);
    const dockerRunData = reactive({
        image: '', name: '', ports: '', volumes: '', envs: '',
        network: '', restart: 'no', cmd: '',
        privileged: false, autoRemove: false, tty: false,
    });

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
            if (dockerTabEnabled.value) ctx.toastNotify(t('检测到 Docker 环境，已启用管理'));
        });
    }

    async function loadDockerImages() {
        if (!ctx.selectedServer.value || dockerLoading.value) return;
        dockerLoading.value = true;
        try {
            dockerImages.value = await fetchDockerImages(ctx.selectedServer.value);
        } catch (err) {
            ctx.toastNotify(t('镜像列表加载失败: ') + err.message, true);
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
            ctx.toastNotify(t('容器列表加载失败: ') + err.message, true);
        } finally {
            dockerLoading.value = false;
        }
    }

    async function dockerPullImage(image) {
        if (!ctx.selectedServer.value) return;
        try {
            await pullDockerImage(ctx.selectedServer.value, image);
            ctx.toastNotify(t('正在拉取镜像: ') + image);
            setTimeout(() => loadDockerImages(), 5000);
        } catch (err) {
            ctx.toastNotify(t('拉取失败: ') + err.message, true);
        }
    }

    async function dockerRemoveImage(id) {
        if (!ctx.selectedServer.value) return;
        if (!await ctx.confirmDialog(t('确定删除镜像 {id} 吗？', { id: id }), { title: t('删除镜像') })) return;
        try {
            await removeDockerImage(ctx.selectedServer.value, id);
            ctx.toastNotify(t('镜像已删除'));
            loadDockerImages();
        } catch (err) {
            ctx.toastNotify(t('删除失败: ') + err.message, true);
        }
    }

    /** 打开一键启动容器表单（镜像管理行按钮）。 */
    function openRunForm(img) {
        dockerRunData.image = `${img.repository}:${img.tag}`;
        dockerRunData.name = '';
        dockerRunData.ports = '';
        dockerRunData.volumes = '';
        dockerRunData.envs = '';
        dockerRunData.network = '';
        dockerRunData.restart = 'no';
        dockerRunData.cmd = '';
        dockerRunData.privileged = false;
        dockerRunData.autoRemove = false;
        dockerRunData.tty = false;
        dockerRunVisible.value = true;
    }

    /** 提交一键启动容器：多行文本按行拆分为数组。 */
    async function dockerRunContainer() {
        if (!ctx.selectedServer.value) return;
        const toList = (s) => s.split('\n').map(v => v.trim()).filter(Boolean);
        dockerRunLoading.value = true;
        try {
            const data = await runDockerContainer(ctx.selectedServer.value, {
                image: dockerRunData.image,
                name: dockerRunData.name,
                ports: toList(dockerRunData.ports),
                volumes: toList(dockerRunData.volumes),
                envs: toList(dockerRunData.envs),
                network: dockerRunData.network,
                restart: dockerRunData.restart,
                cmd: dockerRunData.cmd,
                privileged: dockerRunData.privileged,
                auto_remove: dockerRunData.autoRemove,
                tty: dockerRunData.tty,
            });
            dockerRunVisible.value = false;
            ctx.toastNotify(t('容器已启动: {p1}', { p1: (data.container_id || '').substring(0, 12) }));
            dockerSubTab.value = 'containers';
            loadDockerContainers(true);
        } catch (err) {
            ctx.toastNotify(t('启动失败: ') + err.message, true);
        } finally {
            dockerRunLoading.value = false;
        }
    }

    async function dockerContainerAction(id, action) {
        if (!ctx.selectedServer.value) return;
        try {
            await apiDockerContainerAction(ctx.selectedServer.value, id, action);
            ctx.toastNotify(t('操作成功: ') + action);
            loadDockerContainers(true);
        } catch (err) {
            ctx.toastNotify(t('操作失败: ') + err.message, true);
        }
    }

    async function dockerRemoveContainer(id) {
        if (!ctx.selectedServer.value) return;
        if (!await ctx.confirmDialog(t('确定删除容器 {id} 吗？', { id: id }), { title: t('删除容器') })) return;
        try {
            await removeDockerContainer(ctx.selectedServer.value, id);
            ctx.toastNotify(t('容器已删除'));
            loadDockerContainers(true);
        } catch (err) {
            ctx.toastNotify(t('删除失败: ') + err.message, true);
        }
    }

    async function dockerExecCommand(containerId, command) {
        if (!ctx.selectedServer.value) return;
        dockerExecVisible.value = true;
        dockerExecData.containerId = containerId;
        dockerExecData.command = command;
        dockerExecData.output = t('执行中...');
        try {
            const data = await dockerExec(ctx.selectedServer.value, containerId, command);
            dockerExecData.output = data.output || t('无输出');
        } catch (err) {
            dockerExecData.output = t('执行失败: ') + err.message;
        }
    }

    async function dockerViewLogs(containerId) {
        if (!ctx.selectedServer.value) return;
        dockerLogsVisible.value = true;
        dockerLogsData.containerId = containerId;
        dockerLogsData.logs = t('加载中...');
        try {
            const data = await fetchDockerLogs(ctx.selectedServer.value, containerId, 100);
            dockerLogsData.logs = data.logs || t('暂无日志');
        } catch (err) {
            dockerLogsData.logs = t('获取日志失败: ') + err.message;
        }
    }

    // ─── 容器文件浏览 ───
    const dockerFilesVisible = ref(false);
    const dockerFilesLoading = ref(false);
    const dockerFilesData = reactive({ containerId: '', path: '/', files: [], sudoUsed: false });
    const dockerPreviewVisible = ref(false);
    const dockerPreviewLoading = ref(false);
    const dockerPreviewData = reactive({ containerId: '', path: '', content: '' });

    /** 打开容器文件浏览器（容器行「文件」按钮），从根目录开始。 */
    function openDockerFiles(containerId) {
        dockerFilesData.containerId = containerId;
        dockerFilesData.path = '/';
        dockerFilesData.files = [];
        dockerFilesData.sudoUsed = false;
        dockerFilesVisible.value = true;
        loadDockerFiles('/');
    }

    /** 加载容器内指定目录。 */
    async function loadDockerFiles(path) {
        if (!ctx.selectedServer.value || !dockerFilesData.containerId || dockerFilesLoading.value) return;
        dockerFilesLoading.value = true;
        try {
            const data = await fetchDockerContainerFiles(ctx.selectedServer.value, dockerFilesData.containerId, path);
            dockerFilesData.files = data.files || [];
            dockerFilesData.sudoUsed = data.sudo_used || false;
            dockerFilesData.path = path || '/';
        } catch (err) {
            ctx.toastNotify(t('加载容器文件失败: ') + err.message, true);
        } finally {
            dockerFilesLoading.value = false;
        }
    }

    /** 目录在前、名称排序（与宿主机文件管理一致）。 */
    function sortedDockerFiles() {
        return [...dockerFilesData.files].sort((a, b) => {
            if (a.is_dir !== b.is_dir) return a.is_dir ? -1 : 1;
            return a.name.localeCompare(b.name);
        });
    }

    /** 面包屑：把当前路径拆为可点击层级。 */
    function dockerFileCrumbs() {
        const p = dockerFilesData.path || '/';
        const crumbs = [{ name: '/', path: '/' }];
        if (p === '/') return crumbs;
        let acc = '';
        for (const seg of p.split('/').filter(Boolean)) {
            acc += '/' + seg;
            crumbs.push({ name: seg, path: acc });
        }
        return crumbs;
    }

    /** 进入目录（点击目录行/面包屑）。 */
    function dockerEnterDir(path) {
        loadDockerFiles(path.replace(/\/+$/, '') || '/');
    }

    /** 预览容器内文件（二进制文件后端拒绝，错误展示在弹窗内）。 */
    async function dockerPreviewFile(f) {
        if (!ctx.selectedServer.value) return;
        dockerPreviewVisible.value = true;
        dockerPreviewLoading.value = true;
        dockerPreviewData.containerId = dockerFilesData.containerId;
        dockerPreviewData.path = f.path;
        dockerPreviewData.content = t('加载中...');
        try {
            const data = await fetchDockerFilePreview(ctx.selectedServer.value, dockerFilesData.containerId, f.path);
            dockerPreviewData.content = data.content || t('空文件');
        } catch (err) {
            dockerPreviewData.content = t('预览失败: ') + err.message;
        } finally {
            dockerPreviewLoading.value = false;
        }
    }

    /** 下载容器内文件（目录自动 tar 打包）。 */
    async function dockerDownloadFile(f) {
        if (!ctx.selectedServer.value) return;
        try {
            await downloadDockerFile(ctx.selectedServer.value, dockerFilesData.containerId, f.path);
            ctx.toastNotify(t('已开始下载: ') + f.name);
        } catch (err) {
            ctx.toastNotify(t('下载失败: ') + err.message, true);
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
        if (!selectedImages.value.length) { ctx.toastNotify(t('请选择要删除的镜像'), true); return; }
        if (!await ctx.confirmDialog(t('确定删除选中的 {p1} 个镜像吗？', { p1: selectedImages.value.length }), { title: t('批量删除镜像'), confirmText: t('删除') })) return;
        batchActionLoading.value = true;
        try {
            const result = await batchDeleteDockerImages(ctx.selectedServer.value, selectedImages.value);
            ctx.toastNotify(t('已删除 {p1} 个镜像{p2}', { p1: result.deleted?.length || 0, p2: result.errors?.length ? ', ' + result.errors.length + t(' 个失败') : '' }));
            selectedImages.value = [];
            loadDockerImages();
        } catch (err) {
            ctx.toastNotify(t('批量删除失败: ') + err.message, true);
        } finally {
            batchActionLoading.value = false;
        }
    }

    async function batchContainerAction(action) {
        if (!selectedContainers.value.length) { ctx.toastNotify(t('请选择要操作的容器'), true); return; }
        const actionName = action === 'stop' ? t('停止') : action === 'start' ? t('启动') : action === 'restart' ? t('重启') : action === 'rm' ? t('删除') : action;
        if (!await ctx.confirmDialog(t('确定对选中的 {p1} 个容器执行「{actionName}」操作吗？', { p1: selectedContainers.value.length, actionName: actionName }), { title: t('批量{actionName}', { actionName: actionName }) })) return;
        batchActionLoading.value = true;
        try {
            const result = await batchDockerContainerAction(ctx.selectedServer.value, selectedContainers.value, action);
            ctx.toastNotify(t('已处理 {p1} 个容器{p2}', { p1: result.done?.length || 0, p2: result.errors?.length ? ', ' + result.errors.length + t(' 个失败') : '' }));
            selectedContainers.value = [];
            loadDockerContainers(true);
        } catch (err) {
            ctx.toastNotify(t('批量操作失败: ') + err.message, true);
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
        dockerRunVisible, dockerRunLoading, dockerRunData,
        dockerFilesVisible, dockerFilesLoading, dockerFilesData,
        dockerPreviewVisible, dockerPreviewLoading, dockerPreviewData,
        openDockerFiles, loadDockerFiles, sortedDockerFiles, dockerFileCrumbs,
        dockerEnterDir, dockerPreviewFile, dockerDownloadFile,
        openRunForm, dockerRunContainer,
        checkDockerEnv, onDockerTabClick, loadDockerImages, loadDockerContainers,
        dockerPullImage, dockerRemoveImage, dockerContainerAction,
        dockerRemoveContainer, dockerExecCommand, dockerViewLogs,
        toggleImageSelection, toggleContainerSelection, batchDeleteImages, batchContainerAction,
    });
}
