/**
 * files.js — 文件管理 API
 */

import { request, API_PREFIX, notifyUnauthorized } from './http.js';

/** 列出目录内容。 */
export async function fetchFiles(serverId, path) {
    const params = new URLSearchParams({ server: serverId, path: path || '/' });
    const json = await request(`/files?${params}`);
    if (json.code !== 0) throw new Error(json.message || '加载文件列表失败');
    // 新格式: { files: [...], sudo_used: bool }
    if (json.data && Array.isArray(json.data.files)) {
        return json.data;
    }
    // 兼容旧格式
    return { files: json.data || [], sudo_used: false };
}

/** 获取文件详情。 */
export async function fetchFileInfo(serverId, path) {
    const params = new URLSearchParams({ server: serverId, path });
    const json = await request(`/files/info?${params}`);
    if (json.code !== 0) throw new Error(json.message || '获取文件详情失败');
    return json.data;
}

/** 预览文件内容。 */
export async function fetchFilePreview(serverId, path, lines) {
    const params = new URLSearchParams({ server: serverId, path, lines: lines || 100 });
    const json = await request(`/files/preview?${params}`);
    if (json.code !== 0) throw new Error(json.message || '预览文件失败');
    return json.data;
}

/** 下载文件。 */
export async function downloadFile(serverId, path) {
    const params = new URLSearchParams({ server: serverId, path });
    const response = await fetch(`${API_PREFIX}/files/download?${params}`);
    if (response.status === 401) {
        notifyUnauthorized();
        throw new Error('未登录或会话已过期');
    }
    if (!response.ok) throw new Error('下载失败');
    const blob = await response.blob();
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    // 优先使用后端 Content-Disposition 中的文件名（目录下载为 *.tar.gz）
    const cd = response.headers.get('Content-Disposition') || '';
    const m = cd.match(/filename="?([^";]+)"?/);
    a.download = m ? m[1] : (path.replace(/\/+$/, '').split('/').pop() || 'download');
    a.href = url;
    a.click();
    URL.revokeObjectURL(url);
}
