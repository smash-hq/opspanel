/**
 * software.js — 软件管理 API
 */

import { request } from './http.js';

/**
 * 获取软件目录（可用软件包列表）。
 * @returns {Array} 软件包定义数组
 */
export async function fetchSoftwareCatalog() {
    const json = await request('/software/catalog');
    return json.data || [];
}

/**
 * 查询远程服务器上所有软件的安装状态。
 * @param {string} serverId - 远程服务器 ID
 * @returns {Array} SoftwareStatus 数组
 */
export async function fetchSoftwareStatus(serverId) {
    const json = await request(`/software/status?server=${encodeURIComponent(serverId)}`);
    return json.data || [];
}

/**
 * 安装软件（异步执行，立即返回）。
 * @param {string} serverId - 远程服务器 ID
 * @param {string} packageId - 软件包 ID
 * @returns {Object} { status: "installing", package: "xxx" }
 */
export async function installSoftware(serverId, packageId, version) {
    const json = await request('/software/install', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ server_id: serverId, package_id: packageId, version: version || '' }),
    });
    return json.data;
}

/**
 * 卸载软件（异步执行，立即返回）。
 * @param {string} serverId - 远程服务器 ID
 * @param {string} packageId - 软件包 ID
 * @returns {Object} { status: "uninstalling", package: "xxx" }
 */
export async function uninstallSoftware(serverId, packageId) {
    const json = await request('/software/uninstall', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ server_id: serverId, package_id: packageId, confirm: true }),
    });
    return json.data;
}

/**
 * 控制软件服务启停。
 * @param {string} serverId - 远程服务器 ID
 * @param {string} packageId - 软件包 ID
 * @param {string} action - "start" | "stop" | "restart"
 * @returns {Object} { status: "ok", action: "xxx" }
 */
export async function controlService(serverId, packageId, action) {
    const json = await request('/software/service', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ server_id: serverId, package_id: packageId, action, confirm: true }),
    });
    return json.data;
}

/** 获取软件运行日志。 */
export async function fetchSoftwareLog(serverId, packageId, lines) {
    const params = new URLSearchParams({ server: serverId, package: packageId, lines: lines || 100 });
    const json = await request(`/software/logs?${params}`);
    return json.data;
}
