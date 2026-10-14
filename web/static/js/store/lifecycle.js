/**
 * lifecycle.js — 生命周期域
 *
 * 挂载/卸载时的全局事件绑定与资源清理、
 * 按 Tab 暂停/恢复系统快照推送、快捷键处理。
 *
 * 必须在其他域之后注册（依赖 ctx 中的方法已就绪）。
 */

const { onMounted, onUnmounted, watch } = window.Vue;

import { setPushPaused } from '../api/settings.js';

export function setupLifecycle(ctx) {
    onMounted(() => {
        ctx.initAuth();
        window.addEventListener('keydown', handleKeydown);
        window.addEventListener('resize', ctx.handleWindowResize);
        window.addEventListener('smp:unauthorized', ctx.handleUnauthorized);
    });

    // 根据活跃 Tab 暂停/恢复系统快照推送（减少不必要的 SSH 连接；尽力而为）
    watch(ctx.activeTab, (tab) => {
        setPushPaused(tab !== 'system').catch(() => {});
    });

    onUnmounted(() => {
        clearTimeout(ctx.toastTimer);
        if (ctx.wsController) ctx.wsController.disconnect();
        ctx.disconnectTerminal();
        window.removeEventListener('keydown', handleKeydown);
        window.removeEventListener('resize', ctx.handleWindowResize);
        window.removeEventListener('smp:unauthorized', ctx.handleUnauthorized);
        document.removeEventListener('click', ctx.handleDocClickForCombo);
    });

    function handleKeydown(event) {
        if (event.key === '/' && document.activeElement?.tagName !== 'INPUT') {
            event.preventDefault();
            document.querySelector('.search-box input')?.focus();
        }
    }
}
