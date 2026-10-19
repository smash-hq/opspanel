import { appCtx } from '../../store/index.js';

export const FilePreviewModal = {
    name: 'FilePreviewModal',
    setup: () => appCtx(),
    template: `
    <div class="modal-overlay" @click.self="closeFilePreview">
        <div class="modal file-preview-modal">
            <div class="modal-header">
                <div><span class="eyebrow">FILE PREVIEW</span><h3>{{ filePreviewData.path }}</h3></div>
                <button class="icon-button" @click="closeFilePreview" :title="t('关闭')">×</button>
            </div>
            <div class="modal-body">
                <div class="log-viewer" v-if="!filePreviewLoading">
                    <pre>{{ filePreviewData.content || t('空文件') }}</pre>
                </div>
                <div class="log-loading" v-else>{{ t('加载中...') }}</div>
            </div>
        </div>
    </div>
`,
};

export const FileDetailModal = {
    name: 'FileDetailModal',
    setup: () => appCtx(),
    template: `
    <div class="modal-overlay" @click.self="closeFileDetail">
        <div class="modal file-detail-modal">
            <div class="modal-header">
                <div><span class="eyebrow">FILE DETAILS</span><h3>{{ fileDetailData.name }}</h3></div>
                <button class="icon-button" @click="closeFileDetail" :title="t('关闭')">×</button>
            </div>
            <div class="modal-body">
                <div class="file-detail-grid">
                    <div class="detail-item"><span class="detail-label">{{ t('路径') }}</span><code>{{ fileDetailData.path }}</code></div>
                    <div class="detail-item"><span class="detail-label">{{ t('类型') }}</span><span>{{ fileDetailData.is_dir ? t('目录') : t('文件') }}</span></div>
                    <div class="detail-item"><span class="detail-label">{{ t('大小') }}</span><span>{{ formatFileSize(fileDetailData.size) }}</span></div>
                    <div class="detail-item"><span class="detail-label">{{ t('权限') }}</span><code>{{ fileDetailData.mode }}</code></div>
                </div>
            </div>
            <div class="modal-footer">
                <button class="toolbar-button" @click="closeFileDetail">{{ t('关闭') }}</button>
                <button class="primary-button" @click="handleDownloadFile({path: fileDetailData.path, name: fileDetailData.name})">{{ t('下载') }}</button>
            </div>
        </div>
    </div>
`,
};
