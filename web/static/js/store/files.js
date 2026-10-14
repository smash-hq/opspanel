/**
 * files.js — 文件管理域
 *
 * 远程目录浏览、文件预览/详情/下载与权限指引弹窗。
 */

import { fetchFiles, fetchFileInfo, fetchFilePreview, downloadFile } from '../api/files.js';
import { fmtBytes } from '../utils.js';

const { ref, reactive, computed } = window.Vue;

export function setupFiles(ctx) {
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

    // 权限说明文档弹窗（权限不足时的指引入口）
    const permGuideVisible = ref(false);
    function openPermGuide() { permGuideVisible.value = true; }
    function closePermGuide() { permGuideVisible.value = false; }

    async function loadFiles(path) {
        if (!ctx.selectedServer.value) return; // 本机与远程节点均支持（本机走进程内执行）
        filesLoading.value = true;
        try {
            const targetPath = path || currentFilePath.value;
            const result = await fetchFiles(ctx.selectedServer.value, targetPath);
            files.value = result.files || [];
            fileSudoWarning.value = !result.sudo_used;
            if (path) currentFilePath.value = path;
        } catch (err) {
            ctx.toastNotify('文件列表加载失败: ' + err.message, true);
            fileSudoWarning.value = false;
        } finally {
            filesLoading.value = false;
        }
    }

    function navigateToPath(path) {
        loadFiles(path);
    }

    async function openFilePreview(file) {
        if (!ctx.selectedServer.value || file.is_dir) return;
        filePreviewVisible.value = true;
        filePreviewLoading.value = true;
        filePreviewData.path = file.path;
        filePreviewData.content = '';
        try {
            const data = await fetchFilePreview(ctx.selectedServer.value, file.path, 100);
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
        if (!ctx.selectedServer.value) return;
        fileDetailVisible.value = true;
        fileDetailData.name = file.name;
        fileDetailData.path = file.path;
        fileDetailData.is_dir = file.is_dir;
        fileDetailData.size = file.size;
        fileDetailData.mode = file.mode;
        fileDetailData.mod_time = file.mod_time;
        try {
            const info = await fetchFileInfo(ctx.selectedServer.value, file.path);
            Object.assign(fileDetailData, info);
        } catch (err) {
            // 使用已有数据
        }
    }

    function closeFileDetail() {
        fileDetailVisible.value = false;
    }

    async function handleDownloadFile(file) {
        if (!ctx.selectedServer.value) return;
        try {
            await downloadFile(ctx.selectedServer.value, file.path);
            ctx.toastNotify('下载已开始');
        } catch (err) {
            ctx.toastNotify('下载失败: ' + err.message, true);
        }
    }

    // 文件列表过滤
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

    /** Unix 时间戳 → 本地时间字符串（文件修改时间）。 */
    function formatModTime(ts) {
        if (!ts) return '—';
        return new Date(ts * 1000).toLocaleString('zh-CN', { hour12: false });
    }

    Object.assign(ctx, {
        files, currentFilePath, filesLoading, fileSearchQuery, fileSudoWarning,
        filePreviewVisible, filePreviewData, filePreviewLoading,
        fileDetailVisible, fileDetailData,
        permGuideVisible, openPermGuide, closePermGuide,
        loadFiles, navigateToPath, openFilePreview, closeFilePreview,
        openFileDetail, closeFileDetail, handleDownloadFile,
        filteredFiles, sortedEntries, getFileIcon, formatFileSize: fmtBytes, formatModTime,
    });
}
