/**
 * theme.js — 主题管理域
 *
 * 亮/暗/跟随系统主题切换，持久化到 localStorage 与后端设置。
 */

const { ref } = window.Vue;

import { updateSettings } from '../api/settings.js';

export function setupTheme(ctx) {
    const theme = ref(localStorage.getItem('smp_theme') || 'system');
    applyTheme(theme.value);

    function applyTheme(t) {
        const root = document.documentElement;
        if (t === 'dark') {
            root.dataset.theme = 'dark';
        } else if (t === 'light') {
            root.dataset.theme = 'light';
        } else {
            // 跟随系统
            const dark = window.matchMedia('(prefers-color-scheme: dark)').matches;
            root.dataset.theme = dark ? 'dark' : 'light';
        }
    }

    function setTheme(t) {
        theme.value = t;
        ctx.themeVal.value = t;
        localStorage.setItem('smp_theme', t);
        applyTheme(t);
        // 持久化到后端（尽力而为：本地已生效，失败不打断交互）
        updateSettings({ theme: t }).catch(() => {});
    }

    // 监听系统主题变化
    window.matchMedia('(prefers-color-scheme: dark)').addEventListener('change', (e) => {
        if (theme.value === 'system') {
            document.documentElement.dataset.theme = e.matches ? 'dark' : 'light';
        }
    });

    Object.assign(ctx, {
        theme,
        themeVal: ref(theme.value),
        setTheme,
    });
}
