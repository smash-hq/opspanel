/**
 * files.js — 文件管理 API
 */
import { t } from '../i18n/index.js';


import { request, API_PREFIX, notifyUnauthorized, ApiError } from './http.js';

/** 列出目录内容。 */
export async function fetchFiles(serverId, path) {
    const params = new URLSearchParams({ server: serverId, path: path || '/' });
    const json = await request(`/files?${params}`);
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
    return json.data;
}

/** 预览文件内容。 */
export async function fetchFilePreview(serverId, path, lines) {
    const params = new URLSearchParams({ server: serverId, path, lines: lines || 100 });
    const json = await request(`/files/preview?${params}`);
    return json.data;
}

/** 下载文件。 */
export async function downloadFile(serverId, path) {
    const params = new URLSearchParams({ server: serverId, path });
    let response;
    try {
        response = await fetch(`${API_PREFIX}/files/download?${params}`);
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
        let msg = t('文件下载失败');
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
    // 优先使用后端 Content-Disposition 中的文件名（目录下载为 *.tar.gz）
    const cd = response.headers.get('Content-Disposition') || '';
    const m = cd.match(/filename="?([^";]+)"?/);
    a.download = m ? m[1] : (path.replace(/\/+$/, '').split('/').pop() || 'download');
    a.href = url;
    a.click();
    URL.revokeObjectURL(url);
}
