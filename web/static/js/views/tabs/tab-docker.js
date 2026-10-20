import { appCtx } from '../../store/index.js';

export const TabDocker = {
    name: 'TabDocker',
    setup: () => appCtx(),
    template: `
                <div class="node-tab-content tab-scroll">
                    <!-- Docker 信息栏 -->
                    <div class="docker-header">
                        <div class="docker-info">
                            <span class="docker-badge" :class="dockerAvailable ? 'ok' : 'error'">{{ dockerAvailable ? t('● Docker 已就绪') : t('○ Docker 未安装') }}</span>
                            <span class="docker-version" v-if="dockerVersion">{{ dockerVersion }}</span>
                        </div>
                        <div class="docker-stats">
                            <span class="docker-stat">{{ t('镜像:') }} <strong>{{ dockerImages.length }}</strong></span>
                            <span class="docker-stat">{{ t('容器:') }} <strong>{{ dockerContainers.length }}</strong></span>
                            <span class="docker-stat" v-if="selectedServer !== 'srv_local' && serverMirror.mirror">{{ t('镜像源:') }} <strong>{{ serverMirror.mirror }}</strong></span>
                        </div>
                    </div>
                    <!-- Docker 子 Tab -->
                    <div class="docker-tabs">
                        <button class="docker-tab" :class="{ active: dockerSubTab === 'images' }" @click="dockerSubTab = 'images'; loadDockerImages()">{{ t('镜像管理') }}</button>
                        <button class="docker-tab" :class="{ active: dockerSubTab === 'containers' }" @click="dockerSubTab = 'containers'; loadDockerContainers(true)">{{ t('容器管理') }}</button>
                        <button class="docker-tab" :class="{ active: dockerSubTab === 'mirror' }" @click="dockerSubTab = 'mirror'; loadServerMirror(); loadDockerSources()">{{ t('镜像源管理') }}</button>
                    </div>
                    <!-- 镜像管理 -->
                    <div v-if="dockerSubTab === 'images'" class="docker-sub-pane">
                        <div class="docker-toolbar">
                            <input v-model="dockerPullInput" :placeholder="t('输入镜像名，如：nginx:latest')" @keyup.enter="dockerPullImage()">
                            <button class="primary-button small" @click="dockerPullImage()">{{ t('⬇ 拉取') }}</button>
                            <button class="toolbar-button" @click="loadDockerImages()" :disabled="dockerLoading">{{ t('↻ 刷新') }}</button>
                        </div>
                        <!-- 批量操作栏 -->
                        <div class="batch-toolbar" v-if="selectedImages.length">
                            <span class="batch-info">{{ t('已选中 {p1} 个镜像', { p1: selectedImages.length }) }}</span>
                            <button class="toolbar-button danger-text" @click="batchDeleteImages()" :disabled="batchActionLoading">{{ t('🗑 批量删除') }}</button>
                            <button class="toolbar-button" @click="selectedImages = []">{{ t('取消选择') }}</button>
                        </div>
                        <div v-if="dockerLoading" class="loading-state" style="padding:20px;">{{ t('加载中...') }}</div>
                        <div v-else-if="!dockerImages.length" class="list-empty">
                            <span class="empty-icon">⌁</span><strong>{{ t('暂无镜像') }}</strong>
                        </div>
                        <div v-else class="docker-image-list">
                            <div class="docker-list-header image">
                                <input type="checkbox" class="batch-check" :checked="allImagesSelected" @change="toggleAllImages()" :title="t('全选')">
                                <span>{{ t('名称') }}</span>
                                <span>{{ t('ID') }}</span>
                                <span class="docker-img-size">{{ t('大小') }}</span>
                                <span class="docker-img-actions">{{ t('操作') }}</span>
                            </div>
                            <div v-for="img in dockerImages" :key="img.id" class="docker-image-row" :class="{ selected: selectedImages.includes(img.id) }">
                                <input type="checkbox" class="batch-check" :checked="selectedImages.includes(img.id)" @change="toggleImageSelection(img.id)">
                                <span class="docker-img-repo">{{ img.repository }}:{{ img.tag }}</span>
                                <span class="docker-img-id">{{ img.id.substring(0, 12) }}</span>
                                <span class="docker-img-size">{{ img.size }}</span>
                                <span class="docker-img-actions">
                                    <button class="docker-row-btn primary" @click="openRunForm(img)">{{ t('部署') }}</button>
                                    <button class="docker-row-btn danger" @click="dockerRemoveImage(img.id)">{{ t('删除') }}</button>
                                </span>
                            </div>
                        </div>
                    </div>
                    <!-- 容器管理 -->
                    <div v-if="dockerSubTab === 'containers'" class="docker-sub-pane">
                        <div class="docker-toolbar">
                            <label class="show-all-label">
                                <input type="checkbox" v-model="dockerShowAll" @change="loadDockerContainers(dockerShowAll)"> {{ t('显示已停止') }}
                            </label>
                            <button class="toolbar-button" @click="loadDockerContainers(dockerShowAll)" :disabled="dockerLoading">{{ t('↻ 刷新') }}</button>
                        </div>
                        <!-- 批量操作栏 -->
                        <div class="batch-toolbar" v-if="selectedContainers.length">
                            <span class="batch-info">{{ t('已选中 {p1} 个容器', { p1: selectedContainers.length }) }}</span>
                            <button class="toolbar-button" @click="batchContainerAction('start')" :disabled="batchActionLoading">{{ t('▶ 批量启动') }}</button>
                            <button class="toolbar-button" @click="batchContainerAction('stop')" :disabled="batchActionLoading">{{ t('⏸ 批量停止') }}</button>
                            <button class="toolbar-button" @click="batchContainerAction('restart')" :disabled="batchActionLoading">{{ t('↻ 批量重启') }}</button>
                            <button class="toolbar-button danger-text" @click="batchContainerAction('rm')" :disabled="batchActionLoading">{{ t('🗑 批量删除') }}</button>
                            <button class="toolbar-button" @click="selectedContainers = []">{{ t('取消选择') }}</button>
                        </div>
                        <div v-if="dockerLoading" class="loading-state" style="padding:20px;">{{ t('加载中...') }}</div>
                        <div v-else-if="!dockerContainers.length" class="list-empty">
                            <span class="empty-icon">⌁</span><strong>{{ t('暂无容器') }}</strong>
                        </div>
                        <div v-else class="docker-container-list">
                            <div class="docker-list-header container">
                                <input type="checkbox" class="batch-check" :checked="allContainersSelected" @change="toggleAllContainers()" :title="t('全选')">
                                <span>{{ t('名称') }}</span>
                                <span>{{ t('镜像') }}</span>
                                <span class="docker-ctr-status">{{ t('状态') }}</span>
                                <span class="docker-ctr-time">{{ t('运行时长') }}</span>
                                <span class="docker-ctr-ports">{{ t('端口') }}</span>
                                <span class="docker-ctr-actions">{{ t('操作') }}</span>
                            </div>
                            <div v-for="c in dockerContainers" :key="c.id" class="docker-container-row" :class="{ selected: selectedContainers.includes(c.id) }">
                                <input type="checkbox" class="batch-check" :checked="selectedContainers.includes(c.id)" @change="toggleContainerSelection(c.id)">
                                <span class="docker-ctr-name">{{ c.names || c.id.substring(0, 12) }}</span>
                                <span class="docker-ctr-image">{{ c.image }}</span>
                                <span class="docker-ctr-status" :class="c.state">{{ c.state }}</span>
                                <span class="docker-ctr-time">{{ c.running_for || '—' }}</span>
                                <span class="docker-ctr-ports">{{ c.ports || '—' }}</span>
                                <span class="docker-ctr-actions">
                                    <template v-if="c.state === 'running'">
                                        <button class="ctr-btn" @click="openContainerTerminal(c.id)" :title="t('进入容器 (docker exec -it)')">{{ t('进入容器') }}</button>
                                        <button class="ctr-btn" @click="dockerContainerAction(c.id, 'stop')" :title="t('停止')">{{ t('停止') }}</button>
                                        <button class="ctr-btn" @click="dockerContainerAction(c.id, 'restart')" :title="t('重启')">{{ t('重启') }}</button>
                                        <button class="ctr-btn" @click="openDockerFiles(c.id)" :title="t('文件')">{{ t('文件') }}</button>
                                        <button class="ctr-btn" @click="dockerViewLogs(c.id)" :title="t('日志')">{{ t('日志') }}</button>
                                    </template>
                                    <template v-else-if="c.state === 'restarting'">
                                        <button class="ctr-btn danger" @click="dockerContainerAction(c.id, 'kill')" :title="t('强制终止')">{{ t('强制终止') }}</button>
                                        <button class="ctr-btn" @click="dockerViewLogs(c.id)" :title="t('日志')">{{ t('日志') }}</button>
                                    </template>
                                    <template v-else>
                                        <button class="ctr-btn" @click="dockerContainerAction(c.id, 'start')" :title="t('启动')">{{ t('启动') }}</button>
                                        <button class="ctr-btn" @click="dockerViewLogs(c.id)" :title="t('日志')">{{ t('日志') }}</button>
                                    </template>
                                    <button class="ctr-btn danger" @click="dockerRemoveContainer(c.id)" :title="t('删除')">{{ t('删除') }}</button>
                                </span>
                            </div>
                        </div>
                    </div>
                    <!-- 镜像源管理 -->
                    <div v-if="dockerSubTab === 'mirror'" class="docker-sub-pane docker-sub-pane-scroll">
                        <!-- 服务器当前镜像源 -->
                        <div class="mirror-server-panel" v-if="selectedServer !== 'srv_local'">
                            <div class="mirror-section-header">
                                <h3>{{ t('服务器 Docker 镜像源配置') }}</h3>
                                <button class="toolbar-button" @click="loadServerMirror()" :disabled="serverMirrorLoading">{{ t('↻ 刷新') }}</button>
                            </div>
                            <div v-if="serverMirrorLoading" class="loading-state" style="padding:20px;">{{ t('读取中...') }}</div>
                            <div v-else class="mirror-server-info">
                                <div class="mirror-info-row" v-if="serverMirror.mirror">
                                    <span class="mirror-info-label">registry-mirrors:</span>
                                    <code class="mirror-info-value">{{ serverMirror.mirror }}</code>
                                </div>
                                <div class="mirror-info-row" v-if="serverMirror.registry">
                                    <span class="mirror-info-label">insecure-registries:</span>
                                    <code class="mirror-info-value">{{ serverMirror.registry }}</code>
                                </div>
                                <div class="mirror-info-row" v-if="serverMirror.raw_json && !serverMirror.mirror && !serverMirror.registry">
                                    <span class="mirror-info-label">daemon.json:</span>
                                    <pre class="mirror-info-raw">{{ serverMirror.raw_json }}</pre>
                                </div>
                                <div class="mirror-info-row" v-if="serverMirror.raw_json && (serverMirror.mirror || serverMirror.registry)">
                                    <span class="mirror-info-label">{{ t('原始配置:') }}</span>
                                    <details class="mirror-raw-details">
                                        <summary>{{ t('查看 daemon.json') }}</summary>
                                        <pre class="mirror-info-raw">{{ serverMirror.raw_json }}</pre>
                                    </details>
                                </div>
                                <p class="mirror-info-hint" v-if="!serverMirror.mirror && !serverMirror.registry && !serverMirror.raw_json">{{ t('该服务器未配置镜像源，使用 Docker Hub 默认拉取') }}</p>
                            </div>
                        </div>
                        <!-- 快速应用镜像源 -->
                        <div class="mirror-apply-panel">
                            <div class="mirror-section-header">
                                <h3>{{ t('快速应用镜像源到当前服务器') }}</h3>
                            </div>
                            <div class="mirror-selector">
                                <span class="mirror-label">{{ t('选择镜像源:') }}</span>
                                <select v-model="mirrorSelectMode" @change="onMirrorSelectModeChange">
                                    <option value="preset">{{ t('预设镜像源') }}</option>
                                    <option value="custom">{{ t('自定义') }}</option>
                                </select>
                                <select v-if="mirrorSelectMode === 'preset'" v-model="selectedMirror" class="mirror-preset-select">
                                    <option value="">{{ t('请选择镜像源...') }}</option>
                                    <option v-for="src in dockerSources.filter(s => s.enabled)" :key="src.id" :value="src.url">{{ src.name }}</option>
                                </select>
                                <input v-else v-model="selectedMirror" :placeholder="t('输入自定义镜像源地址，如 https://registry.example.com')" class="mirror-custom-input">
                                <button class="primary-button small" @click="applyMirrorToServer(selectedMirror)" :disabled="!selectedMirror || serverMirrorLoading">{{ t('应用到此服务器') }}</button>
                            </div>
                            <p class="mirror-info-hint">{{ t('应用后会自动写入 daemon.json 并重启 Docker 服务') }}</p>
                        </div>
                        <!-- 预设镜像源列表管理 -->
                        <div class="mirror-sources-panel">
                            <div class="mirror-section-header">
                                <h3>{{ t('预设镜像源列表') }}</h3>
                                <button class="primary-button small" @click="openDockerSourceForm()">{{ t('＋ 添加镜像源') }}</button>
                            </div>
                            <div class="mirror-sources-table">
                                <div class="mirror-sources-header">
                                    <span class="ms-col-name">{{ t('名称') }}</span>
                                    <span class="ms-col-url">{{ t('地址') }}</span>
                                    <span class="ms-col-status">{{ t('状态') }}</span>
                                    <span class="ms-col-actions">{{ t('操作') }}</span>
                                </div>
                                <div v-for="src in dockerSources" :key="src.id" class="mirror-source-row">
                                    <span class="ms-col-name">
                                        <strong>{{ src.name }}</strong>
                                        <span v-if="src.is_default" class="ms-default-badge">{{ t('默认') }}</span>
                                    </span>
                                    <span class="ms-col-url"><code>{{ src.url }}</code></span>
                                    <span class="ms-col-status">
                                        <span class="ms-status-badge" :class="src.enabled ? 'enabled' : 'disabled'">{{ src.enabled ? t('启用') : t('禁用') }}</span>
                                    </span>
                                    <span class="ms-col-actions">
                                        <button class="ms-action-btn" @click="openDockerSourceForm(src)" :title="t('编辑')">✎</button>
                                        <button class="ms-action-btn" v-if="!src.is_default" @click="setDefaultSource(src.id)" :title="t('设为默认')">⭐</button>
                                        <button class="ms-action-btn danger" @click="removeDockerSource(src.id)" :title="t('删除')">×</button>
                                    </span>
                                </div>
                                <div v-if="!dockerSources.length" class="list-empty">
                                    <span class="empty-icon">⌁</span><strong>{{ t('暂无预设镜像源') }}</strong>
                                </div>
                            </div>
                        </div>
                    </div>
                </div>
`,
};
