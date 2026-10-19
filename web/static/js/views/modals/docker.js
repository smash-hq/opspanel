import { appCtx } from '../../store/index.js';

export const DockerExecModal = {
    name: 'DockerExecModal',
    setup: () => appCtx(),
    template: `
    <div class="modal-overlay" @click.self="dockerExecVisible = false">
        <div class="modal file-preview-modal">
            <div class="modal-header">
                <div><span class="eyebrow">DOCKER EXEC</span><h3>{{ t('{p1} · 执行命令', { p1: dockerExecData.containerId.substring(0, 12) }) }}</h3></div>
                <button class="icon-button" @click="dockerExecVisible = false" :title="t('关闭')">×</button>
            </div>
            <div class="modal-body">
                <div class="form-grid">
                    <label class="field full"><span>{{ t('命令') }}</span><input v-model="dockerExecData.command" :placeholder="t('输入命令...')" @keyup.enter="dockerExecCommand(dockerExecData.containerId, dockerExecData.command)"></label>
                </div>
                <div class="log-viewer" style="margin-top: 12px;">
                    <pre>{{ dockerExecData.output || t('等待执行...') }}</pre>
                </div>
            </div>
        </div>
    </div>
`,
};

export const DockerTermModal = {
    name: 'DockerTermModal',
    setup: () => appCtx(),
    template: `
    <div class="modal-overlay" @click.self="closeContainerTerminal">
        <div class="modal docker-terminal-modal">
            <div class="modal-header">
                <div><span class="eyebrow">DOCKER EXEC -IT</span><h3>{{ t('{p1} · 容器终端', { p1: dockerTermContainerId.substring(0, 12) }) }}</h3></div>
                <div class="docker-term-head-actions">
                    <span class="terminal-status" :class="dockerTermStatus"><i></i>{{ dockerTermStatusText }}</span>
                    <button class="icon-button" @click="closeContainerTerminal" :title="t('关闭（断开会话）')">×</button>
                </div>
            </div>
            <div class="docker-term-wrap" ref="dockerTermMount"></div>
        </div>
    </div>
`,
};

export const DockerLogsModal = {
    name: 'DockerLogsModal',
    setup: () => appCtx(),
    template: `
    <div class="modal-overlay" @click.self="dockerLogsVisible = false">
        <div class="modal file-preview-modal">
            <div class="modal-header">
                <div><span class="eyebrow">CONTAINER LOGS</span><h3>{{ t('{p1} · 日志', { p1: dockerLogsData.containerId.substring(0, 12) }) }}</h3></div>
                <button class="icon-button" @click="dockerLogsVisible = false" :title="t('关闭')">×</button>
            </div>
            <div class="modal-body">
                <div class="log-viewer">
                    <pre>{{ dockerLogsData.logs || t('暂无日志') }}</pre>
                </div>
            </div>
        </div>
    </div>
`,
};

export const DockerFilesModal = {
    name: 'DockerFilesModal',
    setup: () => appCtx(),
    template: `
    <div class="modal-overlay" @click.self="dockerFilesVisible = false">
        <div class="modal file-preview-modal">
            <div class="modal-header">
                <div><span class="eyebrow">CONTAINER FILES</span><h3>{{ t('{p1} · 文件浏览', { p1: dockerFilesData.containerId.substring(0, 12) }) }}</h3></div>
                <button class="icon-button" @click="dockerFilesVisible = false" :title="t('关闭')">×</button>
            </div>
            <div class="modal-body">
                <!-- 面包屑导航 -->
                <div class="file-breadcrumb">
                    <button class="bc-home" @click="dockerEnterDir('/')">/</button>
                    <template v-for="crumb in dockerFileCrumbs().slice(1)" :key="crumb.path">
                        <span class="bc-sep">/</span>
                        <button class="bc-segment" @click="dockerEnterDir(crumb.path)">{{ crumb.name }}</button>
                    </template>
                </div>
                <!-- 工具栏 -->
                <div class="file-toolbar">
                    <span v-if="dockerFilesData.sudoUsed" style="font-size: 0.72rem; opacity: 0.7;" :title="t('容器内普通用户无权限，已自动以 root 读取')">{{ t('已用容器内 root 读取') }}</span>
                    <span class="cron-toolbar-spacer"></span>
                    <button class="toolbar-button" @click="loadDockerFiles(dockerFilesData.path)" :disabled="dockerFilesLoading">{{ t('↻ 刷新') }}</button>
                </div>
                <!-- 文件列表（目录在前、名称排序） -->
                <div v-if="dockerFilesLoading" class="loading-state" style="padding:20px;">{{ t('加载中...') }}</div>
                <div v-else-if="!sortedDockerFiles().length" class="list-empty">
                    <span class="empty-icon">⌁</span><strong>{{ t('空目录') }}</strong>
                </div>
                <div v-else class="file-table" style="max-height: 55vh; overflow-y: auto;">
                    <div class="file-table-header">
                        <span class="ft-name">{{ t('名称') }}</span>
                        <span class="ft-size">{{ t('大小') }}</span>
                        <span class="ft-mode">{{ t('权限') }}</span>
                        <span class="ft-time">{{ t('修改时间') }}</span>
                        <span class="ft-actions">{{ t('操作') }}</span>
                    </div>
                    <div v-for="f in sortedDockerFiles()" :key="f.path" class="file-table-row" :class="{ 'is-dir-row': f.is_dir }">
                        <span class="ft-name">
                            <span class="ft-icon">{{ f.is_dir ? '📁' : (f.link_target ? '🔗' : getFileIcon(f.name)) }}</span>
                            <span class="ft-link" :class="{ 'ft-link-dir': f.is_dir }" @click="f.is_dir ? dockerEnterDir(f.path) : dockerPreviewFile(f)">{{ f.name }}</span>
                            <span class="ft-link-target" v-if="f.link_target" :title="t('链接目标: {p1}', { p1: f.link_target })">→ {{ f.link_target }}</span>
                        </span>
                        <span class="ft-size">{{ f.is_dir ? '—' : formatFileSize(f.size) }}</span>
                        <span class="ft-mode">{{ f.mode }}</span>
                        <span class="ft-time">{{ formatModTime(f.mod_time) }}</span>
                        <span class="ft-actions">
                            <button class="ft-btn" v-if="!f.is_dir" @click="dockerPreviewFile(f)" :title="t('预览')">👁</button>
                            <button class="ft-btn" @click="dockerDownloadFile(f)" :title="f.is_dir ? t('下载（tar 打包）') : t('下载')">⬇</button>
                        </span>
                    </div>
                </div>
            </div>
        </div>
    </div>
`,
};

