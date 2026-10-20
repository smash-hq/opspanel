/**
 * docker.js — Docker 管理 API
 */
import { t } from '../i18n/index.js';


import { request, API_PREFIX, notifyUnauthorized, ApiError } from './http.js';

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

/** 查询镜像拉取任务进度（状态 + 实时输出日志）。 */
export async function fetchDockerPullJob(serverId, image) {
    const params = new URLSearchParams({ server: serverId, image });
    const json = await request(`/docker/images/pull/job?${params}`);
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

/** 获取容器日志。 */
export async function fetchDockerLogs(serverId, id, lines) {
    const params = new URLSearchParams({ server: serverId, id, lines: lines || 100 });
    const json = await request(`/docker/logs?${params}`);
    return json.data;
}

/** 列出容器内目录内容（docker exec 实现）。 */
export async function fetchDockerContainerFiles(serverId, container, path) {
    const params = new URLSearchParams({ server: serverId, container, path: path || '/', op: 'list' });
    const json = await request(`/docker/files?${params}`);
    return json.data || { files: [], sudo_used: false };
}

/** 预览容器内文件内容（最多 512KB，二进制文件后端会拒绝）。 */
export async function fetchDockerFilePreview(serverId, container, path) {
    const params = new URLSearchParams({ server: serverId, container, path, op: 'preview' });
    const json = await request(`/docker/files?${params}`);
    return json.data;
}

/** 下载容器内文件（目录为 tar 打包），blob 触发浏览器保存，文件名取 Content-Disposition。 */
export async function downloadDockerFile(serverId, container, path) {
    const params = new URLSearchParams({ server: serverId, container, path, op: 'download' });
    let response;
    try {
        response = await fetch(`${API_PREFIX}/docker/files?${params}`);
    } catch {
        throw new ApiError(t('网络连接失败，文件下载中断'));
    }
    if (response.status === 401) {
        notifyUnauthorized();
        throw new ApiError(t('未登录或会话已过期'));
    }
    if (!response.ok) {
        // 失败时后端返回 JSON 错误信封，透出 message + detail
        let text = '';
        try { text = await response.text(); } catch { /* 忽略读取失败 */ }
        let msg = t('容器文件下载失败');
        let detail = '';
        try {
            const json = JSON.parse(text);
            if (json && json.message) { msg = json.message; detail = json.detail || ''; }
            else detail = text;
        } catch { detail = text; }
        throw new ApiError(msg, detail, response.status);
    }
    const blob = await response.blob();
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    // 优先使用后端 Content-Disposition 中的文件名（目录下载为 *.tar）
    const cd = response.headers.get('Content-Disposition') || '';
    const m = cd.match(/filename="?([^";]+)"?/);
    a.download = m ? m[1] : (path.replace(/\/+$/, '').split('/').pop() || 'download');
    a.href = url;
    a.click();
    URL.revokeObjectURL(url);
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
