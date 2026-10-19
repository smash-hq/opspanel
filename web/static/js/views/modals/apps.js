import { appCtx } from '../../store/index.js';

export const AppStartModal = {
    name: 'AppStartModal',
    setup: () => appCtx(),
    template: `
    <div class="modal-overlay" @click.self="cancelAppStart">
        <div class="modal app-start-modal">
            <div class="modal-header">
                <div>
                    <span class="eyebrow">START SERVICE</span>
                    <h3>{{ t('启动「{p1}」', { p1: appStartInfo.name }) }}</h3>
                </div>
                <button class="icon-button" @click="cancelAppStart" :title="t('关闭')">×</button>
            </div>
            <div class="modal-body">
                <div class="app-start-rows">
                    <div class="app-start-row">
                        <span class="app-start-label">{{ t('📁 部署目录') }}</span>
                        <code class="app-start-value">{{ appStartInfo.deployDir }}</code>
                    </div>
                    <div class="app-start-row">
                        <span class="app-start-label">{{ t('🚀 启动命令') }}</span>
                        <code class="app-start-value">{{ appStartInfo.startCmd }}</code>
                    </div>
                    <div class="app-start-row">
                        <span class="app-start-label">{{ t('📄 日志文件') }}</span>
                        <code class="app-start-value">{{ appStartInfo.logFile }}</code>
                    </div>
                    <div class="app-start-row">
                        <span class="app-start-label">{{ t('⚙ 实际执行命令') }}</span>
                        <code class="app-start-value dim">{{ appStartInfo.effCmd }}</code>
                    </div>
                </div>
                <p class="app-start-hint">
                    <b>{{ t('启动命令里无需再写 nohup、重定向或 &') }}</b>{{ t('，否则会影响 PID 记录与状态检测。') }}
                </p>
            </div>
            <div class="modal-footer">
                <button class="toolbar-button" @click="cancelAppStart">{{ t('取消') }}</button>
                <button class="primary-button" @click="okAppStart">{{ t('▶ 启动') }}</button>
            </div>
        </div>
    </div>
`,
};

