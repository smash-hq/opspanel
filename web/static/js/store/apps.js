/**
 * store/apps.js — 服务部署管理（java/python/go 应用部署、文件上传、生命周期、日志）
 */
import { detectLanguages } from '../api/languages.js';
import {
    fetchApps, createApp, updateApp, deleteApp,
    initAppUpload, fetchAppUploadStatus, uploadAppChunk, completeAppUpload,
    appAction, fetchAppLogs,
} from '../api/apps.js';

const { ref, reactive, computed } = window.Vue;

export function setupApps(ctx) {
    // ----- 列表 -----
    const apps = ref([]);
    const appsLoading = ref(false);
    const appBusyId = ref(''); // 正在执行行操作（启停等）的应用 ID
    const appBusyAction = ref(''); // 正在执行的操作：start / stop / restart

    // ─── 启动确认弹窗（结构化展示部署信息） ───
    const appStartVisible = ref(false);
    const appStartInfo = reactive({ name: '', deployDir: '', startCmd: '', logFile: '' });
    let appStartResolve = null;

    /** 打开启动确认弹窗，返回 Promise<boolean>。 */
    function requestAppStart(app) {
        const dir = (app.deploy_dir || '').replace(/\/+$/, '');
        const logFile = app.log_file || `${dir}/app.log`;
        Object.assign(appStartInfo, {
            name: app.name,
            deployDir: app.deploy_dir,
            startCmd: app.start_cmd,
            logFile,
            // 与后端 StartApp 实际拼接一致（示意）：nohup 后台 + 输出追加到日志 + 丢弃 stdin + 记录 PID
            effCmd: `cd ${app.deploy_dir} && nohup sh -c '${app.start_cmd}' >> ${logFile} 2>&1 < /dev/null &`,
        });
        appStartVisible.value = true;
        return new Promise(resolve => { appStartResolve = resolve; });
    }

    function okAppStart() {
        appStartVisible.value = false;
        if (appStartResolve) { appStartResolve(true); appStartResolve = null; }
    }

    function cancelAppStart() {
        appStartVisible.value = false;
        if (appStartResolve) { appStartResolve(false); appStartResolve = null; }
    }

    /** 部署统计：总数 / 运行中 / 启动失败 / 未启动。 */
    const appStats = computed(() => {
        const total = apps.value.length;
        const running = apps.value.filter(a => a.status === 'running').length;
        const failed = apps.value.filter(a => a.status === 'failed').length;
        return { total, running, failed, stopped: total - running - failed };
    });

    /** 仅当该应用正在执行指定操作时返回 true（用于只让对应按钮转圈）。 */
    function appActionBusy(app, action) {
        return appBusyId.value === app.id && appBusyAction.value === action;
    }

    // ----- 新建/编辑表单 -----
    const appFormVisible = ref(false);
    const appFormMode = ref('add'); // add | edit
    const appFormStep = ref('form'); // form 基本信息 | upload 上传产物
    const appForm = reactive({
        name: '',
        language: 'java',
        deploy_dir: '',
        start_cmd: '',
        log_file: '',
        remark: '',
    });
    const appFormError = ref('');
    const appSaving = ref(false);
    const editingAppId = ref('');

    // ----- 语言环境检测 -----
    const appLangChecking = ref(false);
    const appLangStatus = ref(null); // { installed: bool, version: string } | null

    // ----- 上传步骤 -----
    const createdApp = ref(null); // 上传步骤对应的应用记录
    const uploadDirMode = ref('files'); // files 选择文件 | dir 选择目录
    const pendingAppFiles = ref([]); // 待上传 [{ file, relPath }]
    const appUploadState = reactive({
        running: false,
        currentFileName: '',
        chunkDone: 0, chunkTotal: 0, // 当前文件分片进度
        bytesDone: 0, bytesTotal: 0, // 整体字节进度（进度条依据）
        skipped: 0, // 断点续传跳过的分片数
    });
    const appUploadFailed = ref(false);
    const appUploadPercent = computed(() => {
        const total = appUploadState.bytesTotal || 1;
        return Math.min(100, Math.round((appUploadState.bytesDone / total) * 100));
    });
    const uploadedFiles = ref([]); // 已上传 [{ path, size }]

    /** 文件选择框 accept：java 单文件模式限定 .jar。 */
    const appFileAccept = computed(() =>
        (appForm.language === 'java' && uploadDirMode.value === 'files') ? '.jar' : '');

    // ----- 日志弹窗 -----
    const appLogsVisible = ref(false);
    const appLogsData = reactive({ id: '', name: '', logs: '', logFile: '' });
    const appLogsLines = ref(200);

    function resetAppForm() {
        appForm.name = '';
        appForm.language = 'java';
        appForm.deploy_dir = '';
        appForm.start_cmd = '';
        appForm.log_file = '';
        appForm.remark = '';
        appFormError.value = '';
        appLangStatus.value = null;
        editingAppId.value = '';
    }

    async function loadApps(refresh) {
        appsLoading.value = true;
        try {
            apps.value = await fetchApps(ctx.selectedServer.value, refresh);
        } catch (err) {
            ctx.toastNotify(err.message || '加载服务列表失败', true);
        } finally {
            appsLoading.value = false;
        }
    }

    function onAppsTabClick() {
        if (ctx.activeTab.value === 'apps') return;
        ctx.activeTab.value = 'apps';
        loadApps(true);
    }

    function openAppForm() {
        if (!ctx.selectedServer.value) {
            ctx.toastNotify('请先在顶部选择目标服务器', true);
            return;
        }
        resetAppForm();
        appFormMode.value = 'add';
        appFormStep.value = 'form';
        createdApp.value = null;
        pendingAppFiles.value = [];
        uploadedFiles.value = [];
        resetUploadState();
        appFormVisible.value = true;
    }

    function openAppEditForm(app) {
        resetAppForm();
        appFormMode.value = 'edit';
        editingAppId.value = app.id;
        appForm.name = app.name || '';
        appForm.language = app.language || 'java';
        appForm.deploy_dir = app.deploy_dir || '';
        appForm.start_cmd = app.start_cmd || '';
        appForm.log_file = app.log_file || '';
        appForm.remark = app.remark || '';
        appFormStep.value = 'form';
        appFormVisible.value = true;
        checkAppLanguage();
    }

    /** 直接进入某应用的上传步骤（二次部署：补充上传产物）。 */
    function openAppUpload(app) {
        createdApp.value = app;
        appFormStep.value = 'upload';
        uploadDirMode.value = 'files';
        pendingAppFiles.value = [];
        uploadedFiles.value = appArtifacts(app);
        resetUploadState();
        appFormMode.value = 'edit';
        editingAppId.value = app.id;
        appFormVisible.value = true;
    }

    function closeAppForm() {
        if (appUploadState.running) {
            ctx.toastNotify('正在上传中，请等待完成', true);
            return;
        }
        appFormVisible.value = false;
        loadApps(false);
    }

    /** 检测目标服务器上所选语言的运行环境。 */
    async function checkAppLanguage() {
        const lang = appForm.language;
        if (!lang) return;
        appLangChecking.value = true;
        appLangStatus.value = null;
        try {
            const list = await detectLanguages(ctx.selectedServer.value);
            const hit = (list || []).find((l) => l.id === lang);
            if (hit) {
                appLangStatus.value = {
                    installed: hit.status === 'installed',
                    version: hit.version || '',
                };
            }
        } catch {
            // 检测失败静默处理，启动时后端会兜底校验
        } finally {
            appLangChecking.value = false;
        }
    }

    async function saveApp() {
        appFormError.value = '';
        const f = appForm;
        if (!f.name.trim()) return void (appFormError.value = '请填写服务名称');
        if (!f.deploy_dir.trim()) return void (appFormError.value = '请填写部署目录');
        if (!f.deploy_dir.trim().startsWith('/')) return void (appFormError.value = '部署目录必须为绝对路径（以 / 开头）');
        if (!f.start_cmd.trim()) return void (appFormError.value = '请填写启动命令');

        appSaving.value = true;
        try {
            if (appFormMode.value === 'add') {
                const app = await createApp({
                    server_id: ctx.selectedServer.value,
                    name: f.name.trim(),
                    language: f.language,
                    deploy_dir: f.deploy_dir.trim(),
                    start_cmd: f.start_cmd.trim(),
                    log_file: f.log_file.trim(),
                    remark: f.remark.trim(),
                });
                createdApp.value = app;
                appFormStep.value = 'upload';
                checkAppLanguage();
            } else {
                await updateApp(editingAppId.value, {
                    name: f.name.trim(),
                    language: f.language,
                    deploy_dir: f.deploy_dir.trim(),
                    start_cmd: f.start_cmd.trim(),
                    log_file: f.log_file.trim(),
                    remark: f.remark.trim(),
                });
                appFormVisible.value = false;
                ctx.toastNotify('保存成功');
                loadApps(false);
            }
        } catch (err) {
            appFormError.value = err.message || '保存失败';
        } finally {
            appSaving.value = false;
        }
    }

    /** 重置上传进度状态。 */
    function resetUploadState() {
        appUploadState.running = false;
        appUploadState.currentFileName = '';
        appUploadState.chunkDone = 0;
        appUploadState.chunkTotal = 0;
        appUploadState.bytesDone = 0;
        appUploadState.bytesTotal = 0;
        appUploadState.skipped = 0;
        appUploadFailed.value = false;
    }

    /** 触发隐藏的原生文件选择框（自定义按钮样式）。 */
    function pickAppFiles(mode) {
        const el = document.getElementById(mode === 'dir' ? 'app-dir-input' : 'app-file-input');
        if (el) el.click();
    }

    /** 文件/目录选择 input 的 change 处理：目录上传剥离顶层目录名；java 单文件模式仅接受 .jar。 */
    function onAppFileInputChange(e) {
        let files = Array.from(e.target.files || []);
        if (createdApp.value && createdApp.value.language === 'java' && uploadDirMode.value === 'files') {
            const jars = files.filter((f) => /\.jar$/i.test(f.name));
            const rejected = files.length - jars.length;
            if (rejected > 0) {
                ctx.toastNotify(`Java 服务仅支持上传 .jar 文件，已忽略 ${rejected} 个其他文件`, true);
            }
            files = jars;
        }
        pendingAppFiles.value = files.map((f) => {
            let relPath = f.name;
            if (f.webkitRelativePath) {
                const parts = f.webkitRelativePath.split('/');
                parts.shift(); // 剥离顶层目录名，内容平铺进部署目录
                relPath = parts.join('/') || f.name;
            }
            return { file: f, relPath };
        });
        e.target.value = '';
    }

    function clearPendingAppFiles() {
        pendingAppFiles.value = [];
    }

    /** 字节数格式化（进度展示用）。 */
    function formatAppSize(bytes) {
        if (!bytes && bytes !== 0) return '-';
        if (bytes < 1024) return `${bytes} B`;
        if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
        if (bytes < 1024 * 1024 * 1024) return `${(bytes / 1024 / 1024).toFixed(1)} MB`;
        return `${(bytes / 1024 / 1024 / 1024).toFixed(2)} GB`;
    }

    /**
     * 生成稳定上传会话 key（FNV 双 hash，16 位 hex）：
     * 同一应用同一文件（路径+大小+mtime）在失败重试、重开弹窗时 key 不变，
     * 后端据此复用已落盘分片，实现跨会话断点续传。
     */
    function makeUploadKey(app, item) {
        const seed = `${app.id}|${item.relPath}|${item.file.size}|${item.file.lastModified}`;
        let h1 = 0x811c9dc5;
        let h2 = 0x1b873593;
        for (let i = 0; i < seed.length; i++) {
            const c = seed.charCodeAt(i);
            h1 = Math.imul(h1 ^ c, 16777619) >>> 0;
            h2 = Math.imul(h2 ^ c, 2246822519) >>> 0;
        }
        return h1.toString(16).padStart(8, '0') + h2.toString(16).padStart(8, '0');
    }

    /**
     * 分片上传全部待传文件（断点续传）：
     * init 建会话 → status 查已落盘分片并跳过 → XHR 逐分片上传（进度回调）→ complete 合并。
     */
    async function uploadPendingAppFiles() {
        if (!pendingAppFiles.value.length || appUploadState.running) return;
        const app = createdApp.value;
        if (!app) return;

        appUploadState.running = true;
        appUploadState.currentFileName = '';
        appUploadState.chunkDone = 0;
        appUploadState.chunkTotal = 0;
        appUploadState.bytesDone = 0;
        appUploadState.bytesTotal = pendingAppFiles.value.reduce((s, it) => s + it.file.size, 0);
        appUploadState.skipped = 0;
        appUploadFailed.value = false;

        try {
            for (const item of pendingAppFiles.value) {
                appUploadState.currentFileName = item.relPath;
                // 1) 创建上传会话（稳定 key：重试/重开弹窗复用旧分片）
                const init = await initAppUpload(app.id, item.relPath, item.file.size, makeUploadKey(app, item));
                const chunkSize = init.chunk_size || 4 * 1024 * 1024;
                const totalChunks = Math.max(1, Math.ceil(item.file.size / chunkSize));
                // 2) 查询已落盘分片（断点续传：跳过已存在分片）
                let received = [];
                try {
                    const st = await fetchAppUploadStatus(app.id, init.key);
                    received = (st && st.received) || [];
                } catch { /* 查询失败按全新上传处理 */ }
                const receivedSet = new Set(received);
                if (receivedSet.size) {
                    appUploadState.skipped += receivedSet.size;
                    let skippedBytes = 0;
                    for (const idx of receivedSet) {
                        skippedBytes += Math.min(chunkSize, item.file.size - idx * chunkSize);
                    }
                    appUploadState.bytesDone += skippedBytes;
                }
                appUploadState.chunkTotal = totalChunks;
                appUploadState.chunkDone = receivedSet.size;
                // 3) 逐分片上传（覆盖写幂等，失败重试亦安全）
                for (let idx = 0; idx < totalChunks; idx++) {
                    if (receivedSet.has(idx)) continue;
                    const start = idx * chunkSize;
                    const blob = item.file.slice(start, Math.min(start + chunkSize, item.file.size));
                    const before = appUploadState.bytesDone;
                    await uploadAppChunk(app.id, init.key, idx, blob, (loaded) => {
                        appUploadState.bytesDone = before + loaded;
                    });
                    appUploadState.bytesDone = before + blob.size;
                    appUploadState.chunkDone += 1;
                }
                // 4) 校验并合并
                const res = await completeAppUpload(app.id, init.key, item.relPath, totalChunks, item.file.size);
                uploadedFiles.value.push(res || { path: item.relPath, size: item.file.size });
            }
            pendingAppFiles.value = [];
            ctx.toastNotify(appUploadState.skipped > 0
                ? `上传完成（断点续传，跳过已传 ${appUploadState.skipped} 个分片）`
                : `上传完成，共 ${uploadedFiles.value.length} 个文件`);
            loadApps(false);
        } catch (err) {
            appUploadFailed.value = true;
            ctx.toastNotify(`上传 ${appUploadState.currentFileName || ''} 失败：${err.message || '未知错误'}（已传分片保留，可重试续传）`, true);
        } finally {
            appUploadState.running = false;
        }
    }

    /** 启动服务（结构化确认弹窗 → 调接口 → 刷新列表）。 */
    async function appStart(app) {
        if (appBusyId.value) return;
        const ok = await requestAppStart(app);
        if (!ok) return;
        appBusyId.value = app.id;
        appBusyAction.value = 'start';
        try {
            const res = await appAction(app.id, 'start');
            ctx.toastNotify(`服务已启动（PID ${res && res.pid ? res.pid : '-'}）`);
            loadApps(true);
        } catch (err) {
            // 简短 toast + 完整原因进通知中心与状态徽标
            ctx.toastNotify((err.message || '启动失败').split('\n')[0], true);
            ctx.loadNotifications();
            loadApps(false);
        } finally {
            appBusyId.value = '';
            appBusyAction.value = '';
        }
    }

    async function appStop(app) {
        if (appBusyId.value) return;
        const ok = await ctx.confirmDialog(`确定停止服务「${app.name}」吗？`, {
            title: '停止服务',
            confirmText: '停止',
            danger: true,
        });
        if (!ok) return;
        appBusyId.value = app.id;
        appBusyAction.value = 'stop';
        try {
            await appAction(app.id, 'stop');
            ctx.toastNotify('服务已停止');
            loadApps(true);
        } catch (err) {
            ctx.toastNotify(err.message || '停止失败', true);
        } finally {
            appBusyId.value = '';
            appBusyAction.value = '';
        }
    }

    async function appRestart(app) {
        if (appBusyId.value) return;
        const ok = await ctx.confirmDialog(`确定重启服务「${app.name}」吗？`, {
            title: '重启服务',
            confirmText: '重启',
        });
        if (!ok) return;
        appBusyId.value = app.id;
        appBusyAction.value = 'restart';
        try {
            const res = await appAction(app.id, 'restart');
            ctx.toastNotify(`服务已重启（PID ${res && res.pid ? res.pid : '-'}）`);
            loadApps(true);
        } catch (err) {
            ctx.toastNotify((err.message || '重启失败').split('\n')[0], true);
            ctx.loadNotifications();
            loadApps(false);
        } finally {
            appBusyId.value = '';
            appBusyAction.value = '';
        }
    }

    async function appViewLogs(app) {
        appLogsData.id = app.id;
        appLogsData.name = app.name;
        appLogsData.logs = '';
        appLogsData.logFile = app.log_file || (app.deploy_dir ? `${app.deploy_dir.replace(/\/$/, '')}/app.log` : '');
        appLogsVisible.value = true;
        await refreshAppLogs();
    }

    async function refreshAppLogs() {
        if (!appLogsData.id) return;
        try {
            const res = await fetchAppLogs(appLogsData.id, appLogsLines.value);
            appLogsData.logs = (res && res.log) || '';
            if (res && res.log_file) appLogsData.logFile = res.log_file;
        } catch (err) {
            ctx.toastNotify(err.message || '获取日志失败', true);
        }
    }

    function closeAppLogs() {
        appLogsVisible.value = false;
        appLogsData.id = '';
        appLogsData.logs = '';
    }

    async function appDelete(app) {
        const ok = await ctx.confirmDialog(
            `确定删除服务「${app.name}」吗？运行中的进程会被停止，已上传文件保留在服务器上。`,
            { title: '删除服务', confirmText: '删除', danger: true },
        );
        if (!ok) return;
        try {
            await deleteApp(app.id);
            ctx.toastNotify('已删除');
            loadApps(false);
        } catch (err) {
            ctx.toastNotify(err.message || '删除失败', true);
        }
    }

    /** 解析应用已上传产物清单。 */
    function appArtifacts(app) {
        try {
            const arr = JSON.parse(app.artifacts || '[]');
            return Array.isArray(arr) ? arr : [];
        } catch {
            return [];
        }
    }

    /** 查看上次启动失败原因。 */
    function showAppError(app) {
        ctx.confirmDialog(app.last_error || '未知错误，可查看服务日志排查。', {
            title: `「${app.name}」启动失败原因`,
            confirmText: '知道了',
            danger: false,
        });
    }

    /** 状态徽标文案：运行中 / 未启动 / 启动失败 / 状态未知。 */
    function appStatusText(app) {
        if (app.status === 'running') return '● 运行中';
        if (app.status === 'failed') return '⚠ 启动失败';
        if (app.status === 'unknown') return '○ 状态未知';
        return '○ 未启动';
    }

    /** 状态徽标样式类。 */
    function appStatusClass(app) {
        if (app.status === 'running') return 'on';
        if (app.status === 'failed') return 'failed';
        if (app.status === 'unknown') return 'off unknown';
        return 'off';
    }

    Object.assign(ctx, {
        apps, appsLoading, appBusyId, appActionBusy, appStats,
        appStartVisible, appStartInfo, okAppStart, cancelAppStart,
        appFormVisible, appFormMode, appFormStep, appForm, appFormError, appSaving,
        appLangChecking, appLangStatus,
        createdApp, uploadDirMode, pendingAppFiles, appUploadState, appUploadFailed,
        appUploadPercent, appFileAccept, uploadedFiles,
        appLogsVisible, appLogsData, appLogsLines,
        loadApps, onAppsTabClick,
        openAppForm, openAppEditForm, openAppUpload, closeAppForm,
        checkAppLanguage, saveApp,
        pickAppFiles, onAppFileInputChange, clearPendingAppFiles,
        uploadPendingAppFiles, formatAppSize,
        appStart, appStop, appRestart, appViewLogs, refreshAppLogs, closeAppLogs,
        appDelete, appArtifacts,
        showAppError, appStatusText, appStatusClass,
    });
}
