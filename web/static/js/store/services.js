/**
 * services.js — 系统服务（systemd）域
 *
 * 服务列表、启停/重启/开机自启操作与服务日志查看。
 */
import { t } from '../i18n/index.js';


import { fetchServices, serviceAction, fetchServiceLog, fetchUnitFile, saveUnitFile as saveUnitFileApi } from '../api/services.js';

const { ref, computed } = window.Vue;

// 服务运行状态 → 展示文本与徽标样式类（text 存中文源键，读取时经 t() 渲染期翻译）
const STATE_META = {
    active: { text: '运行中', cls: 'ok' },
    activating: { text: '启动中', cls: 'warn' },
    failed: { text: '失败', cls: 'err' },
    inactive: { text: '未运行', cls: 'idle' },
};

export function setupServices(ctx) {
    // ─── 服务管理状态 ───
    const services = ref([]);
    const servicesLoading = ref(false);
    const servicesError = ref('');
    const serviceSearch = ref('');
    const serviceStateFilter = ref(''); // 按运行状态筛选（空 = 全部）
    const serviceLogVisible = ref(false);
    const serviceLogData = ref({ name: '', content: '' });
    const serviceLogLoading = ref(false);
    const serviceLogLines = ref(100);
    const serviceActionPending = ref(''); // 正在执行操作的单元名

    // 关键字（服务名 + 描述）与运行状态双重过滤，不区分大小写
    const filteredServices = computed(() => {
        const kw = serviceSearch.value.trim().toLowerCase();
        const st = serviceStateFilter.value;
        return services.value.filter(s => {
            if (st && s.active_state !== st) return false;
            if (!kw) return true;
            return s.name.toLowerCase().includes(kw) ||
                (s.description || '').toLowerCase().includes(kw);
        });
    });

    // 各运行状态数量统计，用于可点击的状态筛选标签（基于全量列表，不随关键字变化）
    const serviceStateCounts = computed(() => {
        const counts = { active: 0, activating: 0, failed: 0, inactive: 0 };
        for (const s of services.value) {
            if (counts[s.active_state] !== undefined) counts[s.active_state]++;
        }
        return [
            { value: '', label: t('全部'), count: services.value.length },
            { value: 'active', label: t('运行中'), count: counts.active },
            { value: 'activating', label: t('启动中'), count: counts.activating },
            { value: 'failed', label: t('失败'), count: counts.failed },
            { value: 'inactive', label: t('未运行'), count: counts.inactive },
        ];
    });

    function serviceStateText(s) {
        return t((STATE_META[s.active_state] || {}).text) || s.active_state || '-';
    }

    function serviceStateClass(s) {
        return (STATE_META[s.active_state] || {}).cls || 'idle';
    }

    async function loadServices() {
        if (!ctx.selectedServer.value) return; // 本机与远程节点均支持
        servicesLoading.value = true;
        servicesError.value = '';
        try {
            services.value = await fetchServices(ctx.selectedServer.value);
        } catch (err) {
            services.value = [];
            servicesError.value = err.message;
        } finally {
            servicesLoading.value = false;
        }
    }

    async function handleServiceAction(svc, action) {
        if (!ctx.selectedServer.value) return;
        const labels = { start: t('启动'), stop: t('停止'), restart: t('重启'), enable: t('设置开机自启'), disable: t('取消开机自启') };
        if (!await ctx.confirmDialog(t('确定要{p1}服务 {p2} 吗？', { p1: labels[action] || action, p2: svc.name }), { title: t('服务操作'), confirmText: t('确定') })) return;
        serviceActionPending.value = svc.name;
        try {
            await serviceAction(ctx.selectedServer.value, svc.name, action);
            ctx.toastNotify(t('服务 {p1} {p2}成功', { p1: svc.name, p2: labels[action] || action }));
            loadServices();
        } catch (err) {
            ctx.toastNotify(t('{p1}失败: ', { p1: labels[action] || action }) + err.message, true);
        } finally {
            serviceActionPending.value = '';
        }
    }

    async function viewServiceLog(svc) {
        if (!ctx.selectedServer.value) return;
        serviceLogVisible.value = true;
        serviceLogLoading.value = true;
        serviceLogData.value = { name: svc.name, content: '' };
        try {
            const data = await fetchServiceLog(ctx.selectedServer.value, svc.name, serviceLogLines.value);
            serviceLogData.value.content = data.log || t('暂无日志');
        } catch (err) {
            serviceLogData.value.content = t('获取日志失败: ') + err.message;
        } finally {
            serviceLogLoading.value = false;
        }
    }

    function refreshServiceLog() {
        if (serviceLogData.value.name) viewServiceLog({ name: serviceLogData.value.name });
    }

    function closeServiceLog() {
        serviceLogVisible.value = false;
    }

    // ─── unit 文件编辑 ───
    const unitEditVisible = ref(false);
    const unitEditLoading = ref(false);
    const unitEditSaving = ref(false);
    const unitEditData = ref({ name: '', path: '', content: '' });

    async function openUnitEditor(svc) {
        if (!ctx.selectedServer.value) return;
        unitEditVisible.value = true;
        unitEditLoading.value = true;
        unitEditData.value = { name: svc.name, path: '', content: '' };
        try {
            const data = await fetchUnitFile(ctx.selectedServer.value, svc.name);
            if (!data.path) {
                unitEditData.value.content = t('该服务没有独立的 unit 文件（可能由生成器动态生成）。');
                return;
            }
            unitEditData.value.path = data.path;
            unitEditData.value.content = data.content;
        } catch (err) {
            unitEditData.value.content = t('读取 unit 文件失败: ') + err.message;
        } finally {
            unitEditLoading.value = false;
        }
    }

    async function saveUnitFile() {
        if (!ctx.selectedServer.value || unitEditSaving.value) return;
        if (!unitEditData.value.path) {
            ctx.toastNotify(t('该服务没有可保存的 unit 文件'), true);
            return;
        }
        if (!await ctx.confirmDialog(t('确定要保存 {p1} 的 unit 文件吗？保存后将自动执行 systemctl daemon-reload。', { p1: unitEditData.value.name }), { title: t('保存 unit 文件'), confirmText: t('保存') })) return;
        unitEditSaving.value = true;
        try {
            await saveUnitFileApi(ctx.selectedServer.value, unitEditData.value.name, unitEditData.value.content);
            ctx.toastNotify(t('unit 文件已保存，原文件已备份为 *.bak'));
        } catch (err) {
            ctx.toastNotify(t('保存失败: ') + err.message, true);
        } finally {
            unitEditSaving.value = false;
        }
    }

    function closeUnitEditor() {
        unitEditVisible.value = false;
    }

    Object.assign(ctx, {
        services, servicesLoading, servicesError, serviceSearch, serviceStateFilter, filteredServices, serviceStateCounts,
        serviceLogVisible, serviceLogData, serviceLogLoading, serviceLogLines, serviceActionPending,
        loadServices, handleServiceAction, viewServiceLog, refreshServiceLog, closeServiceLog,
        serviceStateText, serviceStateClass,
        unitEditVisible, unitEditLoading, unitEditSaving, unitEditData,
        openUnitEditor, saveUnitFile, closeUnitEditor,
    });
}