export const AppFormModal = {
    name: 'AppFormModal',
    setup: () => appCtx(),
    template: `
    <div class="modal-overlay" @click.self="closeAppForm">
        <div class="modal file-preview-modal">
            <div class="modal-header">
                <div>
                    <span class="eyebrow">APP DEPLOY</span>
                    <h3 v-if="appFormStep === 'form'">{{ appFormMode === 'add' ? t('新建服务部署') : t('编辑服务部署') }}</h3>
                    <h3 v-else>{{ t('上传部署产物 · {p1}', { p1: createdApp ? createdApp.name : '' }) }}</h3>
                </div>
                <button class="icon-button" @click="closeAppForm" :title="t('关闭')">×</button>
            </div>
            <div class="modal-body">
                <!-- 步骤一：基本信息 -->
                <template v-if="appFormStep === 'form'">
                    <div class="form-grid">
                        <label class="field"><span>{{ t('服务名称 *') }}</span><input v-model="appForm.name" placeholder="my-app"></label>
                        <label class="field"><span>{{ t('部署语言 *') }}</span>
                            <select v-model="appForm.language" @change="checkAppLanguage()">
                                <option value="java">{{ t('Java（jar 包）') }}</option>
                                <option value="python">{{ t('Python（代码目录）') }}</option>
                                <option value="go">{{ t('Go（二进制 / go run）') }}</option>
                            </select>
                        </label>
                        <label class="field full"><span>{{ t('部署目录（绝对路径）*') }}</span><input v-model="appForm.deploy_dir" placeholder="/opt/my-app"></label>
                        <label class="field full"><span>{{ t('启动命令 *') }}</span><input v-model="appForm.start_cmd" placeholder="java -jar app.jar --server.port=8080"><small class="field-hint">{{ t('只填核心命令即可，无需 nohup、日志重定向或结尾的 &：系统会自动以 nohup 后台运行，并将输出追加写入下方日志文件') }}</small></label>
                        <label class="field full"><span>{{ t('日志文件（可选，默认 部署目录/app.log）') }}</span><input v-model="appForm.log_file" placeholder="/opt/my-app/logs/stdout.log"></label>
                        <label class="field full"><span>{{ t('备注（可选）') }}</span><input v-model="appForm.remark" :placeholder="t('订单服务 · 8080 端口')"></label>
                    </div>
                    <!-- 语言环境检测 -->
                    <div class="app-lang-check" v-if="appLangChecking">{{ t('⟳ 正在检测服务器 {p1} 运行环境...', { p1: appForm.language }) }}</div>
                    <div class="app-lang-check ok" v-else-if="appLangStatus && appLangStatus.installed">{{ t('✓ 已检测到 {p1} 环境{p2}，可正常启动', { p1: appForm.language, p2: appLangStatus.version ? '（' + appLangStatus.version + '）' : '' }) }}</div>
                    <div class="app-lang-check error" v-else-if="appLangStatus && !appLangStatus.installed">
                        <span>{{ t('⚠ 服务器缺少 {p1} 运行环境，请先安装后再启动服务', { p1: appForm.language }) }}</span>
                        <button class="toolbar-button" style="margin-left:10px;" @click="appFormVisible = false; activeTab = 'languages'; detectServerLanguages()">{{ t('去语言环境安装') }}</button>
                    </div>
                    <div class="form-notice" style="margin-top:10px;">{{ t('服务通过 nohup 后台拉起并记录 PID；停止时先 TERM 后 KILL。日志文件用于页面内 tail 查看。') }}</div>
                    <div v-if="appFormError" class="app-lang-check error">{{ appFormError }}</div>
                    <div style="margin-top: 14px; display:flex; justify-content:flex-end; gap:10px;">
                        <button class="toolbar-button" @click="closeAppForm">{{ t('取消') }}</button>
                        <button class="primary-button small" @click="saveApp()" :disabled="appSaving">{{ appSaving ? t('保存中...') : (appFormMode === 'add' ? t('保存并上传产物 →') : t('保存')) }}</button>
                    </div>
                </template>
                <!-- 步骤二：上传产物 -->
                <template v-else>
                    <div class="form-notice" v-if="createdApp">
                        {{ t('部署目录：') }}<strong>{{ createdApp.deploy_dir }}</strong>　{{ t('启动命令：') }}<strong>{{ createdApp.start_cmd }}</strong>
                    </div>
                    <!-- 隐藏原生 input，自定义美化按钮触发 -->
                    <input id="app-file-input" type="file" multiple hidden :accept="appFileAccept" @change="onAppFileInputChange">
                    <input id="app-dir-input" type="file" webkitdirectory hidden @change="onAppFileInputChange">
                    <div class="app-upload-picker">
                        <label class="show-all-label"><input type="radio" value="files" v-model="uploadDirMode"> {{ t('选择文件') }}</label>
                        <label class="show-all-label"><input type="radio" value="dir" v-model="uploadDirMode"> {{ t('选择目录（python/go 源码）') }}</label>
                        <span class="cron-toolbar-spacer" style="flex:1;"></span>
                        <button type="button" class="app-pick-btn" @click="pickAppFiles(uploadDirMode)">
                            📁 {{ uploadDirMode === 'files' ? t('选择文件') : t('选择目录') }}
                        </button>
                        <span v-if="appFileAccept" class="app-pick-hint">{{ t('仅支持 {p1}', { p1: appFileAccept }) }}</span>
                    </div>
                    <div v-if="pendingAppFiles.length" class="app-upload-pending">
                        <div class="app-upload-head">
                            <span>{{ t('待上传 {p1} 个文件', { p1: pendingAppFiles.length }) }}</span>
                            <button class="toolbar-button" @click="clearPendingAppFiles()" :disabled="appUploadState.running">{{ t('清空') }}</button>
                        </div>
                        <div class="app-upload-list">
                            <div v-for="item in pendingAppFiles" :key="item.relPath" class="app-upload-item">📄 {{ item.relPath }}<span class="app-upload-size">{{ formatAppSize(item.file.size) }}</span></div>
                        </div>
                    </div>
                    <!-- 上传进度条（分片 + 断点续传） -->
                    <div v-if="appUploadState.running || appUploadFailed" class="app-upload-progress" :class="{ error: appUploadFailed && !appUploadState.running }">
                        <div class="app-upload-progress-head">
                            <span class="app-upload-file">{{ appUploadState.running ? t('⟳ 正在上传：') : t('✗ 上传中断：') }}{{ appUploadState.currentFileName }}</span>
                            <span class="app-upload-pct">{{ appUploadPercent }}%</span>
                        </div>
                        <div class="app-upload-bar"><div class="app-upload-bar-fill" :style="{ width: appUploadPercent + '%' }"></div></div>
                        <div class="app-upload-progress-meta">
                            {{ t('分片 {p1}/{p2}\\n                            · {p3} / {p4}', { p1: appUploadState.chunkDone, p2: appUploadState.chunkTotal, p3: formatAppSize(appUploadState.bytesDone), p4: formatAppSize(appUploadState.bytesTotal) }) }}
                            <template v-if="appUploadState.skipped > 0"> {{ t('· 断点续传，已跳过 {p1} 个分片', { p1: appUploadState.skipped }) }}</template>
                        </div>
                    </div>
                    <div v-else-if="uploadedFiles.length" class="app-upload-pending">
                        <div class="app-upload-head"><span>{{ t('已上传 {p1} 个文件', { p1: uploadedFiles.length }) }}</span></div>
                        <div class="app-upload-list">
                            <div v-for="f in uploadedFiles" :key="f.path" class="app-upload-item done">✓ {{ f.path }}<span class="app-upload-size">{{ formatAppSize(f.size) }}</span></div>
                        </div>
                    </div>
                    <div class="form-notice" style="margin-top:10px;">{{ t('文件将写入部署目录（相对路径平铺）。大文件分片上传、支持断点续传；上传完成后在列表页启动服务。') }}</div>
                    <div style="margin-top: 14px; display:flex; justify-content:flex-end; gap:10px;">
                        <button class="toolbar-button" @click="closeAppForm" :disabled="appUploadState.running">{{ t('完成') }}</button>
                        <button class="primary-button small" @click="uploadPendingAppFiles()" :disabled="!pendingAppFiles.length || appUploadState.running">
                            {{ appUploadState.running ? t('上传中 ') + appUploadPercent + '%' : (appUploadFailed ? t('⇧ 继续上传（断点续传）') : t('⇧ 开始上传')) }}
                        </button>
                    </div>
                </template>
            </div>
        </div>
    </div>
`,
};

