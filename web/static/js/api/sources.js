/**
 * sources.js — 软件源管理 API
 */

import { request } from './http.js';

/** 获取软件源列表。 */
export async function fetchSources() {
    const json = await request('/software/sources');
    return json.data || [];
}

/** 新增软件源。 */
export async function createSource(payload) {
    const json = await request('/software/sources', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(payload),
    });
    return json.data;
}

/** 更新软件源。 */
export async function updateSource(id, payload) {
    const json = await request(`/software/sources/${id}`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(payload),
    });
    return json.data;
}

/** 删除软件源。 */
export async function deleteSource(id) {
    await request(`/software/sources/${id}`, { method: 'DELETE' });
}
