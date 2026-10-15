/**
 * notifications.js — 站内通知 API
 */

import { request } from './http.js';

/** 获取通知列表，返回 { items, unread }。 */
export async function fetchNotifications(limit = 50) {
    const json = await request(`/notifications?limit=${limit}`);
    return json.data || { items: [], unread: 0 };
}

/** 全部标记已读。 */
export async function markNotificationsRead() {
    const json = await request('/notifications/read', { method: 'POST' });
    return json.data || { unread: 0 };
}

/** 标记单条已读。 */
export async function markNotificationRead(id) {
    const json = await request(`/notifications/${encodeURIComponent(id)}/read`, { method: 'POST' });
    return json.data || { unread: 0 };
}

/** 清空全部通知。 */
export async function clearNotifications() {
    const json = await request('/notifications', { method: 'DELETE' });
    return json.data;
}

/** 获取邮件（SMTP）配置，返回 { config, has_password }。密码已脱敏为空。 */
export async function fetchEmailConfig() {
    const json = await request('/notifications/email-config');
    return json.data || { config: {}, has_password: false };
}

/** 保存邮件（SMTP）配置；password 为空表示沿用已存密码。 */
export async function updateEmailConfig(config) {
    const json = await request('/notifications/email-config', {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(config),
    });
    return json.data;
}

/** 发送测试邮件；opts.password 非空时用于本次测试（不落库）。 */
export async function sendTestEmail(opts = {}) {
    const json = await request('/notifications/email-test', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(opts),
    });
    return json.data;
}
