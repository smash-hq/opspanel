import { appCtx } from '../store/index.js';

export const ConfigPage = {
    name: 'ConfigPage',
    setup: () => appCtx(),
    template: `
            <section class="config-page">
                <!-- 配置侧栏 -->
                <div class="config-sidebar">
                    <div class="config-menu-item" :class="{ active: configTab === 'sources' }" @click="configTab = 'sources'">
                        <span class="config-menu-label">{{ t('软件源') }}</span>
                        <span class="config-menu-badge">{{ sources.length }}</span>
                    </div>
                    <div class="config-menu-item" :class="{ active: configTab === 'docker-sources' }" @click="configTab = 'docker-sources'">
                        <span class="config-menu-label">{{ t('Docker 镜像源') }}</span>
                        <span class="config-menu-badge">{{ dockerSources.length }}</span>
                    </div>
                    <div class="config-menu-item" :class="{ active: configTab === 'packages' }" @click="configTab = 'packages'">
                        <span class="config-menu-label">{{ t('软件包') }}</span>
                        <span class="config-menu-badge">{{ softwareCatalog.length }}</span>
                    </div>
                    <div class="config-menu-item" :class="{ active: configTab === 'languages' }" @click="configTab = 'languages'">
                        <span class="config-menu-label">{{ t('语言环境') }}</span>
                        <span class="config-menu-badge">{{ languages.length }}</span>
                    </div>
                    <div class="config-menu-item" :class="{ active: configTab === 'startup' }" @click="configTab = 'startup'">
                        <span class="config-menu-label">{{ t('启动信息') }}</span>
                    </div>
                    <div class="config-menu-item" :class="{ active: configTab === 'favicon' }" @click="configTab = 'favicon'">
                        <span class="config-menu-label">{{ t('网站图标') }}</span>
                    </div>
                </div>
                <!-- 配置内容 -->
                <div class="config-content">
                    <!-- 软件源管理 -->
                    <div v-if="configTab === 'sources'">
                        <h3>{{ t('软件源管理') }}</h3>
                        <p class="settings-desc">{{ t('管理软件安装使用的系统镜像源与软件仓库地址（全局生效，不含 Docker 镜像源）') }}</p>
                        <div class="section-header">
                            <button class="primary-button small" @click="openSourceForm('add')">{{ t('＋ 添加软件源') }}</button>
                        </div>
                        <div class="sources-list">
                            <div v-for="src in sources" :key="src.id" class="source-card">
                                <div class="source-info">
                                    <span class="source-type" :class="src.type">{{ src.type === 'mirror' ? t('镜像') : t('仓库') }}</span>
                                    <strong>{{ src.name }}</strong>
                                    <small>{{ src.url }}</small>
                                </div>
                                <div class="source-meta">
                                    <span class="source-priority">{{ t('优先级: {p1}', { p1: src.priority }) }}</span>
                                    <span class="source-status" :class="src.enabled ? 'enabled' : 'disabled'">{{ src.enabled ? t('启用') : t('禁用') }}</span>
                                </div>
                                <div class="source-actions">
                                    <button @click="openSourceForm('edit', src)" :title="t('编辑')">✎</button>
                                    <button class="danger" @click="handleDeleteSource(src.id)" :title="t('删除')">×</button>
                                </div>
                            </div>
                            <div v-if="!sources.length" class="list-empty">
                                <span class="empty-icon">⌁</span>
                                <strong>{{ t('还没有软件源') }}</strong>
                                <small>{{ t('添加镜像源或软件仓库地址') }}</small>
                            </div>
                        </div>
                    </div>
                    <!-- 软件包管理 -->
                    <div v-if="configTab === 'packages'">
                        <h3>{{ t('软件包管理') }}</h3>
                        <p class="settings-desc">{{ t('管理可安装的软件包定义（全局生效）') }}</p>
                        <div class="section-header">
                            <button class="primary-button small" @click="openPackageForm('add')">{{ t('＋ 添加软件包') }}</button>
                        </div>
                        <div class="packages-list">
                            <div v-for="pkg in softwareCatalog" :key="pkg.id" class="package-card">
                                <span class="package-icon">{{ pkg.icon }}</span>
                                <div class="package-info">
                                    <strong>{{ pkg.name }}</strong>
                                    <small>{{ pkg.description }}</small>
                                    <span class="package-category">{{ pkg.category }}</span>
                                </div>
                                <div class="package-actions">
                                    <button @click="openConfigDetail('pkg', pkg)" :title="t('详情')">ℹ</button>
                                    <button @click="openPackageForm('edit', pkg)" :title="t('编辑')">✎</button>
                                    <button class="danger" @click="handleDeletePackage(pkg.id)" :title="t('删除')">×</button>
                                </div>
                            </div>
                        </div>
                    </div>
                    <!-- 语言环境管理 -->
                    <div v-if="configTab === 'languages'">
                        <h3>{{ t('编程语言环境') }}</h3>
                        <p class="settings-desc">{{ t('管理服务器编程语言环境定义（全局生效）') }}</p>
                        <div class="section-header">
                            <button class="primary-button small" @click="openLangForm('add')">{{ t('＋ 添加语言') }}</button>
                        </div>
                        <div class="packages-list">
                            <div v-for="lang in languages" :key="lang.id" class="package-card">
                                <span class="package-icon">{{ lang.icon || '📦' }}</span>
                                <div class="package-info">
                                    <strong>{{ lang.name }}</strong>
                                    <small>{{ lang.description }}</small>
                                    <span class="package-category">{{ lang.category }}</span>
                                </div>
                                <div class="package-actions">
                                    <button @click="openConfigDetail('lang', lang)" :title="t('详情')">ℹ</button>
                                    <button @click="openLangForm('edit', lang)" :title="t('编辑')">✎</button>
                                    <button class="danger" @click="deleteLanguage(lang.id)" :title="t('删除')">×</button>
                                </div>
                            </div>
                        </div>
                    </div>
                    <!-- Docker 镜像源管理 -->
                    <div v-if="configTab === 'docker-sources'">
                        <h3>{{ t('Docker 镜像源管理') }}</h3>
                        <p class="settings-desc">{{ t('管理 Docker 镜像加速器源（全局生效，可应用到各服务器）') }}</p>
                        <div class="section-header">
                            <button class="primary-button small" @click="openDockerSourceForm()">{{ t('＋ 添加镜像源') }}</button>
                        </div>
                        <div class="docker-source-list">
                            <div v-for="src in dockerSources" :key="src.id" class="docker-source-row">
                                <span class="src-default" v-if="src.is_default">★</span>
                                <span class="src-default empty" v-else>☆</span>
                                <span class="src-name">{{ src.name }}</span>
                                <span class="src-url">{{ src.url }}</span>
                                <span class="src-status" :class="src.enabled ? 'enabled' : 'disabled'">{{ src.enabled ? t('启用') : t('禁用') }}</span>
                                <span class="src-actions">
                                    <button class="proc-btn" @click="setDefaultSource(src.id)" :title="t('设为默认')" v-if="!src.is_default">★</button>
                                    <button class="proc-btn" @click="openDockerSourceForm(src)" :title="t('编辑')">✎</button>
                                    <button class="proc-btn danger" @click="removeDockerSource(src.id)" :title="t('删除')" v-if="!src.is_default">×</button>
                                </span>
                            </div>
                            <div v-if="!dockerSources.length" class="list-empty">
                                <span class="empty-icon">⌁</span>
                                <strong>{{ t('还没有镜像源') }}</strong>
                                <small>{{ t('添加 Docker 镜像加速器源') }}</small>
                            </div>
                        </div>
                    </div>
                    <!-- 启动信息 -->
                    <div v-if="configTab === 'startup'">
                        <h3>{{ t('启动信息') }}</h3>
                        <p class="settings-desc">{{ t('配置面板启动时加载的全局信息') }}</p>
                        <div class="form-grid">
                            <label class="field full"><span>{{ t('面板名称') }}</span><input v-model="settingsForm.panelName" placeholder="OpsPanel"></label>
                            <label class="field full"><span>{{ t('管理员邮箱') }}</span><input v-model="settingsForm.adminEmail" placeholder="admin@example.com"></label>
                            <label class="field"><span>{{ t('推送间隔（秒）') }}</span><input v-model.number="settingsForm.pushInterval" type="number" min="3" max="60"></label>
                            <label class="field"><span>{{ t('默认 SSH 端口') }}</span><input v-model.number="settingsForm.defaultSSHPort" type="number" min="1" max="65535"></label>
                            <label class="field full"><span>{{ t('欢迎语') }}</span><textarea v-model="settingsForm.welcomeMessage" rows="2" :placeholder="t('欢迎使用 OpsPanel 服务器管理面板')"></textarea></label>
                        </div>
                        <div class="settings-actions">
                            <button class="primary-button" @click="saveSettings" :disabled="settingsSaving">{{ settingsSaving ? t('保存中...') : t('保存设置') }}</button>
                        </div>
                    </div>
                    <!-- 网站图标 -->
                    <div v-if="configTab === 'favicon'">
                        <h3>{{ t('网站图标') }}</h3>
                        <p class="settings-desc">{{ t('自定义浏览器标签页图标（支持 SVG/PNG/ICO，最大 1MB）') }}</p>
                        <div class="favicon-upload">
                            <div class="favicon-preview">
                                <img :src="'/api/v1/settings/favicon?' + faviconTs" :alt="t('当前图标')">
                            </div>
                            <div class="favicon-actions">
                                <label class="toolbar-button">
                                    {{ t('📤 上传图标') }}
                                    <input type="file" accept="image/*" hidden @change="uploadFavicon">
                                </label>
                                <button class="toolbar-button danger-text" @click="resetFavicon" :disabled="!hasCustomFavicon">{{ t('↺ 恢复默认') }}</button>
                            </div>
                        </div>
                    </div>
                </div>
            </section>
`,
};
