import { appCtx } from '../../store/index.js';

export const TabApps = {
    name: 'TabApps',
    setup: () => appCtx(),
    template: `
                <div class="node-tab-content">
                    <div class="cron-toolbar">
                        <div class="app-stats">
                            <span class="app-stat-chip"><b>{{ appStats.total }}</b>{{ t('个部署') }}</span>
                            <span class="app-stat-chip on" :title="t('运行中')">{{ t('● 运行中') }}<b>{{ appStats.running }}</b></span>
                            <span class="app-stat-chip failed" :title="t('启动失败')">{{ t('⚠ 启动失败') }}<b>{{ appStats.failed }}</b></span>
                            <span class="app-stat-chip" :title="t('未启动')">{{ t('○ 未启动') }}<b>{{ appStats.stopped }}</b></span>
                        </div>
                        <span class="cron-toolbar-spacer"></span>
                        <button class="toolbar-button" @click="loadApps(true)" :disabled="appsLoading">{{ t('↻ 刷新') }}</button>
                        <button class="primary-button" @click="openAppForm()">{{ t('＋ 新建部署') }}</button>
                    </div>
                    <div v-if="appsLoading" class="loading-state" style="padding:20px;">{{ t('加载中...') }}</div>
                    <div v-else-if="!apps.length" class="list-empty">
                        <span class="empty-icon">🚀</span><strong>{{ t('尚未部署任何服务') }}</strong>
                        <p>{{ t('点击右上角「新建部署」上传 jar 包 / go 文件 / python 代码并配置启动命令') }}</p>
                    </div>
                    <div v-else class="cron-list">
                        <div v-for="app in apps" :key="app.id" class="cron-card">
                            <div class="cron-card-main">
                                <div class="cron-card-head">
                                    <span class="cron-name" :title="app.name">{{ app.name }}</span>
                                    <span class="app-lang-badge" :class="app.language">{{ app.language }}</span>
                                    <span class="app-status-badge" :class="[appStatusClass(app), { clickable: app.status === 'failed' }]"
                                          :title="app.status === 'failed' ? t('点击查看失败原因') : ''"
                                          @click="app.status === 'failed' && showAppError(app)">
                                        {{ appStatusText(app) }}
                                    </span>
                                    <span v-if="app.status === 'running' && app.pid > 0" class="cron-expr" :title="'PID ' + app.pid">PID {{ app.pid }}</span>
                                </div>
                                <div class="cron-command" :title="app.start_cmd">$ {{ app.start_cmd }}</div>
                                <div class="cron-meta">
                                    <span class="cron-meta-item" :title="t('部署目录')">📁 {{ app.deploy_dir }}</span>
                                    <span class="cron-meta-item" :title="t('日志文件')">📄 {{ app.log_file || (app.deploy_dir.replace(/\\/$/, '') + '/app.log') }}</span>
                                    <span class="cron-meta-item" :title="t('已上传产物')">{{ t('{p1} 个文件', { p1: appArtifacts(app).length }) }}</span>
                                    <span v-if="app.remark" class="cron-meta-item" :title="app.remark">✎ {{ app.remark }}</span>
                                </div>
                            </div>
                            <div class="cron-card-actions">
                                <button class="app-row-btn primary" v-if="app.status !== 'running'" @click="appStart(app)" :disabled="appBusyId === app.id">
                                    <span v-if="appActionBusy(app, 'start')" class="btn-spinner"></span>{{ appActionBusy(app, 'start') ? t('启动中') : t('启动') }}
                                </button>
                                <button class="app-row-btn warn" v-else @click="appStop(app)" :disabled="appBusyId === app.id">
                                    <span v-if="appActionBusy(app, 'stop')" class="btn-spinner"></span>{{ appActionBusy(app, 'stop') ? t('停止中') : t('停止') }}
                                </button>
                                <button class="app-row-btn" @click="appRestart(app)" :disabled="appBusyId === app.id">
                                    <span v-if="appActionBusy(app, 'restart')" class="btn-spinner"></span>{{ appActionBusy(app, 'restart') ? t('重启中') : t('重启') }}
                                </button>
                                <button class="app-row-btn" @click="appViewLogs(app)">{{ t('日志') }}</button>
                                <button class="app-row-btn" @click="openAppUpload(app)" :title="t('上传新产物覆盖部署')">{{ t('二次部署') }}</button>
                                <button class="app-row-btn" @click="openAppEditForm(app)">{{ t('编辑') }}</button>
                                <button class="app-row-btn danger" @click="appDelete(app)">{{ t('删除') }}</button>
                            </div>
                        </div>
                    </div>
                </div>
`,
};
