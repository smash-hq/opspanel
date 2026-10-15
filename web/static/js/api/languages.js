/**
 * languages.js — 编程语言环境 API
 */

import { request } from './http.js';

/** 获取语言环境列表。 */
export async function fetchLanguages() {
    const json = await request('/languages');
    if (json.code !== 0) throw new Error(json.message || '加载语言列表失败');
    return json.data || [];
}

/** 新增语言环境。 */
export async function createLanguage(payload) {
    const json = await request('/languages', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(payload),
    });
    if (json.code !== 0) throw new Error(json.message || '创建失败');
    return json.data;
}

/** 更新语言环境。 */
export async function updateLanguage(id, payload) {
    const json = await request(`/languages/${id}`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(payload),
    });
    if (json.code !== 0) throw new Error(json.message || '更新失败');
    return json.data;
}

/** 删除语言环境。 */
export async function deleteLanguage(id) {
    const json = await request(`/languages/${id}`, { method: 'DELETE' });
    if (json.code !== 0) throw new Error(json.message || '删除失败');
}

/** 检测服务器已安装的语言。 */
export async function detectLanguages(serverId) {
    const json = await request(`/languages/detect?server=${encodeURIComponent(serverId)}`);
    if (json.code !== 0) throw new Error(json.message || '检测失败');
    return json.data || [];
}

/** 安装语言环境。 */
export async function installLanguage(serverId, languageId, version) {
    const json = await request('/languages/install', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ server_id: serverId, language_id: languageId, version: version || '' }),
    });
    if (json.code !== 0) throw new Error(json.message || '安装失败');
    return json.data;
}

/** 卸载语言环境。 */
export async function uninstallLanguage(serverId, languageId) {
    const json = await request('/languages/uninstall', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ server_id: serverId, language_id: languageId }),
    });
    if (json.code !== 0) throw new Error(json.message || '卸载失败');
    return json.data;
}

/** 查询语言安装/卸载任务进度。action: install | uninstall。 */
export async function fetchLanguageJob(serverId, languageId, action) {
    const qs = `server_id=${encodeURIComponent(serverId)}&language_id=${encodeURIComponent(languageId)}&action=${action}`;
    const json = await request(`/languages/job?${qs}`);
    if (json.code !== 0) throw new Error(json.message || '查询任务进度失败');
    return json.data || {};
}
