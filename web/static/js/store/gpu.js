/**
 * gpu.js — GPU/NPU 信息域
 *
 * 服务器详情「GPU」tab：实时拉取国产化 GPU（昇腾 npu-smi / 寒武纪 cnmon）快照，
 * 以卡片展示每卡的型号、健康、功耗、温度、核心利用率与显存。
 * 手动刷新节流 3s，避免频繁 SSH 拉取。
 */

import { fetchGpuSnapshot } from '../api/gpu.js';

const { ref, computed } = window.Vue;

const REFRESH_MIN_MS = 3000;

export function setupGpu(ctx) {
    const gpuSnapshot = ref(null);
    const gpuLoading = ref(false);
    const gpuError = ref('');
    const gpuDevices = computed(() => (gpuSnapshot.value && gpuSnapshot.value.devices) || []);

    let lastFetchAt = 0;

    async function loadGpu(force = false) {
        if (!ctx.selectedServer.value) return;
        const now = Date.now();
        if (!force && now - lastFetchAt < REFRESH_MIN_MS) return;
        lastFetchAt = now;
        gpuLoading.value = true;
        gpuError.value = '';
        try {
            gpuSnapshot.value = await fetchGpuSnapshot(ctx.selectedServer.value);
        } catch (err) {
            gpuError.value = err.message;
        } finally {
            gpuLoading.value = false;
        }
    }

    // ─── 展示辅助 ───
    function gpuNum(v) {
        const n = parseFloat(v);
        return Number.isFinite(n) ? n : null;
    }

    function gpuPercent(v) {
        const n = gpuNum(v);
        return n === null ? 0 : Math.min(100, Math.max(0, n));
    }

    function gpuMemPercent(dev) {
        const used = gpuNum(dev.mem_used_mb);
        const total = gpuNum(dev.mem_total_mb);
        if (used === null || !total) return 0;
        return Math.min(100, Math.max(0, (used / total) * 100));
    }

    function gpuMemText(dev) {
        if (dev.mem_used_mb === '' || dev.mem_total_mb === '') return '—';
        return `${dev.mem_used_mb} / ${dev.mem_total_mb} MB`;
    }

    function gpuHealthClass(health) {
        const h = String(health || '').toLowerCase();
        if (!h || h === '-') return 'unknown';
        if (h === 'ok' || h.includes('ok') || h.includes('健康') || h.includes('正常')) return 'ok';
        return 'bad';
    }

    function gpuUtilClass(v) {
        const n = gpuNum(v);
        if (n === null) return '';
        if (n >= 85) return 'hot';
        if (n >= 60) return 'warm';
        return '';
    }

    function gpuTempClass(v) {
        const n = gpuNum(v);
        if (n === null) return '';
        if (n >= 80) return 'hot';
        if (n >= 60) return 'warm';
        return '';
    }

    Object.assign(ctx, {
        gpuSnapshot, gpuLoading, gpuError, gpuDevices, loadGpu,
        gpuPercent, gpuMemPercent, gpuMemText, gpuHealthClass, gpuUtilClass, gpuTempClass,
    });
}
