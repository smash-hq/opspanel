/**
 * servers.js — 服务器管理 API
 */

import { request } from './http.js';

/**
 * 获取服务器列表。
 * @returns {Promise<Array>} 服务器配置数组
 */
export async function fetchServers() {
    const json = await request('/servers');
    return json.data || [];
}

/**
 * 新增服务器。
 * @param {Object} payload - 服务器配置 { name, type, host, port, username, auth_type, password, key_path }
 * @returns {Object} 创建后的服务器对象（含 id）
 */
export async function createServer(payload) {
    const json = await request('/servers', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(payload),
    });
    return json.data;
}

/**
 * 更新服务器配置。
 * @param {string} id - 服务器 ID
 * @param {Object} payload - 更新的字段
 * @returns {Object} 更新后的服务器对象
 */
export async function updateServer(id, payload) {
    const json = await request(`/servers/${id}`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(payload),
    });
    return json.data;
}

/**
 * 删除服务器（危险操作，需用户确认后携带 confirm 标记）。
 * @param {string} id - 服务器 ID
 * @returns {void}
 */
export async function deleteServer(id) {
    await request(`/servers/${id}?confirm=true`, { method: 'DELETE' });
}

/**
 * 采集指定服务器的系统快照。
 * @param {string} serverId - 服务器 ID（"local" 表示本机）
 * @returns {Object} 系统信息快照
 */
export async function loadServerInfo(serverId) {
    const json = await request(`/system/info?server=${encodeURIComponent(serverId)}`);
    return json.data;
}

/**
 * 测试远程服务器连接（不保存配置）。
 * @param {Object} payload - 服务器配置 { name, type, host, port, username, auth_type, password, key_path }
 * @returns {Object} { success: boolean, host?: { hostname, os, ... }, error?: string }
 */
export async function testConnection(payload) {
    const json = await request('/servers/test', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(payload),
    });
    return json.data;
}
