import { appCtx } from '../../store/index.js';

export const TabServices = {
    name: 'TabServices',
    setup: () => appCtx(),
    template: `
                <div class="node-tab-content tab-scroll">
                    <div class="process-toolbar">
                        <div class="service-toolbar-left">
                            <h3>{{ t('systemd 服务') }}</h3>
                            <input class="service-search" v-model="serviceSearch" :placeholder="t('搜索服务名或描述...')" />
                            <div class="service-state-tabs">
                                <button v-for="st in serviceStateCounts" :key="st.label" class="state-tab"
                                    :class="{ active: serviceStateFilter === st.value, 'has-failed': st.value === 'failed' && st.count > 0 }"
                                    @click="serviceStateFilter = st.value">
                                    {{ st.label }}<span class="state-tab-count">{{ st.count }}</span>
                                </button>
                            </div>
                        </div>
                        <button class="toolbar-button" @click="loadServices()" :disabled="servicesLoading">{{ t('↻ 刷新') }}</button>
                    </div>
                    <div v-if="servicesError" class="list-empty">
                        <span class="empty-icon">⚠</span><strong>{{ servicesError }}</strong>
                    </div>
                    <div v-else-if="servicesLoading" class="loading-state" style="padding:20px;">{{ t('加载中...') }}</div>
                    <div v-else-if="!filteredServices.length" class="list-empty">
                        <span class="empty-icon">⌁</span><strong>{{ serviceSearch ? t('无匹配服务') : t('暂无服务数据') }}</strong>
                    </div>
                    <div v-else class="process-table">
                        <div class="process-header">
                            <span class="svc-col-name">{{ t('服务名') }}</span>
                            <span class="svc-col-desc">{{ t('描述') }}</span>
                            <span class="svc-col-state">{{ t('状态') }}</span>
                            <span class="svc-col-enabled">{{ t('开机自启') }}</span>
                            <span class="svc-col-actions">{{ t('操作') }}</span>
                        </div>
                        <div v-for="s in filteredServices" :key="s.name" class="process-row">
                            <span class="svc-col-name" :title="s.name">{{ s.name }}</span>
                            <span class="svc-col-desc" :title="s.description">{{ s.description || '—' }}</span>
                            <span class="svc-col-state">
                                <span class="svc-badge" :class="serviceStateClass(s)">{{ serviceStateText(s) }}</span>
                            </span>
                            <span class="svc-col-enabled">
                                <span class="svc-badge" :class="s.enabled === 'enabled' ? 'ok' : 'idle'">{{ s.enabled === 'enabled' ? t('已启用') : (s.enabled === 'disabled' ? t('未启用') : s.enabled) }}</span>
                            </span>
                            <span class="svc-col-actions">
                                <button class="svc-btn start" @click="handleServiceAction(s, 'start')" :disabled="serviceActionPending === s.name || s.active_state === 'active'">{{ t('启动') }}</button>
                                <button class="svc-btn danger" @click="handleServiceAction(s, 'stop')" :disabled="serviceActionPending === s.name || s.active_state !== 'active'">{{ t('停止') }}</button>
                                <button class="svc-btn restart" @click="handleServiceAction(s, 'restart')" :disabled="serviceActionPending === s.name">{{ t('重启') }}</button>
                                <button class="svc-btn edit" @click="openUnitEditor(s)">{{ t('编辑') }}</button>
                                <button class="svc-btn" @click="viewServiceLog(s)">{{ t('日志') }}</button>
                            </span>
                        </div>
                    </div>
                </div>
`,
};
