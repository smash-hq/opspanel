/**
 * processes.js — 进程管理 API
 */

import { request } from './http.js';

/** 获取进程列表。 */
export async function fetchProcesses(serverId, lines) {
    const params = new URLSearchParams({ server: serverId, lines: lines || 50 });
    const json = await request(`/processes?${params}`);
    if (json.code !== 0) throw new Error(json.message || '获取进程列表失败');
    return json.data || [];
}

/** 终止进程。 */
export async function killProcess(serverId, pid, force) {
    const json = await request('/processes/kill', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ server_id: serverId, pid, force: force || false }),
    });
    if (json.code !== 0) throw new Error(json.message || '终止进程失败');
    return json.data;
}

/** 获取进程日志。 */
export async function fetchProcessLog(serverId, pid, lines) {
    const params = new URLSearchParams({ server: serverId, pid, lines: lines || 50 });
    const json = await request(`/processes/log?${params}`);
    if (json.code !== 0) throw new Error(json.message || '获取进程日志失败');
    return json.data;
}
