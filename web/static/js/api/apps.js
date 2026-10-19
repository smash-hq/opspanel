/**
 * apps.js — 应用部署管理 API
 */

import { request, API_PREFIX, notifyUnauthorized } from './http.js';

/** 获取部署应用列表（serverId 可选，refresh=1 时后端核实进程状态）。 */
export async function fetchApps(serverId, refresh) {
    const params = new URLSearchParams();
    if (serverId) params.set('server', serverId);
    if (refresh) params.set('refresh', '1');
    const qs = params.toString();
    const json = await request(`/apps${qs ? '?' + qs : ''}`);
    return json.data || [];
}

/** 创建部署应用。 */
export async function createApp(payload) {
    const json = await request('/apps', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(payload),
    });
    return json.data;
}

/** 更新部署应用。 */
export async function updateApp(id, payload) {
    const json = await request(`/apps/${encodeURIComponent(id)}`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(payload),
    });
    return json.data;
}

/** 删除部署应用（危险操作，需用户确认后携带 confirm 标记；运行中的进程会被一并停止）。 */
export async function deleteApp(id) {
    await request(`/apps/${encodeURIComponent(id)}?confirm=true`, { method: 'DELETE' });
}

/**
 * 创建分片上传会话，返回 { key, chunk_size }。
 * @param {string} [key] 可选稳定 key：同一应用同一文件重试时复用已落盘分片（断点续传）。
 */
export async function initAppUpload(appId, relPath, size, key) {
    const json = await request('/apps/upload/init', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ app_id: appId, rel_path: relPath, size, key: key || '' }),
    });
    return json.data;
}

/** 查询已落盘分片索引（断点续传依据），返回 { received: number[] }。 */
export async function fetchAppUploadStatus(appId, key) {
    const json = await request(`/apps/upload/status?key=${encodeURIComponent(key)}&app_id=${encodeURIComponent(appId)}`);
    return json.data;
}

/**
 * 用 XMLHttpRequest 上传单个分片（fetch 不支持上传进度）。
 * @param {string} appId 应用 ID
 * @param {string} key 上传会话 key
 * @param {number} index 分片序号
 * @param {Blob} blob 分片数据
 * @param {(loaded: number) => void} [onProgress] 已发送字节回调
 */
export function uploadAppChunk(appId, key, index, blob, onProgress) {
    return new Promise((resolve, reject) => {
        const form = new FormData();
        form.append('app_id', appId);
        form.append('key', key);
        form.append('index', String(index));
        form.append('file', blob, `chunk-${index}.part`);
        const xhr = new XMLHttpRequest();
        xhr.open('POST', `${API_PREFIX}/apps/upload/chunk`);
        xhr.upload.onprogress = (e) => {
            if (e.lengthComputable && onProgress) onProgress(e.loaded);
        };
        xhr.onload = () => {
            if (xhr.status === 401) {
                notifyUnauthorized();
                return reject(new Error('未登录或会话已过期'));
            }
            let json = null;
            try { json = JSON.parse(xhr.responseText); } catch { /* ignore */ }
            if (xhr.status >= 200 && xhr.status < 300 && json && json.code === 0) {
                resolve(json.data);
            } else {
                reject(new Error((json && json.message) || `分片上传失败（HTTP ${xhr.status}）`));
            }
        };
        xhr.onerror = () => reject(new Error('网络异常，分片上传中断'));
        xhr.send(form); // FormData 自动带 boundary，勿手动设置 Content-Type
    });
}

/** 通知后端合并分片为最终文件，返回 { path, size }。 */
export async function completeAppUpload(appId, key, relPath, total, size) {
    const json = await request('/apps/upload/complete', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ app_id: appId, key, rel_path: relPath, total, size }),
    });
    return json.data;
}

/** 应用生命周期操作：start / stop / restart（stop/restart 为危险操作，需携带 confirm 标记）。 */
export async function appAction(id, action) {
    const qs = action === 'start' ? '' : '?confirm=true';
    const json = await request(`/apps/${encodeURIComponent(id)}/${action}${qs}`, { method: 'POST' });
    return json.data;
}

/** 查看应用日志末尾。 */
export async function fetchAppLogs(id, lines) {
    const json = await request(`/apps/${encodeURIComponent(id)}/logs?lines=${lines || 200}`);
    return json.data;
}