export const AppLogsModal = {
    name: 'AppLogsModal',
    setup: () => appCtx(),
    template: `
    <div class="modal-overlay" @click.self="closeAppLogs">
        <div class="modal log-modal">
            <div class="modal-header">
                <div><span class="eyebrow">APP LOG</span><h3>{{ t('{p1} · 服务日志', { p1: appLogsData.name }) }}</h3></div>
                <button class="icon-button" @click="closeAppLogs" :title="t('关闭')">×</button>
            </div>
            <div class="modal-body">
                <div class="log-toolbar">
                    <span class="log-file-path" :title="appLogsData.logFile">{{ t('日志文件：{p1}', { p1: appLogsData.logFile }) }}</span>
                    <span class="log-toolbar-spacer"></span>
                    <select v-model.number="appLogsLines" @change="refreshAppLogs()" class="log-lines-select" :title="t('日志行数')">
                        <option :value="100">{{ t('末尾 100 行') }}</option>
                        <option :value="200">{{ t('末尾 200 行') }}</option>
                        <option :value="500">{{ t('末尾 500 行') }}</option>
                        <option :value="1000">{{ t('末尾 1000 行') }}</option>
                        <option :value="5000">{{ t('末尾 5000 行') }}</option>
                    </select>
                    <button class="toolbar-button" @click="refreshAppLogs()">{{ t('↻ 刷新') }}</button>
                </div>
                <div class="log-viewer"><pre>{{ appLogsData.logs || t('（暂无日志）') }}</pre></div>
            </div>
        </div>
    </div>
`,
};

export const ProcessLogModal = {
    name: 'ProcessLogModal',
    setup: () => appCtx(),
    template: `
    <div class="modal-overlay" @click.self="closeProcessLog">
        <div class="modal log-modal">
            <div class="modal-header">
                <div><span class="eyebrow">PROCESS LOG</span><h3>{{ t('PID {p1} · 进程日志', { p1: processLogData.pid }) }}</h3></div>
                <button class="icon-button" @click="closeProcessLog" :title="t('关闭')">×</button>
            </div>
            <div class="modal-body">
                <div class="log-viewer" v-if="!processLogLoading">
                    <pre>{{ processLogData.content || t('暂无日志') }}</pre>
                </div>
                <div class="log-loading" v-else>{{ t('加载中...') }}</div>
            </div>
        </div>
    </div>
`,
};

export const ServiceLogModal = {
    name: 'ServiceLogModal',
    setup: () => appCtx(),
    template: `
    <div class="modal-overlay" @click.self="closeServiceLog">
        <div class="modal log-modal">
            <div class="modal-header">
                <div><span class="eyebrow">SYSTEMD LOG</span><h3>{{ t('{p1} · 服务日志', { p1: serviceLogData.name }) }}</h3></div>
                <button class="icon-button" @click="closeServiceLog" :title="t('关闭')">×</button>
            </div>
            <div class="modal-body">
                <div class="log-toolbar">
                    <span class="log-file-path">journalctl -u {{ serviceLogData.name }}</span>
                    <span class="log-toolbar-spacer"></span>
                    <select v-model.number="serviceLogLines" @change="refreshServiceLog()" class="log-lines-select" :title="t('日志行数')">
                        <option :value="50">{{ t('末尾 50 行') }}</option>
                        <option :value="100">{{ t('末尾 100 行') }}</option>
                        <option :value="200">{{ t('末尾 200 行') }}</option>
                        <option :value="500">{{ t('末尾 500 行') }}</option>
                        <option :value="1000">{{ t('末尾 1000 行') }}</option>
                    </select>
                    <button class="toolbar-button" @click="refreshServiceLog()">{{ t('↻ 刷新') }}</button>
                </div>
                <div class="log-viewer" v-if="!serviceLogLoading">
                    <pre>{{ serviceLogData.content || t('暂无日志') }}</pre>
                </div>
                <div class="log-loading" v-else>{{ t('加载中...') }}</div>
            </div>
        </div>
    </div>
`,
};
