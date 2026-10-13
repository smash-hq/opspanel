/**
 * sources.js — 软件源管理 API
 */

import { request } from './http.js';

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
