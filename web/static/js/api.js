/**
 * api.js — REST API 客户端
 *
 * 封装所有 /api/v1/* 接口调用，返回标准化 Promise。
 * 调用方负责处理 UI 反馈（Toast）。
 */

const API_PREFIX = '/api/v1';

/**
 * 通用 JSON 请求封装。
 * @param {string} path - API 路径（不含前缀）
 * @param {Object} [options] - fetch 选项
 * @returns {Promise<{code: number, message: string, data: any}>}
 * @throws {Error} 网络异常或响应解析失败
 */
async function request(path, options = {}) {
    const response = await fetch(API_PREFIX + path, options);
    return response.json();
}

/**
 * 获取服务器列表。
 * @returns {Promise<Array>} 服务器配置数组
 */
export async function fetchServers() {
    const json = await request('/servers');
    if (json.code !== 0) throw new Error(json.message || '加载服务器列表失败');
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
    if (json.code !== 0) throw new Error(json.message || '创建失败');
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
    if (json.code !== 0) throw new Error(json.message || '更新失败');
    return json.data;
}

/**
 * 删除服务器。
 * @param {string} id - 服务器 ID
 * @returns {void}
 */
export async function deleteServer(id) {
    const json = await request(`/servers/${id}`, { method: 'DELETE' });
    if (json.code !== 0) throw new Error(json.message || '删除失败');
}

/**
 * 采集指定服务器的系统快照。
 * @param {string} serverId - 服务器 ID（"local" 表示本机）
 * @returns {Object} 系统信息快照
 */
export async function loadServerInfo(serverId) {
    const json = await request(`/system/info?server=${encodeURIComponent(serverId)}`);
    if (json.code !== 0) throw new Error(json.message || '获取服务器信息失败');
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
    if (json.code !== 0) throw new Error(json.message || '测试连接失败');
    return json.data;
}

/**
 * 获取软件目录（可用软件包列表）。
 * @returns {Array} 软件包定义数组
 */
export async function fetchSoftwareCatalog() {
    const json = await request('/software/catalog');
    if (json.code !== 0) throw new Error(json.message || '加载软件目录失败');
    return json.data || [];
}

/**
 * 查询远程服务器上所有软件的安装状态。
 * @param {string} serverId - 远程服务器 ID
 * @returns {Array} SoftwareStatus 数组
 */
export async function fetchSoftwareStatus(serverId) {
    const json = await request(`/software/status?server=${encodeURIComponent(serverId)}`);
    if (json.code !== 0) throw new Error(json.message || '查询软件状态失败');
    return json.data || [];
}

/**
 * 安装软件（异步执行，立即返回）。
 * @param {string} serverId - 远程服务器 ID
 * @param {string} packageId - 软件包 ID
 * @returns {Object} { status: "installing", package: "xxx" }
 */
export async function installSoftware(serverId, packageId) {
    const json = await request('/software/install', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ server_id: serverId, package_id: packageId }),
    });
    if (json.code !== 0) throw new Error(json.message || '安装失败');
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
        body: JSON.stringify({ server_id: serverId, package_id: packageId }),
    });
    if (json.code !== 0) throw new Error(json.message || '卸载失败');
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
        body: JSON.stringify({ server_id: serverId, package_id: packageId, action }),
    });
    if (json.code !== 0) throw new Error(json.message || '操作失败');
    return json.data;
}

// --- 软件包管理 ---

/** 获取软件包列表。 */
export async function fetchPackages() {
    const json = await request('/software/packages');
    if (json.code !== 0) throw new Error(json.message || '加载软件包失败');
    return json.data || [];
}

/** 新增软件包。 */
export async function createPackage(payload) {
    const json = await request('/software/packages', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(payload),
    });
    if (json.code !== 0) throw new Error(json.message || '创建失败');
    return json.data;
}

/** 更新软件包。 */
export async function updatePackage(id, payload) {
    const json = await request(`/software/packages/${id}`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(payload),
    });
    if (json.code !== 0) throw new Error(json.message || '更新失败');
    return json.data;
}

/** 删除软件包。 */
export async function deletePackage(id) {
    const json = await request(`/software/packages/${id}`, { method: 'DELETE' });
    if (json.code !== 0) throw new Error(json.message || '删除失败');
}

// --- 镜像源管理 ---

/** 获取软件源列表。 */
export async function fetchSources() {
    const json = await request('/software/sources');
    if (json.code !== 0) throw new Error(json.message || '加载软件源失败');
    return json.data || [];
}

/** 新增软件源。 */
export async function createSource(payload) {
    const json = await request('/software/sources', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(payload),
    });
    if (json.code !== 0) throw new Error(json.message || '创建失败');
    return json.data;
}

/** 更新软件源。 */
export async function updateSource(id, payload) {
    const json = await request(`/software/sources/${id}`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(payload),
    });
    if (json.code !== 0) throw new Error(json.message || '更新失败');
    return json.data;
}

/** 删除软件源。 */
export async function deleteSource(id) {
    const json = await request(`/software/sources/${id}`, { method: 'DELETE' });
    if (json.code !== 0) throw new Error(json.message || '删除失败');
}

// --- 日志查看 ---

/** 获取软件运行日志。 */
export async function fetchSoftwareLog(serverId, packageId, lines) {
    const params = new URLSearchParams({ server: serverId, package: packageId, lines: lines || 100 });
    const json = await request(`/software/logs?${params}`);
    if (json.code !== 0) throw new Error(json.message || '获取日志失败');
    return json.data;
}
