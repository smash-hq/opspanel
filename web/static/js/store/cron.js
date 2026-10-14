/**
 * cron.js — 定时任务管理域
 *
 * 管理目标服务器 crontab 中的定时任务：列表、新增、编辑、启停、删除。
 * 任务由服务器系统 cron 守护进程执行，本服务仅负责 crontab 的读写。
 * 表单提供主流的「执行周期选择器」：按类型生成 5 字段 cron 表达式，支持自定义。
 */

import {
    fetchCronJobs, createCronJob, updateCronJob, deleteCronJob, previewCronExpr,
} from '../api/cron.js';

const { ref, reactive } = window.Vue;

// 执行周期类型
const CRON_KINDS = [
    { key: 'minutes', label: '每 N 分钟' },
    { key: 'hourly', label: '每小时' },
    { key: 'daily', label: '每天' },
    { key: 'weekly', label: '每周' },
    { key: 'monthly', label: '每月' },
    { key: 'reboot', label: '开机执行' },
    { key: 'custom', label: '自定义' },
];

const CRON_WEEKDAYS = [
    { value: '0', label: '周日' },
    { value: '1', label: '周一' },
    { value: '2', label: '周二' },
    { value: '3', label: '周三' },
    { value: '4', label: '周四' },
    { value: '5', label: '周五' },
    { value: '6', label: '周六' },
];

function clampInt(v, lo, hi, dflt) {
    const n = parseInt(v, 10);
    if (Number.isNaN(n)) return dflt;
    return Math.min(hi, Math.max(lo, n));
}

function pad2(n) {
    return String(n).padStart(2, '0');
}

