import { appCtx } from '../../store/index.js';

export const TabSystem = {
    name: 'TabSystem',
    setup: () => appCtx(),
    template: `
                <div class="node-tab-content">
                    <!-- 工具栏：刷新按钮与其他功能页统一固定在内容区右上角 -->
                    <div class="sw-tab-toolbar">
                        <span class="sync-time" v-if="lastUpdate">{{ t('最后更新 {p1}', { p1: lastUpdate }) }}</span>
                        <button class="toolbar-button" @click="refreshCurrent" :disabled="loading" :title="t('刷新数据')">↻ <span>{{ t('刷新') }}</span></button>
                    </div>
                    <!-- 系统指标卡片 -->
                    <div class="metric-grid" v-if="hasInfo">
                        <div class="metric-card">
                            <div class="metric-top"><span class="metric-label">{{ t('CPU 使用率') }}</span><span class="metric-symbol blue">CPU</span></div>
                            <div class="metric-value">{{ number(info.cpu && info.cpu.percent) }}<small>%</small></div>
                            <div class="metric-bar"><i :style="{ width: bounded(info.cpu && info.cpu.percent) + '%' }" class="blue-fill"></i></div>
                            <span class="metric-foot">{{ t('{p1} 核心 · {p2} 线程', { p1: info.cpu && info.cpu.cores || 0, p2: info.cpu && info.cpu.threads || 0 }) }}</span>
                        </div>
                        <div class="metric-card">
                            <div class="metric-top"><span class="metric-label">{{ t('内存使用') }}</span><span class="metric-symbol violet">RAM</span></div>
                            <div class="metric-value">{{ number(info.memory && info.memory.percent) }}<small>%</small></div>
                            <div class="metric-bar"><i :style="{ width: bounded(info.memory && info.memory.percent) + '%' }" class="violet-fill"></i></div>
                            <span class="metric-foot">{{ fmtBytes(info.memory && info.memory.used) }} / {{ fmtBytes(info.memory && info.memory.total) }}</span>
                        </div>
                        <div class="metric-card">
                            <div class="metric-top"><span class="metric-label">{{ t('磁盘使用') }}</span><span class="metric-symbol amber">DISK</span></div>
                            <div class="metric-value">{{ diskPercent }}<small>%</small></div>
                            <div class="metric-bar"><i :style="{ width: diskPercent + '%' }" class="amber-fill"></i></div>
                            <span class="metric-foot">{{ t('{p1} / {p2} · {p3} 个挂载点', { p1: diskUsed, p2: diskTotal, p3: diskCount }) }}</span>
                        </div>
                        <div class="metric-card">
                            <div class="metric-top"><span class="metric-label">{{ t('运行时间') }}</span><span class="metric-symbol green">UP</span></div>
                            <div class="metric-value metric-time">{{ fmtUptime(info.uptime) }}</div>
                            <div class="metric-status"><span class="status-dot ok"></span>{{ t('系统运行正常') }}</div>
                            <span class="metric-foot">{{ t('负载 {p1} · {p2} 个进程', { p1: loadValue, p2: info.processes || 0 }) }}</span>
                        </div>
                    </div>
                    <!-- 权限警告 -->
                    <div v-if="hasInfo && info.sudo_used" class="permission-warning">
                        <div class="warning-icon">⚠</div>
                        <div class="warning-content">
                            <strong>{{ t('权限不足 — 部分系统信息可能不完整') }}</strong>
                            <details class="warning-details">
                                <summary>{{ t('📖 如何配置 sudo 免密权限') }}</summary>
                                <div class="warning-docs">
                                    <p><strong>{{ t('方法一：使用 visudo（推荐）') }}</strong></p>
                                    <pre><code>sudo visudo -f /etc/sudoers.d/your_username</code></pre>
                                    <pre><code>your_username ALL=(ALL) NOPASSWD: ALL</code></pre>
                                    <p><strong>{{ t('方法二：直接编辑 /etc/sudoers') }}</strong></p>
                                    <pre><code>sudo visudo</code></pre>
                                    <pre><code>your_username ALL=(ALL) NOPASSWD: ALL</code></pre>
                                    <p class="warning-tip">{{ t('💡 配置完成后，建议重新连接服务器以获取完整信息。') }}</p>
                                </div>
                            </details>
                        </div>
                    </div>
                    <!-- 主机信息卡片 -->
                    <div class="info-cards" v-if="hasInfo">
                        <div class="info-card">
                            <div class="info-card-header">
                                <span class="info-card-icon">🖥️</span>
                                <h4>{{ t('主机信息') }}</h4>
                            </div>
                            <div class="info-card-body">
                                <div class="info-row">
                                    <span class="info-key">{{ t('主机名') }}</span>
                                    <span class="info-value">{{ info.host && info.host.hostname || '—' }}</span>
                                </div>
                                <div class="info-row">
                                    <span class="info-key">{{ t('操作系统') }}</span>
                                    <span class="info-value">{{ info.host && info.host.os || '—' }}</span>
                                </div>
                                <div class="info-row">
                                    <span class="info-key">{{ t('平台') }}</span>
                                    <span class="info-value">{{ info.host && info.host.platform || '—' }}</span>
                                </div>
                                <div class="info-row">
                                    <span class="info-key">{{ t('内核版本') }}</span>
                                    <span class="info-value">{{ info.host && info.host.kernel || '—' }}</span>
                                </div>
                                <div class="info-row">
                                    <span class="info-key">{{ t('架构') }}</span>
                                    <span class="info-value">{{ info.host && info.host.arch || '—' }}</span>
                                </div>
                                <div class="info-row">
                                    <span class="info-key">{{ t('语言环境') }}</span>
                                    <span class="info-value">{{ info.host && info.host.locale || '—' }}</span>
                                </div>
                                <div class="info-row">
                                    <span class="info-key">{{ t('CPU 型号') }}</span>
                                    <span class="info-value info-model">{{ info.cpu && info.cpu.model || '—' }}</span>
                                </div>
                            </div>
                        </div>
                        <div class="info-card">
                            <div class="info-card-header">
                                <span class="info-card-icon">💾</span>
                                <h4>{{ t('磁盘空间') }}</h4>
                            </div>
                            <div class="info-card-body" v-if="info.disks && info.disks.length">
                                <div v-for="(d, i) in info.disks" :key="i" class="disk-card-row">
                                    <div class="disk-card-header">
                                        <span class="disk-card-mount">{{ d.mount }}</span>
                                        <span class="disk-card-percent" :class="usageClass(d.percent)">{{ number(d.percent) }}%</span>
                                    </div>
                                    <div class="disk-card-bar">
                                        <i :style="{ width: bounded(d.percent) + '%' }" :class="usageClass(d.percent)"></i>
                                    </div>
                                    <div class="disk-card-size">
                                        <span>{{ fmtBytes(d.used) }} / {{ fmtBytes(d.total) }}</span>
                                        <small>{{ d.device }}</small>
                                    </div>
                                </div>
                            </div>
                            <div class="info-card-body" v-else>
                                <span class="text-muted">{{ t('暂无磁盘数据') }}</span>
                            </div>
                        </div>
                    </div>
                    <!-- 加载中 -->
                    <div v-else-if="loading" class="loading-state" style="padding:40px;">
                        <div class="loading-orbit"><span></span></div><h3>{{ t('正在连接 {p1}', { p1: currentServerName }) }}</h3>
                    </div>
                    <!-- 错误 -->
                    <div v-else-if="loadError" class="error-state" style="padding:40px;">
                        <div class="error-mark">!</div><h3>{{ t('无法连接服务器') }}</h3><p>{{ loadError }}</p>
                        <div class="error-actions"><button class="primary-button" @click="retryLoad">{{ t('重新连接') }}</button><button class="toolbar-button" @click="openEdit(currentServer)">{{ t('编辑配置') }}</button></div>
                    </div>
                </div>
`,
};
