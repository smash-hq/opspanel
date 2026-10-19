/**
 * gpu.js — GPU/NPU 信息 API
 *
 * 实时按需拉取：后端经 SSH 探测厂商工具并解析全量卡信息，无 agent。
 */

import { request } from './http.js';

/** 获取指定节点的 GPU 快照（vendor/vendor_name/devices/error）。 */
export async function fetchGpuSnapshot(serverId) {
    const params = new URLSearchParams({ server_id: serverId });
    const json = await request(`/gpu?${params}`);
    return json.data || { vendor: '', devices: [] };
}