export function setupCron(ctx) {
    // ─── 任务列表 ───
    const cronJobs = ref([]);
    const cronLoading = ref(false);
    const cronBusyId = ref('');

    async function loadCronJobs() {
        if (!ctx.selectedServer.value) return;
        cronLoading.value = true;
        try {
            cronJobs.value = await fetchCronJobs(ctx.selectedServer.value);
        } catch (err) {
            ctx.toastNotify('定时任务加载失败: ' + err.message, true);
        } finally {
            cronLoading.value = false;
        }
    }

    // ─── 任务表单（执行周期选择器 + 命令） ───
    const cronFormVisible = ref(false);
    const cronFormSaving = ref(false);
    const cronForm = reactive({
        id: '', name: '', command: '',
        kind: 'minutes', interval: 5, minute: 0,
        time: '03:00', week: '1', monthDay: 1,
        expr: '*/5 * * * *',
    });
    const cronPreview = reactive({ valid: true, next: [], error: '', loading: false });

    // 按表单字段生成 5 字段表达式；自定义类型保留手输内容
    function cronExprFromForm() {
        const [h, m] = String(cronForm.time || '00:00').split(':').map((v) => parseInt(v, 10) || 0);
        switch (cronForm.kind) {
            case 'minutes': return `*/${clampInt(cronForm.interval, 1, 59, 5)} * * * *`;
            case 'hourly': return `${clampInt(cronForm.minute, 0, 59, 0)} * * * *`;
            case 'daily': return `${m} ${h} * * *`;
            case 'weekly': return `${m} ${h} * * ${cronForm.week}`;
            case 'monthly': return `${m} ${h} ${clampInt(cronForm.monthDay, 1, 31, 1)} * *`;
            case 'reboot': return '@reboot';
            default: return cronForm.expr;
        }
    }

    // 类型/参数变化时重新生成表达式并刷新预览
    function updateCronSchedule() {
        if (cronForm.kind === 'custom') return;
        cronForm.expr = cronExprFromForm();
        refreshCronPreview();
    }

    function applyCronKind(kind) {
        cronForm.kind = kind;
        updateCronSchedule();
    }

    // 将已有表达式尽力还原为表单字段；解析不了则落入自定义
    function parseExprToForm(expr) {
        if (expr.startsWith('@')) return { kind: 'reboot' };
        const p = expr.trim().split(/\s+/);
        if (p.length !== 5) return { kind: 'custom' };
        const [mi, h, dom, mon, dow] = p;
        const time = /^\d+$/.test(h) && /^\d+$/.test(mi) ? `${pad2(+h)}:${pad2(+mi)}` : null;
        if (mon === '*' && dom === '*' && dow === '*' && h === '*') {
            const m = mi.match(/^\*\/(\d+)$/);
            if (m) return { kind: 'minutes', interval: clampInt(m[1], 1, 59, 5) };
            if (/^\d+$/.test(mi)) return { kind: 'hourly', minute: clampInt(mi, 0, 59, 0) };
        }
        if (time && mon === '*' && dom === '*' && dow === '*') return { kind: 'daily', time };
        if (time && mon === '*' && dom === '*' && /^\d+$/.test(dow)) return { kind: 'weekly', time, week: dow };
        if (time && mon === '*' && dow === '*' && /^\d+$/.test(dom)) return { kind: 'monthly', time, monthDay: clampInt(dom, 1, 31, 1) };
        return { kind: 'custom' };
    }

    function openCronForm(job) {
        if (job) {
            const parsed = parseExprToForm(job.expr);
            Object.assign(cronForm, {
                id: job.id, name: job.name, command: job.command,
                kind: parsed.kind, interval: 5, minute: 0,
                time: '03:00', week: '1', monthDay: 1,
                expr: job.expr,
            }, parsed);
        } else {
            Object.assign(cronForm, {
                id: '', name: '', command: '',
                kind: 'minutes', interval: 5, minute: 0,
                time: '03:00', week: '1', monthDay: 1,
                expr: '*/5 * * * *',
            });
        }
        cronPreview.valid = true;
        cronPreview.next = [];
        cronPreview.error = '';
        cronFormVisible.value = true;
        refreshCronPreview();
    }

    function closeCronForm() {
        cronFormVisible.value = false;
    }

    let previewTimer = null;
    function refreshCronPreview() {
        clearTimeout(previewTimer);
        previewTimer = setTimeout(async () => {
            const expr = cronForm.expr.trim();
            if (!expr) {
                cronPreview.valid = false;
                cronPreview.next = [];
                cronPreview.error = '表达式不能为空';
                return;
            }
            cronPreview.loading = true;
            try {
                const data = await previewCronExpr(expr);
                cronPreview.valid = !!data.valid;
                cronPreview.next = data.next || [];
                cronPreview.error = data.error || '';
            } catch (err) {
                cronPreview.valid = false;
                cronPreview.error = err.message;
            } finally {
                cronPreview.loading = false;
            }
        }, 300);
    }

    async function submitCronForm() {
        if (!cronForm.command.trim()) return ctx.toastNotify('请填写执行命令', true);
        if (!cronPreview.valid) return ctx.toastNotify('cron 表达式无效: ' + cronPreview.error, true);

        const payload = {
            server_id: ctx.selectedServer.value,
            name: cronForm.name.trim(),
            expr: cronForm.expr.trim(),
            command: cronForm.command.trim(),
        };
        cronFormSaving.value = true;
        try {
            if (cronForm.id) {
                await updateCronJob(cronForm.id, payload);
                ctx.toastNotify('任务已更新');
            } else {
                await createCronJob(payload);
                ctx.toastNotify('任务已写入 crontab');
            }
            cronFormVisible.value = false;
            loadCronJobs();
        } catch (err) {
            ctx.toastNotify(err.message, true);
        } finally {
            cronFormSaving.value = false;
        }
    }

    // ─── 启停切换 / 删除 ───
    async function toggleCronJob(job) {
        cronBusyId.value = job.id;
        try {
            await updateCronJob(job.id, {
                server_id: ctx.selectedServer.value,
                enabled: !job.enabled,
            });
            ctx.toastNotify(job.enabled ? '任务已停用（注释该行）' : '任务已启用');
            loadCronJobs();
        } catch (err) {
            ctx.toastNotify('切换失败: ' + err.message, true);
        } finally {
            cronBusyId.value = '';
        }
    }

    async function handleDeleteCronJob(job) {
        const label = job.name || job.expr + ' ' + job.command;
        if (!await ctx.confirmDialog(`确定从服务器 crontab 中删除任务「${label}」吗？`, { title: '删除任务', confirmText: '删除' })) return;
        cronBusyId.value = job.id;
        try {
            await deleteCronJob(job.id, ctx.selectedServer.value);
            ctx.toastNotify('任务已删除');
            loadCronJobs();
        } catch (err) {
            ctx.toastNotify('删除失败: ' + err.message, true);
        } finally {
            cronBusyId.value = '';
        }
    }

    // ─── 展示辅助 ───
    function cronExprText(job) {
        return job.expr;
    }

    function cronNextText(job) {
        return job.next_runs && job.next_runs.length ? job.next_runs[0] : '—';
    }

    Object.assign(ctx, {
        cronJobs, cronLoading, cronBusyId,
        cronFormVisible, cronFormSaving, cronForm, cronPreview,
        cronKinds: CRON_KINDS, cronWeekdays: CRON_WEEKDAYS,
        loadCronJobs, openCronForm, closeCronForm, applyCronKind, updateCronSchedule, refreshCronPreview, submitCronForm,
        toggleCronJob, handleDeleteCronJob,
        cronExprText, cronNextText,
    });
}
