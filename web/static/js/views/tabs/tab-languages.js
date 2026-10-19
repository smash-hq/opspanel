import { appCtx } from '../../store/index.js';

export const TabLanguages = {
    name: 'TabLanguages',
    setup: () => appCtx(),
    template: `
                <div class="node-tab-content">
                    <div class="sw-tab-toolbar">
                        <button class="toolbar-button" @click="detectServerLanguages()" :disabled="langDetectLoading">{{ langDetectLoading ? t('检测中...') : t('↻ 刷新检测') }}</button>
                    </div>
                    <div v-if="langDetectLoading" class="loading-state" style="padding:20px;">{{ t('检测中...') }}</div>
                    <div v-else-if="!detectedLanguages.length" class="list-empty" style="padding:40px;">
                        <span class="empty-icon">⌁</span><strong>{{ t('暂无检测数据') }}</strong>
                        <small>{{ t('点击右上角「刷新检测」按钮开始检测服务器已安装的语言环境') }}</small>
                    </div>
                    <template v-else>
                        <div class="sw-dual-layout">
                        <!-- 已安装语言 -->
                        <div class="sw-section" v-if="detectedLanguages.filter(l => l.status === 'installed').length">
                            <h4>{{ t('已安装语言（{p1}）', { p1: detectedLanguages.filter(l => l.status === 'installed').length }) }}</h4>
                            <div class="sw-installed-table lang-table-4col">
                                <div class="sw-installed-header">
                                    <span class="sw-col-name">{{ t('语言') }}</span>
                                    <span class="sw-col-version">{{ t('版本') }}</span>
                                    <span class="sw-col-status">{{ t('状态') }}</span>
                                    <span class="sw-col-spacer"></span>
                                    <span class="sw-col-actions">{{ t('操作') }}</span>
                                </div>
                                <div v-for="lang in detectedLanguages.filter(l => l.status === 'installed')" :key="lang.id" class="sw-installed-row">
                                    <span class="sw-col-name" :data-title="lang.name">
                                        <span class="sw-row-icon">{{ lang.icon || '📦' }}</span>
                                        <strong>{{ lang.name }}</strong>
                                    </span>
                                    <span class="sw-col-version" :data-title="lang.version || ''"><span class="sw-version-text">{{ lang.version || '—' }}</span></span>
                                    <span class="sw-col-status">
                                        <span class="sw-status-badge running">{{ t('已安装') }}</span>
                                    </span>
                                    <span class="sw-col-spacer"></span>
                                    <span class="sw-col-actions">
                                        <button class="sw-btn sw-btn-danger" @click="uninstallLang(lang.id)" :disabled="langInstallStatus === 'installing'">
                                            {{ t('卸载') }}
                                        </button>
                                    </span>
                                </div>
                            </div>
                        </div>
                        <!-- 未安装语言 -->
                        <div class="sw-section" v-if="uninstalledLanguages.length">
                            <h4>{{ t('可安装语言（{p1}）', { p1: uninstalledLanguages.length }) }}</h4>
                            <div class="sw-install-list">
                                <div v-for="lang in uninstalledLanguages" :key="lang.id" class="sw-install-row">
                                    <span class="sw-row-icon">{{ lang.icon || '📦' }}</span>
                                    <div class="sw-install-info">
                                        <strong class="sw-install-name" :data-title="lang.name">{{ lang.name }}</strong>
                                        <small class="sw-install-desc" :data-title="lang.description || ''">{{ lang.description || lang.id }}</small>
                                    </div>
                                    <div class="sw-combo sw-install-combo">
                                        <input class="sw-combo-input" v-model.trim="selectedVersions[lang.id]" @focus="versionComboOpen = lang.id" :placeholder="t('请选择版本')" :title="t('从下拉选择通用配置中的预置版本，也可直接输入')">
                                        <button type="button" class="sw-combo-arrow" @click="toggleVersionCombo(lang.id)" tabindex="-1" :title="t('展开版本列表')">▾</button>
                                        <ul class="sw-combo-list" v-if="versionComboOpen === lang.id">
                                            <li v-for="v in (lang.versions || [])" :key="v" :class="{ active: selectedVersions[lang.id] === v }" @mousedown.prevent="pickVersion(lang.id, v)">{{ v }}</li>
                                            <li v-if="!(lang.versions || []).length" class="sw-combo-empty">{{ t('暂无预置版本，可直接输入') }}</li>
                                        </ul>
                                    </div>
                                    <button class="sw-btn sw-btn-primary" @click="installLang(lang.id, selectedVersions[lang.id] || '')" :disabled="langInstallStatus === 'installing' || selectedServer === 'srv_local'" :title="t('仅远程服务器可安装')">
                                        {{ t('安装') }}
                                    </button>
                                </div>
                            </div>
                        </div>
                        </div><!-- /.sw-dual-layout -->
                    </template>
                </div>
`,
};
