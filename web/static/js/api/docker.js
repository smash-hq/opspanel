/**
 * docker.js — Docker 管理 API
 */

import { request } from './http.js';

/** 检测 Docker 环境。 */
export async function checkDocker(serverId) {
    const json = await request(`/docker?server=${encodeURIComponent(serverId)}`);
    if (json.code !== 0) throw new Error(json.message || '检测失败');
    return json.data;
}

/** 获取镜像列表。 */
export async function fetchDockerImages(serverId) {
    const json = await request(`/docker/images?server=${encodeURIComponent(serverId)}`);
    if (json.code !== 0) throw new Error(json.message || '加载镜像失败');
    return json.data || [];
}

/** 拉取镜像。 */
export async function pullDockerImage(serverId, image) {
    const json = await request('/docker/images/pull', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ server_id: serverId, image }),
    });
    if (json.code !== 0) throw new Error(json.message || '拉取失败');
    return json.data;
}

/** 删除镜像。 */
export async function removeDockerImage(serverId, id) {
    const json = await request(`/docker/images/${encodeURIComponent(id)}?server=${encodeURIComponent(serverId)}`, { method: 'DELETE' });
    if (json.code !== 0) throw new Error(json.message || '删除失败');
}

/** 获取容器列表。 */
export async function fetchDockerContainers(serverId, all) {
    const params = new URLSearchParams({ server: serverId, all: all ? 'true' : 'false' });
    const json = await request(`/docker/containers?${params}`);
    if (json.code !== 0) throw new Error(json.message || '加载容器失败');
    return json.data || [];
}

/** 容器操作。 */
export async function dockerContainerAction(serverId, id, action) {
    const json = await request(`/docker/containers/${encodeURIComponent(id)}/${action}?server=${encodeURIComponent(serverId)}`, { method: 'POST' });
    if (json.code !== 0) throw new Error(json.message || '操作失败');
    return json.data;
}

/** 删除容器。 */
export async function removeDockerContainer(serverId, id) {
    const json = await request(`/docker/containers/${encodeURIComponent(id)}?server=${encodeURIComponent(serverId)}`, { method: 'DELETE' });
    if (json.code !== 0) throw new Error(json.message || '删除失败');
}

/** 在容器内执行命令。 */
export async function dockerExec(serverId, containerId, command) {
    const json = await request('/docker/exec', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ server_id: serverId, container_id: containerId, command }),
    });
    if (json.code !== 0) throw new Error(json.message || '执行失败');
    return json.data;
}

/** 获取容器日志。 */
export async function fetchDockerLogs(serverId, id, lines) {
    const params = new URLSearchParams({ server: serverId, id, lines: lines || 100 });
    const json = await request(`/docker/logs?${params}`);
    if (json.code !== 0) throw new Error(json.message || '获取日志失败');
    return json.data;
}

/** 批量删除镜像。 */
export async function batchDeleteDockerImages(serverId, ids) {
    const json = await request('/docker/images/batch-delete', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ server_id: serverId, ids }),
    });
    if (json.code !== 0) throw new Error(json.message || '批量删除失败');
    return json.data;
}

/** 批量容器操作。 */
export async function batchDockerContainerAction(serverId, ids, action) {
    const json = await request('/docker/containers/batch', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ server_id: serverId, ids, action }),
    });
    if (json.code !== 0) throw new Error(json.message || '批量操作失败');
    return json.data;
}
