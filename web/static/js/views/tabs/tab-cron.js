import { appCtx } from '../../store/index.js';

export const TabCron = {
    name: 'TabCron',
    setup: () => appCtx(),
    template: `
                <div class="node-tab-content">
                    <!-- 任务工具栏 -->
                    <div class="cron-toolbar">
                        <h3>{{ t('定时任务 ({p1})', { p1: cronJobs.length }) }}</h3>
                        <span class="cron-toolbar-hint">{{ t('任务保存在服务器 crontab 中，由系统 cron 执行') }}</span>
                        <span class="cron-toolbar-spacer"></span>
                        <button class="toolbar-button" @click="loadCronJobs()" :disabled="cronLoading">{{ t('↻ 刷新') }}</button>
                        <button class="primary-button" @click="openCronForm(null)">{{ t('＋ 新建任务') }}</button>
                    </div>
                    <div v-if="cronLoading" class="loading-state" style="padding:20px;">{{ t('加载中...') }}</div>
                    <div v-else-if="!cronJobs.length" class="list-empty">
                        <span class="empty-icon">⏱</span><strong>{{ t('服务器 crontab 中暂无定时任务') }}</strong>
                        <p>{{ t('点击右上角「新建任务」向 crontab 写入定时命令') }}</p>
                    </div>
                    <div v-else class="cron-list">
                        <div v-for="job in cronJobs" :key="job.id" class="cron-card" :class="{ disabled: !job.enabled }">
                            <div class="cron-card-main">
                                <div class="cron-card-head">
                                    <span class="cron-name" :title="job.name">{{ job.name || t('未命名任务') }}</span>
                                    <span class="cron-expr" :title="job.expr">{{ cronExprText(job) }}</span>
                                    <span v-if="!job.enabled" class="cron-badge off">{{ t('已停用') }}</span>
                                    <button class="cron-switch" :class="{ on: job.enabled }" @click="toggleCronJob(job)"
                                            :disabled="cronBusyId === job.id"
                                            :title="job.enabled ? t('点击停用（注释该行）') : t('点击启用')"><i></i></button>
                                </div>
                                <div class="cron-command" :title="job.command">$ {{ job.command }}</div>
                                <div class="cron-meta">
                                    <span class="cron-meta-item" :title="t('下次执行')">{{ t('⏭ 下次 {p1}', { p1: cronNextText(job) }) }}</span>
                                    <template v-if="job.next_runs && job.next_runs.length > 1">
                                        <span class="cron-meta-item" :title="t('后续执行')">{{ t('之后 {p1}', { p1: job.next_runs[1] }) }}</span>
                                    </template>
                                </div>
                            </div>
                            <div class="cron-card-actions">
                                <button class="proc-btn" @click="openCronForm(job)" :title="t('编辑')">✎</button>
                                <button class="proc-btn danger" @click="handleDeleteCronJob(job)" :title="t('删除')">✗</button>
                            </div>
                        </div>
                    </div>
                </div>
`,
};
