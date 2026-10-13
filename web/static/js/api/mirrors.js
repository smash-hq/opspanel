/**
 * mirrors.js — 镜像源管理 API（Docker 镜像源 + 服务器镜像配置）
 */

import { request } from './http.js';

/** 获取镜像源列表。 */
export async function fetchDockerSources() {
    const json = await request('/docker-sources');
    if (json.code !== 0) throw new Error(json.message || '加载镜像源失败');
    return json.data || [];
}

/** 新增镜像源。 */
export async function createDockerSource(payload) {
    const json = await request('/docker-sources', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(payload),
    });
    if (json.code !== 0) throw new Error(json.message || '创建失败');
    return json.data;
}

/** 更新镜像源。 */
export async function updateDockerSource(id, payload) {
    const json = await request(`/docker-sources/${id}`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(payload),
    });
    if (json.code !== 0) throw new Error(json.message || '更新失败');
    return json.data;
}

/** 删除镜像源。 */
export async function deleteDockerSource(id) {
    const json = await request(`/docker-sources/${id}`, { method: 'DELETE' });
    if (json.code !== 0) throw new Error(json.message || '删除失败');
}

/** 设置默认镜像源。 */
export async function setDefaultDockerSource(id) {
    const json = await request(`/docker-sources/${id}`, { method: 'POST' });
    if (json.code !== 0) throw new Error(json.message || '设置失败');
    return json.data;
}

/** 获取服务器当前镜像源配置。 */
export async function fetchServerMirror(serverId) {
    const json = await request(`/docker/mirror?server=${encodeURIComponent(serverId)}`);
    if (json.code !== 0) throw new Error(json.message || '获取镜像源失败');
    return json.data;
}

/** 应用镜像源到服务器。 */
export async function applyServerMirror(serverId, mirror) {
    const json = await request('/docker/mirror?server=' + encodeURIComponent(serverId), {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ mirror }),
    });
    if (json.code !== 0) throw new Error(json.message || '应用失败');
    return json.data;
}
