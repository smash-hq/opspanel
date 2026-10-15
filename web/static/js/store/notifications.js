/**
 * notifications.js — 站内通知域
 *
 * 顶栏铃铛通知中心：列表、未读数、全部已读、清空；
 * 每 30s 轮询未读数（页面隐藏时跳过），服务异常事件由后端写入。
 * 另含「通知中心」页面：按类型/模块/已读筛选列表，以及 SMTP 邮件通知配置。
 */

import {
    fetchNotifications, markNotificationsRead, markNotificationRead, clearNotifications,
    fetchEmailConfig, updateEmailConfig, sendTestEmail,
} from '../api/notifications.js';

const { ref, reactive, computed } = window.Vue;

/** 通知类型（level）→ 展示文案。 */
const TYPE_LABELS = { info: '信息', warn: '警告', error: '错误' };

/** 来源模块 → 展示文案（与节点页签名称一致）。 */
const MODULE_LABELS = {
    apps: '服务部署',
    software: '软件管理',
    servers: '节点管理',
    terminal: '终端',
    cron: '定时任务',
    system: '系统信息',
    gpu: 'GPU',
    docker: 'Docker',
};

export function setupNotifications(ctx) {
    const notifications = ref([]);
    const unreadCount = ref(0);
    const notifyPanelVisible = ref(false);
    const notifyLoading = ref(false);
    const notifDetailVisible = ref(false);
    const notifDetail = ref(null);

    /** 拉取通知列表与未读数。 */
    async function loadNotifications() {
        notifyLoading.value = true;
        try {
            const data = await fetchNotifications(50);
            notifications.value = data.items || [];
            unreadCount.value = data.unread || 0;
        } catch {
            // 静默失败：通知非关键路径，不打断用户
        } finally {
            notifyLoading.value = false;
        }
    }

    /** 仅刷新未读数（轮询用，limit=1 减少传输）。 */
    async function refreshUnread() {
        if (document.hidden) return;
        try {
            const data = await fetchNotifications(1);
            unreadCount.value = data.unread || 0;
        } catch {
            // 静默失败
        }
    }

    /** 开关通知面板；打开时拉取最新列表。 */
    function toggleNotifyPanel() {
        notifyPanelVisible.value = !notifyPanelVisible.value;
        if (notifyPanelVisible.value) loadNotifications();
    }

    function closeNotifyPanel() {
        notifyPanelVisible.value = false;
    }

    /** 全部标记已读。 */
    async function markAllRead() {
        try {
            await markNotificationsRead();
            unreadCount.value = 0;
            notifications.value = notifications.value.map(n => ({ ...n, read: true }));
        } catch (err) {
            ctx.toastNotify(err.message || '标记已读失败', true);
        }
    }

    /** 清空全部通知。 */
    async function clearAllNotifications() {
        try {
            await clearNotifications();
            notifications.value = [];
            unreadCount.value = 0;
        } catch (err) {
            ctx.toastNotify(err.message || '清空失败', true);
        }
    }

    /** 打开通知详情；收起通知中心，未读则同时标记已读。 */
    async function openNotification(n) {
        notifyPanelVisible.value = false;
        notifDetail.value = n;
        notifDetailVisible.value = true;
        if (!n.read) {
            n.read = true;
            try {
                const data = await markNotificationRead(n.id);
                unreadCount.value = data.unread || 0;
            } catch {
                // 静默失败：已读状态下次拉取会纠正
            }
        }
    }

    function closeNotifDetail() {
        notifDetailVisible.value = false;
    }

    /** 通知类型展示文案。 */
    function notifyTypeLabel(level) {
        return TYPE_LABELS[level] || '信息';
    }

    /** 来源模块展示文案。 */
    function notifyModuleLabel(module) {
        return MODULE_LABELS[module] || module || '未知模块';
    }

    /** 通知时间格式化：今天 HH:MM / 昨天 HH:MM / MM-DD HH:MM。 */
    function formatNotifyTime(ts) {
        if (!ts) return '';
        const d = new Date(ts * 1000);
        const now = new Date();
        const pad = n => String(n).padStart(2, '0');
        const hm = `${pad(d.getHours())}:${pad(d.getMinutes())}`;
        const sameDay = (a, b) => a.getFullYear() === b.getFullYear() && a.getMonth() === b.getMonth() && a.getDate() === b.getDate();
        const yesterday = new Date(now);
        yesterday.setDate(now.getDate() - 1);
        if (sameDay(d, now)) return hm;
        if (sameDay(d, yesterday)) return `昨天 ${hm}`;
        return `${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${hm}`;
    }

    // ===== 通知中心页面：筛选 =====

    /** 打开通知中心页面：收起铃铛面板，刷新列表并加载邮件配置。 */
    function openNotifyCenter() {
        ctx.activeMenu.value = 'notifycenter';
        notifyPanelVisible.value = false;
        loadNotifications();
        loadEmailConfig();
    }

    const ncFilterRead = ref('all');     // all | unread | read
    const ncFilterLevel = ref('all');    // all | info | warn | error
    const ncFilterModule = ref('all');   // all | 具体模块 key

    /** 通知中心页可筛选的模块选项（来自 MODULE_LABELS）。 */
    const ncModuleOptions = Object.keys(MODULE_LABELS).map(k => ({ value: k, label: MODULE_LABELS[k] }));

    const ncFiltered = computed(() => notifications.value.filter(n => {
        if (ncFilterRead.value === 'unread' && n.read) return false;
        if (ncFilterRead.value === 'read' && !n.read) return false;
        if (ncFilterLevel.value !== 'all' && (n.level || 'info') !== ncFilterLevel.value) return false;
        if (ncFilterModule.value !== 'all' && (n.module || '') !== ncFilterModule.value) return false;
        return true;
    }));

    // ===== 邮件通知（SMTP）配置 =====
    const emailForm = reactive({
        enabled: false, host: '', port: 465, user: '', password: '',
        from: '', to: '', encryption: 'ssl',
    });
    const emailHasPassword = ref(false);
    const emailSaving = ref(false);
    const emailTesting = ref(false);

    /** 拉取邮件配置（密码脱敏，仅告知是否已设置）。 */
    async function loadEmailConfig() {
        try {
            const data = await fetchEmailConfig();
            Object.assign(emailForm, {
                enabled: !!data.enabled,
                host: data.host || '',
                port: data.port || 465,
                user: data.user || '',
                password: '',
                from: data.from || '',
                to: data.to || '',
                encryption: data.encryption || 'ssl',
            });
            emailHasPassword.value = !!data.has_password;
        } catch (err) {
            ctx.toastNotify(err.message || '读取邮件配置失败', true);
        }
    }

    /** 保存邮件配置；password 留空表示沿用已存密码。 */
    async function saveEmailConfig() {
        emailSaving.value = true;
        try {
            const data = await updateEmailConfig({ ...emailForm });
            emailHasPassword.value = !!data.has_password;
            emailForm.password = '';
            ctx.toastNotify('邮件配置已保存');
        } catch (err) {
            ctx.toastNotify(err.message || '保存邮件配置失败', true);
        } finally {
            emailSaving.value = false;
        }
    }

    /** 发送测试邮件；密码框有输入时优先用输入值（不落库）。 */
    async function testEmail() {
        emailTesting.value = true;
        try {
            await sendTestEmail({ to: emailForm.to, password: emailForm.password });
            ctx.toastNotify('测试邮件已发送，请查收');
        } catch (err) {
            ctx.toastNotify(err.message || '测试邮件发送失败', true);
        } finally {
            emailTesting.value = false;
        }
    }

    // 30s 轮询未读数
    setInterval(refreshUnread, 30000);
    refreshUnread();

    Object.assign(ctx, {
        notifications, unreadCount, notifyPanelVisible, notifyLoading,
        notifDetailVisible, notifDetail,
        loadNotifications, toggleNotifyPanel, closeNotifyPanel,
        markAllRead, clearAllNotifications, openNotification, closeNotifDetail,
        notifyTypeLabel, notifyModuleLabel, formatNotifyTime,
        ncFilterRead, ncFilterLevel, ncFilterModule, ncModuleOptions, ncFiltered,
        openNotifyCenter,
        emailForm, emailHasPassword, emailSaving, emailTesting,
        loadEmailConfig, saveEmailConfig, testEmail,
    });
}
