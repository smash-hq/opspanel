/**
 * packages.js — 软件包管理 API
 */

import { request } from './http.js';

/** 获取软件包列表。 */
export async function fetchPackages() {
    const json = await request('/software/packages');
    return json.data || [];
}

/** 新增软件包。 */
export async function createPackage(payload) {
    const json = await request('/software/packages', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(payload),
    });
    return json.data;
}

/** 更新软件包。 */
export async function updatePackage(id, payload) {
    const json = await request(`/software/packages/${id}`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(payload),
    });
    return json.data;
}

/** 删除软件包。 */
export async function deletePackage(id) {
    await request(`/software/packages/${id}`, { method: 'DELETE' });
}
