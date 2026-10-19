import { appCtx } from '../../store/index.js';

export const TabProcess = {
    name: 'TabProcess',
    setup: () => appCtx(),
    template: `
                <div class="node-tab-content tab-scroll">
                    <!-- 系统概览 -->
                    <div class="process-overview" v-if="hasInfo">
                        <div class="overview-card">
                            <span class="overview-label">{{ t('CPU 使用率') }}</span>
                            <span class="overview-value">{{ number(info.cpu && info.cpu.percent) }}%</span>
                            <span class="overview-foot">{{ t('{p1} 线程', { p1: info.cpu && info.cpu.threads || 0 }) }}</span>
                        </div>
                        <div class="overview-card">
                            <span class="overview-label">{{ t('内存使用') }}</span>
                            <span class="overview-value">{{ number(info.memory && info.memory.percent) }}%</span>
                            <span class="overview-foot">{{ fmtBytes(info.memory && info.memory.used) }} / {{ fmtBytes(info.memory && info.memory.total) }}</span>
                        </div>
                        <div class="overview-card">
                            <span class="overview-label">{{ t('系统负载') }}</span>
                            <span class="overview-value">{{ loadValue }}</span>
                            <span class="overview-foot">{{ t('{p1} 个进程', { p1: info.processes || 0 }) }}</span>
                        </div>
                    </div>
                    <!-- 进程列表 -->
                    <div class="process-toolbar">
                        <h3>{{ t('进程列表 ({p1})', { p1: processes.length }) }}</h3>
                        <button class="toolbar-button" @click="loadProcesses()" :disabled="processLoading">{{ t('↻ 刷新') }}</button>
                    </div>
                    <div v-if="processLoading" class="loading-state" style="padding:20px;">{{ t('加载中...') }}</div>
                    <div v-else-if="!processes.length" class="list-empty">
                        <span class="empty-icon">⌁</span><strong>{{ t('暂无进程数据') }}</strong>
                    </div>
                    <div v-else class="process-table">
                        <div class="process-header">
                            <span class="col-pid">PID</span>
                            <span class="col-user">{{ t('用户') }}</span>
                            <span class="col-cpu sortable" :class="{ active: procSortKey === 'cpu' }" @click="toggleProcSort('cpu')" :title="t('点击按 CPU 排序')">
                                CPU<span class="sort-arrows" :class="{ asc: procSortKey === 'cpu' && !procSortDesc, desc: procSortKey === 'cpu' && procSortDesc }"><i>▲</i><i>▼</i></span>
                            </span>
                            <span class="col-mem sortable" :class="{ active: procSortKey === 'mem' }" @click="toggleProcSort('mem')" :title="t('点击按内存排序')">
                                MEM<span class="sort-arrows" :class="{ asc: procSortKey === 'mem' && !procSortDesc, desc: procSortKey === 'mem' && procSortDesc }"><i>▲</i><i>▼</i></span>
                            </span>
                            <span class="col-cmd">{{ t('命令') }}</span>
                            <span class="col-actions">{{ t('操作') }}</span>
                        </div>
                        <div v-for="p in sortedProcesses" :key="p.pid" class="process-row">
                            <span class="col-pid">{{ p.pid }}</span>
                            <span class="col-user">{{ p.user }}</span>
                            <span class="col-cpu" :class="{ warn: parseFloat(p.cpu) > 50 }">{{ p.cpu }}%</span>
                            <span class="col-mem">{{ p.mem }}%</span>
                            <span class="col-cmd" :title="p.command">{{ p.command }}</span>
                            <span class="col-actions">
                                <button class="proc-btn" @click="viewProcessLog(p.pid)" :title="t('日志')">📋</button>
                                <button class="proc-btn danger" @click="handleKillProcess(p.pid, false)" :title="t('终止')">✗</button>
                                <button class="proc-btn danger" @click="handleKillProcess(p.pid, true)" :title="t('强制终止')">⚡</button>
                            </span>
                        </div>
                    </div>
                </div>
`,
};
