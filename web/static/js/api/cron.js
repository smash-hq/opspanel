/**
 * cron.js — 定时任务管理 API
 *
 * 数据源为目标服务器的 crontab，任务由系统 cron 守护进程执行。
 */

import { request } from './http.js';

/** 获取指定节点的 crontab 任务列表。 */
export async function fetchCronJobs(serverId) {
    const params = new URLSearchParams({ server_id: serverId });
    const json = await request(`/cron/jobs?${params}`);
    if (json.code !== 0) throw new Error(json.message || '获取定时任务失败');
    return json.data || [];
}

/** 创建定时任务（写回服务器 crontab）。 */
export async function createCronJob(payload) {
    const json = await request('/cron/jobs', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(payload),
    });
    if (json.code !== 0) throw new Error(json.message || '创建定时任务失败');
    return json.data;
}

/** 更新定时任务（空字段保留原值，enabled 传布尔即切换启停）。 */
export async function updateCronJob(id, payload) {
    const params = new URLSearchParams({ server_id: payload.server_id });
    const json = await request(`/cron/jobs/${id}?${params}`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(payload),
    });
    if (json.code !== 0) throw new Error(json.message || '更新定时任务失败');
    return json.data;
}

/** 删除定时任务（从服务器 crontab 移除）。 */
export async function deleteCronJob(id, serverId) {
    const params = new URLSearchParams({ server_id: serverId });
    const json = await request(`/cron/jobs/${id}?${params}`, { method: 'DELETE' });
    if (json.code !== 0) throw new Error(json.message || '删除定时任务失败');
    return json.data;
}

/** 预览 cron 表达式后续执行时间。 */
export async function previewCronExpr(expr) {
    const json = await request('/cron/preview', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ expr }),
    });
    if (json.code !== 0) throw new Error(json.message || '表达式解析失败');
    return json.data;
}
