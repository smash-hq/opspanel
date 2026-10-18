/**
 * services.js — 系统服务（systemd）管理 API
 */

import { request } from './http.js';

/** 获取 systemd 服务列表。 */
export async function fetchServices(serverId) {
    const params = new URLSearchParams({ server: serverId });
    const json = await request(`/services?${params}`);
    if (json.code !== 0) throw new Error(json.message || '获取服务列表失败');
    return json.data || [];
}

/** 执行服务操作（start/stop/restart/enable/disable）。 */
export async function serviceAction(serverId, name, action) {
    const json = await request('/services/action', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ server_id: serverId, name, action }),
    });
    if (json.code !== 0) throw new Error(json.message || '服务操作失败');
    return json.data;
}

/** 获取服务日志（journalctl 优先）。 */
export async function fetchServiceLog(serverId, name, lines) {
    const params = new URLSearchParams({ server: serverId, name, lines: lines || 100 });
    const json = await request(`/services/logs?${params}`);
    if (json.code !== 0) throw new Error(json.message || '获取服务日志失败');
    return json.data;
}

/** 读取服务 unit 文件，返回 { path, content }；path 为空表示无独立单元文件。 */
export async function fetchUnitFile(serverId, name) {
    const params = new URLSearchParams({ server: serverId, name });
    const json = await request(`/services/unit?${params}`);
    if (json.code !== 0) throw new Error(json.message || '读取 unit 文件失败');
    return json.data || { path: '', content: '' };
}

/** 保存服务 unit 文件（自动备份 *.bak 并 daemon-reload）。 */
export async function saveUnitFile(serverId, name, content) {
    const json = await request('/services/unit', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ server_id: serverId, name, content }),
    });
    if (json.code !== 0) throw new Error(json.message || '保存 unit 文件失败');
    return json.data;
}
