import { appCtx } from '../../store/index.js';

export const TabSoftware = {
    name: 'TabSoftware',
    setup: () => appCtx(),
    template: `
                <div class="node-tab-content">
                    <div class="sw-tab-toolbar">
                        <button class="toolbar-button" @click="loadSoftwareStatus(selectedServer)" :disabled="softwareLoading">{{ softwareLoading ? t('查询中...') : t('↻ 刷新状态') }}</button>
                    </div>
                    <div class="sw-dual-layout">
                    <!-- 已安装软件 -->
                    <div class="sw-section" v-if="softwareStatuses.filter(s => s.installed).length">
                        <h4>{{ t('已安装软件（{p1}）', { p1: softwareStatuses.filter(s => s.installed).length }) }}</h4>
                        <div class="sw-installed-table">
                            <div class="sw-installed-header">
                                <span class="sw-col-name">{{ t('软件') }}</span>
                                <span class="sw-col-version">{{ t('版本') }}</span>
                                <span class="sw-col-status">{{ t('状态') }}</span>
                                <span class="sw-col-service">{{ t('服务控制') }}</span>
                                <span class="sw-col-spacer"></span>
                                <span class="sw-col-actions">{{ t('操作') }}</span>
                            </div>
                            <div v-for="sw in softwareStatuses.filter(s => s.installed)" :key="sw.package.id" class="sw-installed-row">
                                <span class="sw-col-name" :data-title="sw.package.name">
                                    <span class="sw-row-icon">{{ sw.package.icon }}</span>
                                    <strong>{{ sw.package.name }}</strong>
                                </span>
                                <span class="sw-col-version" :data-title="sw.version || sw.package.version || ''"><span class="sw-version-text">{{ sw.version || sw.package.version || '—' }}</span></span>
                                <span class="sw-col-status">
                                    <span class="sw-status-badge" :class="sw.status">{{ sw.status === 'running' ? t('运行中') : sw.status === 'stopped' ? t('已停止') : sw.status === 'none' ? t('无服务') : t('未知') }}</span>
                                </span>
                                <span class="sw-col-service">
                                    <div class="sw-service-btns" v-if="sw.status !== 'none'">
                                        <button class="sw-svc-btn" @click="controlPackageService(sw.package.id, 'start')" :disabled="sw.status === 'running' || softwareOperating[sw.package.id]" :title="t('启动')">▶</button>
                                        <button class="sw-svc-btn" @click="controlPackageService(sw.package.id, 'stop')" :disabled="sw.status === 'stopped' || softwareOperating[sw.package.id]" :title="t('停止')">⏸</button>
                                        <button class="sw-svc-btn" @click="controlPackageService(sw.package.id, 'restart')" :disabled="softwareOperating[sw.package.id]" :title="t('重启')">↻</button>
                                        <button class="sw-svc-btn" @click="viewLog(sw.package.id)" :title="t('日志')">📋</button>
                                    </div>
                                    <span v-else class="text-muted">—</span>
                                </span>
                                <span class="sw-col-spacer"></span>
                                <span class="sw-col-actions">
                                    <button class="sw-btn sw-btn-danger" @click="uninstallPackage(sw.package.id)" :disabled="softwareOperating[sw.package.id]">
                                        {{ softwareOperating[sw.package.id] ? t('卸载中...') : t('卸载') }}
                                    </button>
                                </span>
                            </div>
                        </div>
                    </div>
                    <!-- 可安装软件（仅展示未安装的） -->
                    <div class="sw-section" v-if="availableSoftware.length">
                        <h4>{{ t('可安装软件（{p1}）', { p1: availableSoftware.length }) }}</h4>
                        <div class="sw-install-list">
                            <div v-for="pkg in availableSoftware" :key="pkg.id" class="sw-install-row">
                                <span class="sw-row-icon">{{ pkg.icon }}</span>
                                <div class="sw-install-info">
                                    <strong class="sw-install-name" :data-title="pkg.name">{{ pkg.name }}</strong>
                                    <small class="sw-install-desc" :data-title="pkg.description">{{ pkg.description }}</small>
                                </div>
                                <div class="sw-combo sw-install-combo">
                                    <input class="sw-combo-input" v-model.trim="selectedVersions[pkg.id]" @focus="versionComboOpen = pkg.id" :placeholder="t('请选择版本')" :title="t('从下拉选择通用配置中的预置版本，也可直接输入')">
                                    <button type="button" class="sw-combo-arrow" @click="toggleVersionCombo(pkg.id)" tabindex="-1" :title="t('展开版本列表')">▾</button>
                                    <ul class="sw-combo-list" v-if="versionComboOpen === pkg.id">
                                        <li v-for="v in (pkg.versions || [])" :key="v" :class="{ active: selectedVersions[pkg.id] === v }" @mousedown.prevent="pickVersion(pkg.id, v)">{{ v }}</li>
                                        <li v-if="!(pkg.versions || []).length" class="sw-combo-empty">{{ t('暂无预置版本，可直接输入') }}</li>
                                    </ul>
                                </div>
                                <button class="sw-btn sw-btn-primary" @click="installPackage(pkg.id, selectedVersions[pkg.id] || '')" :disabled="softwareOperating[pkg.id] || selectedServer === 'srv_local'" :title="t('仅远程服务器可安装')">
                                    {{ softwareOperating[pkg.id] ? t('安装中...') : t('安装') }}
                                </button>
                            </div>
                        </div>
                    </div>
                    </div><!-- /.sw-dual-layout -->
                    <!-- 加载中 -->
                    <div v-if="softwareLoading" class="loading-state" style="padding:20px;">
                        <div class="loading-orbit small"><span></span></div><span>{{ t('正在加载软件状态...') }}</span>
                    </div>
                    <!-- 空状态 -->
                    <div v-else-if="!softwareStatuses.filter(s => s.installed).length && !availableSoftware.length" class="list-empty" style="padding:40px;">
                        <span class="empty-icon">⌁</span><strong>{{ t('暂无可管理或安装的软件') }}</strong>
                    </div>
                </div>
`,
};
