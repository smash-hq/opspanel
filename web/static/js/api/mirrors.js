/**
 * mirrors.js — 镜像源管理 API（Docker 镜像源 + 服务器镜像配置）
 */

import { request } from './http.js';

/** 获取镜像源列表。 */
export async function fetchDockerSources() {
    const json = await request('/docker-sources');
    return json.data || [];
}

/** 新增镜像源。 */
export async function createDockerSource(payload) {
    const json = await request('/docker-sources', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(payload),
    });
    return json.data;
}

/** 更新镜像源。 */
export async function updateDockerSource(id, payload) {
    const json = await request(`/docker-sources/${id}`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(payload),
    });
    return json.data;
}

/** 删除镜像源。 */
export async function deleteDockerSource(id) {
    await request(`/docker-sources/${id}`, { method: 'DELETE' });
}

/** 设置默认镜像源。 */
export async function setDefaultDockerSource(id) {
    const json = await request(`/docker-sources/${id}`, { method: 'POST' });
    return json.data;
}

/** 获取服务器当前镜像源配置。 */
export async function fetchServerMirror(serverId) {
    const json = await request(`/docker/mirror?server=${encodeURIComponent(serverId)}`);
    return json.data;
}

/** 应用镜像源到服务器（会重启 Docker，需用户确认后携带 confirm 标记）。 */
export async function applyServerMirror(serverId, mirror) {
    const json = await request('/docker/mirror?server=' + encodeURIComponent(serverId), {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ mirror, confirm: true }),
    });
    return json.data;
}
