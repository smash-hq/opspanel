/**
 * docker.js — Docker 管理 API
 */

import { request } from './http.js';

/** 检测 Docker 环境。 */
export async function checkDocker(serverId) {
    const json = await request(`/docker?server=${encodeURIComponent(serverId)}`);
    return json.data;
}

/** 获取镜像列表。 */
export async function fetchDockerImages(serverId) {
    const json = await request(`/docker/images?server=${encodeURIComponent(serverId)}`);
    return json.data || [];
}

/** 拉取镜像。 */
export async function pullDockerImage(serverId, image) {
    const json = await request('/docker/images/pull', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ server_id: serverId, image }),
    });
    return json.data;
}

/** 删除镜像（危险操作，需用户确认后携带 confirm 标记）。 */
export async function removeDockerImage(serverId, id) {
    await request(`/docker/images/${encodeURIComponent(id)}?server=${encodeURIComponent(serverId)}&confirm=true`, { method: 'DELETE' });
}

/** 一键启动容器（基于已有镜像）。 */
export async function runDockerContainer(serverId, payload) {
    const json = await request('/docker/run', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ server_id: serverId, ...payload }),
    });
    return json.data;
}

/** 获取容器列表。 */
export async function fetchDockerContainers(serverId, all) {
    const params = new URLSearchParams({ server: serverId, all: all ? 'true' : 'false' });
    const json = await request(`/docker/containers?${params}`);
    return json.data || [];
}

/** 容器操作（stop/restart/kill 为危险操作，需携带 confirm 标记）。 */
export async function dockerContainerAction(serverId, id, action) {
    const confirm = action === 'start' ? '' : '&confirm=true';
    const json = await request(`/docker/containers/${encodeURIComponent(id)}/${action}?server=${encodeURIComponent(serverId)}${confirm}`, { method: 'POST' });
    return json.data;
}

/** 删除容器（危险操作，需用户确认后携带 confirm 标记）。 */
export async function removeDockerContainer(serverId, id) {
    await request(`/docker/containers/${encodeURIComponent(id)}?server=${encodeURIComponent(serverId)}&confirm=true`, { method: 'DELETE' });
}

/** 在容器内执行命令。 */
export async function dockerExec(serverId, containerId, command) {
    const json = await request('/docker/exec', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ server_id: serverId, container_id: containerId, command }),
    });
    return json.data;
}

/** 获取容器日志。 */
export async function fetchDockerLogs(serverId, id, lines) {
    const params = new URLSearchParams({ server: serverId, id, lines: lines || 100 });
    const json = await request(`/docker/logs?${params}`);
    return json.data;
}

/** 批量删除镜像（危险操作，需用户确认后携带 confirm 标记）。 */
export async function batchDeleteDockerImages(serverId, ids) {
    const json = await request('/docker/images/batch-delete', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ server_id: serverId, ids, confirm: true }),
    });
    return json.data;
}

/** 批量容器操作（危险操作，需用户确认后携带 confirm 标记）。 */
export async function batchDockerContainerAction(serverId, ids, action) {
    const json = await request('/docker/containers/batch', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ server_id: serverId, ids, action, confirm: true }),
    });
    return json.data;
}
