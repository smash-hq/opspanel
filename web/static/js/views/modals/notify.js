import { appCtx } from '../../store/index.js';

export const NotifDetailModal = {
    name: 'NotifDetailModal',
    setup: () => appCtx(),
    template: `
    <div class="modal-overlay" @click.self="closeNotifDetail">
        <div class="modal notif-detail-modal">
            <div class="modal-header">
                <div><span class="eyebrow">NOTIFICATION</span><h3>{{ t('通知详情') }}</h3></div>
                <button class="icon-button" @click="closeNotifDetail" :title="t('关闭')">×</button>
            </div>
            <div class="modal-body" v-if="notifDetail">
                <div class="notif-detail-title">{{ notifDetail.title }}</div>
                <div class="notif-detail-meta">
                    <span class="notify-type-tag" :class="notifDetail.level">{{ notifyTypeLabel(notifDetail.level) }}</span>
                    <span class="notify-module-tag">{{ notifyModuleLabel(notifDetail.module) }}</span>
                    <span class="notify-read-state" :class="{ unread: !notifDetail.read }">{{ notifDetail.read ? t('已读') : t('未读') }}</span>
                    <span class="notif-detail-time">{{ formatNotifyTime(notifDetail.created_at) }}</span>
                </div>
                <div class="notif-detail-section">
                    <div class="form-section-title">{{ t('详情') }}</div>
                    <pre class="notif-detail-content">{{ notifDetail.message || t('（无更多详情）') }}</pre>
                </div>
            </div>
            <div class="modal-footer">
                <button class="primary-button" @click="closeNotifDetail">{{ t('关闭') }}</button>
            </div>
        </div>
    </div>
`,
};