export const DockerPreviewModal = {
    name: 'DockerPreviewModal',
    setup: () => appCtx(),
    template: `
    <div class="modal-overlay" @click.self="dockerPreviewVisible = false">
        <div class="modal file-preview-modal">
            <div class="modal-header">
                <div><span class="eyebrow">CONTAINER FILE</span><h3>{{ dockerPreviewData.path }}</h3></div>
                <button class="icon-button" @click="dockerPreviewVisible = false" :title="t('关闭')">×</button>
            </div>
            <div class="modal-body">
                <div class="file-toolbar">
                    <span style="font-size: 0.72rem; opacity: 0.7;">{{ t('最多预览 512KB，二进制文件请下载查看') }}</span>
                    <span class="cron-toolbar-spacer"></span>
                    <button class="toolbar-button" @click="dockerDownloadFile({ path: dockerPreviewData.path, name: dockerPreviewData.path.split('/').pop() })">{{ t('⬇ 下载') }}</button>
                </div>
                <div class="log-viewer" v-if="!dockerPreviewLoading" style="margin-top: 8px;">
                    <pre>{{ dockerPreviewData.content }}</pre>
                </div>
                <div class="log-loading" v-else>{{ t('加载中...') }}</div>
            </div>
        </div>
    </div>
`,
};

export const DockerRunModal = {
    name: 'DockerRunModal',
    setup: () => appCtx(),
    template: `
    <div class="modal-overlay" @click.self="dockerRunVisible = false">
        <div class="modal file-preview-modal">
            <div class="modal-header">
                <div><span class="eyebrow">RUN CONTAINER</span><h3>{{ t('{p1} · 启动容器', { p1: dockerRunData.image }) }}</h3></div>
                <button class="icon-button" @click="dockerRunVisible = false" :title="t('关闭')">×</button>
            </div>
            <div class="modal-body">
                <div class="form-grid">
                    <label class="field"><span>{{ t('镜像') }}</span><input v-model="dockerRunData.image" placeholder="nginx:latest"></label>
                    <label class="field"><span>{{ t('容器名（可选）') }}</span><input v-model="dockerRunData.name" placeholder="my-nginx"></label>
                    <label class="field full"><span>{{ t('端口映射（每行一条，格式：宿主机端口:容器端口）') }}</span><textarea v-model="dockerRunData.ports" rows="2" placeholder="8080:80"></textarea></label>
                    <label class="field full"><span>{{ t('卷挂载（每行一条，格式：宿主机路径:容器路径）') }}</span><textarea v-model="dockerRunData.volumes" rows="2" placeholder="/data/nginx:/usr/share/nginx/html"></textarea></label>
                    <label class="field full"><span>{{ t('环境变量（每行一条，格式：KEY=VALUE）') }}</span><textarea v-model="dockerRunData.envs" rows="2" placeholder="TZ=Asia/Shanghai"></textarea></label>
                    <label class="field"><span>{{ t('网络模式') }}</span><select v-model="dockerRunData.network"><option value="">{{ t('默认 (bridge)') }}</option><option value="host">host</option><option value="none">none</option></select></label>
                    <label class="field"><span>{{ t('重启策略') }}</span><select v-model="dockerRunData.restart"><option value="no">{{ t('不重启') }}</option><option value="always">always</option><option value="on-failure">on-failure</option><option value="unless-stopped">unless-stopped</option></select></label>
                    <label class="field full"><span>{{ t('启动命令（可选，覆盖镜像默认 CMD）') }}</span><input v-model="dockerRunData.cmd" placeholder="nginx -g 'daemon off;'"></label>
                    <div class="field full docker-run-checks">
                        <label class="show-all-label"><input type="checkbox" v-model="dockerRunData.privileged"> {{ t('特权模式') }}</label>
                        <label class="show-all-label"><input type="checkbox" v-model="dockerRunData.autoRemove"> {{ t('退出后自动删除') }}</label>
                        <label class="show-all-label"><input type="checkbox" v-model="dockerRunData.tty"> {{ t('分配 TTY') }}</label>
                    </div>
                </div>
                <div style="margin-top: 14px; display:flex; justify-content:flex-end; gap:10px;">
                    <button class="toolbar-button" @click="dockerRunVisible = false">{{ t('取消') }}</button>
                    <button class="primary-button small" @click="dockerRunContainer()" :disabled="dockerRunLoading">{{ dockerRunLoading ? t('启动中...') : t('▶ 一键启动') }}</button>
                </div>
            </div>
        </div>
    </div>
`,
};
