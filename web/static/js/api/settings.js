/**
 * settings.js — 系统设置 API
 *
 * 设置读写、网站图标管理与快照推送暂停开关。
 */

import { request } from './http.js';

/** 读取系统设置。 */
export function getSettings() {
    return request('/settings/');
}

/** 保存系统设置。 */
export function updateSettings(form) {
    return request('/settings/', {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(form),
    });
}

/** 上传网站图标（原始二进制）。 */
export function uploadFaviconFile(file) {
    return request('/settings/favicon', { method: 'POST', body: file });
}

/** 恢复默认网站图标。 */
export function resetFaviconFile() {
    return request('/settings/favicon', { method: 'DELETE' });
}

/** 暂停/恢复系统快照推送。 */
export function setPushPaused(paused) {
    return request('/settings/push', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ paused }),
    });
}
