/**
 * processes.js — 进程管理域
 *
 * 进程列表、终止进程与进程日志查看。
 */

import { fetchProcesses, killProcess, fetchProcessLog } from '../api/processes.js';

const { ref, computed } = window.Vue;

export function setupProcesses(ctx) {
    // ─── 进程管理状态 ───
    const processes = ref([]);
    const processLoading = ref(false);
    const processLogVisible = ref(false);
    const processLogData = ref({ pid: 0, content: '' });
    const processLogLoading = ref(false);
    // 列排序：procSortKey 为 '' 表示按后端默认顺序
    const procSortKey = ref('');
    const procSortDesc = ref(true);

    /** 按 CPU/MEM 排序后的进程列表。 */
    const sortedProcesses = computed(() => {
        if (!procSortKey.value) return processes.value;
        const key = procSortKey.value;
        const dir = procSortDesc.value ? -1 : 1;
        return [...processes.value].sort((a, b) =>
            ((parseFloat(a[key]) || 0) - (parseFloat(b[key]) || 0)) * dir
        );
    });

    /** 点击表头切换排序：首次点击降序（占用最高在前），再次点击切换正/倒序。 */
    function toggleProcSort(key) {
        if (procSortKey.value === key) {
            procSortDesc.value = !procSortDesc.value;
        } else {
            procSortKey.value = key;
            procSortDesc.value = true;
        }
    }

    async function loadProcesses(lines) {
        if (!ctx.selectedServer.value) return; // 本机与远程节点均支持
        processLoading.value = true;
        try {
            processes.value = await fetchProcesses(ctx.selectedServer.value, lines || 50);
        } catch (err) {
            ctx.toastNotify('进程列表加载失败: ' + err.message, true);
        } finally {
            processLoading.value = false;
        }
    }

    async function handleKillProcess(pid, force) {
        if (!ctx.selectedServer.value) return;
        if (!await ctx.confirmDialog(`确定要终止进程 PID: ${pid} 吗？${force ? '（强制终止）' : ''}`, { title: '终止进程', confirmText: '终止' })) return;
        try {
            await killProcess(ctx.selectedServer.value, pid, force);
            ctx.toastNotify(`进程 ${pid} 已终止`);
            loadProcesses();
        } catch (err) {
            ctx.toastNotify('终止失败: ' + err.message, true);
        }
    }

    async function viewProcessLog(pid) {
        if (!ctx.selectedServer.value) return;
        processLogVisible.value = true;
        processLogLoading.value = true;
        processLogData.value = { pid, content: '' };
        try {
            const data = await fetchProcessLog(ctx.selectedServer.value, pid, 50);
            processLogData.value.content = data.log || '暂无日志';
        } catch (err) {
            processLogData.value.content = '获取日志失败: ' + err.message;
        } finally {
            processLogLoading.value = false;
        }
    }

    function closeProcessLog() {
        processLogVisible.value = false;
    }

    Object.assign(ctx, {
        processes, processLoading, processLogVisible, processLogData, processLogLoading,
        procSortKey, procSortDesc, sortedProcesses, toggleProcSort,
        loadProcesses, killProcess: handleKillProcess, handleKillProcess,
        viewProcessLog, closeProcessLog,
    });
}
