import { appCtx } from '../../store/index.js';

export const TabFiles = {
    name: 'TabFiles',
    setup: () => appCtx(),
    template: `
                <div class="node-tab-content tab-scroll">
                    <!-- 权限警告 -->
                    <div v-if="fileSudoWarning" class="file-sudo-warning">
                        <span class="warning-icon">⚠</span>
                        <div class="warning-content">
                            <strong>{{ t('权限不足 — 部分文件可能无法显示') }}</strong>
                            <p>{{ t('当前未配置 sudo 权限，部分受保护的文件和目录可能无法访问。建议为用户配置 NOPASSWD sudo。') }}</p>
                            <button class="warning-action" @click="openPermGuide">{{ t('查看配置方法') }}</button>
                        </div>
                    </div>
                    <!-- 面包屑导航 -->
                    <div class="file-breadcrumb">
                        <button class="bc-home" @click="navigateToPath('/')">/</button>
                        <template v-for="(segment, i) in currentFilePath.split('/').filter(Boolean)" :key="i">
                            <span class="bc-sep">/</span>
                            <button class="bc-segment" @click="navigateToPath('/' + currentFilePath.split('/').filter(Boolean).slice(0, i+1).join('/') + '/')">{{ segment }}</button>
                        </template>
                    </div>
                    <!-- 工具栏 -->
                    <div class="file-toolbar">
                        <div class="file-search">
                            <span class="search-icon">⌕</span>
                            <input v-model="fileSearchQuery" :placeholder="t('搜索文件...')">
                        </div>
                        <button class="toolbar-button" @click="loadFiles()" :disabled="filesLoading">{{ t('↻ 刷新') }}</button>
                    </div>
                    <!-- 文件列表（目录在前、名称排序，类似 ls -l） -->
                    <div v-if="filesLoading" class="loading-state" style="padding:20px;">{{ t('加载中...') }}</div>
                    <div v-else-if="!sortedEntries.length" class="list-empty">
                        <span class="empty-icon">⌁</span><strong>{{ t('空目录') }}</strong>
                    </div>
                    <div v-else class="file-section">
                        <div class="file-section-title">
                            <span class="section-icon">📂</span>{{ t('条目 ({p1})', { p1: sortedEntries.length }) }}
                        </div>
                        <div class="file-table">
                            <div class="file-table-header">
                                <span class="ft-name">{{ t('名称') }}</span>
                                <span class="ft-size">{{ t('大小') }}</span>
                                <span class="ft-mode">{{ t('权限') }}</span>
                                <span class="ft-time">{{ t('修改时间') }}</span>
                                <span class="ft-actions">{{ t('操作') }}</span>
                            </div>
                            <div v-for="file in sortedEntries" :key="file.path" class="file-table-row" :class="{ 'is-dir-row': file.is_dir }">
                                <span class="ft-name">
                                    <span class="ft-icon">{{ file.is_hidden ? '🔒' : (file.is_dir ? '📁' : (file.link_target ? '🔗' : getFileIcon(file.name))) }}</span>
                                    <span class="ft-link" :class="{ 'ft-link-dir': file.is_dir }" @click="file.is_dir ? navigateToPath(file.path) : openFilePreview(file)">{{ file.name }}</span>
                                    <span class="ft-link-target" v-if="file.link_target" :title="t('链接目标: {p1}', { p1: file.link_target })">→ {{ file.link_target }}</span>
                                </span>
                                <span class="ft-size">{{ file.is_dir ? '—' : formatFileSize(file.size) }}</span>
                                <span class="ft-mode">{{ file.mode }}</span>
                                <span class="ft-time">{{ formatModTime(file.mod_time) }}</span>
                                <span class="ft-actions">
                                    <button class="ft-btn" v-if="!file.is_dir" @click="openFilePreview(file)" :title="t('预览')">👁</button>
                                    <button class="ft-btn" @click="openFileDetail(file)" :title="t('详情')">ℹ</button>
                                    <button class="ft-btn" @click="handleDownloadFile(file)" :title="file.is_dir ? t('下载（tar.gz 打包）') : t('下载')">⬇</button>
                                </span>
                            </div>
                        </div>
                    </div>
                </div>
`,
};
