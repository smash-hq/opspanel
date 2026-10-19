/**
 * server-form.js — 服务器表单域
 *
 * 添加/编辑服务器、连接测试与删除确认。
 */
import { t } from '../i18n/index.js';


import { createServer, updateServer, testConnection, deleteServer as apiDeleteServer } from '../api/servers.js';

const { ref, reactive } = window.Vue;

/**
 * 创建空的表单模板。
 * @returns {Object} 空白表单对象
 */
function emptyForm() {
    return {
        id: '', name: '', type: 'remote', host: '', port: 22,
        username: '', auth_type: 'password', password: '', key_path: '',
    };
}

export function setupServerForm(ctx) {
    // ─── 表单状态 ───
    const formMode = ref(null);       // null | "add" | "edit"
    const formError = ref('');
    const saving = ref(false);
    const form = reactive(emptyForm());

    // ─── 连接测试状态 ───
    const testing = ref(false);
    const testResult = ref(null);     // { success, host?, error? } | null

    /** 打开「添加服务器」表单。 */
    function openAdd() {
        formMode.value = 'add';
        formError.value = '';
        Object.assign(form, emptyForm());
        form.port = ctx.settingsForm.defaultSSHPort || 22;
    }

    /** 打开「编辑服务器」表单。 */
    function openEdit(server) {
        if (!server) return;
        formMode.value = 'edit';
        formError.value = '';
        Object.assign(form, {
            id: server.id, name: server.name, type: server.type,
            host: server.host || '', port: server.port || 22,
            username: server.username || '', auth_type: server.auth_type || 'password',
            password: '', key_path: server.key_path || '',
        });
    }

    /** 关闭表单。 */
    function closeForm() {
        if (!saving.value) formMode.value = null;
    }

    /** 校验表单并提交。 */
    async function saveServer() {
        formError.value = '';
        if (!form.name.trim()) { formError.value = t('请输入服务器名称'); return; }
        if (form.type === 'remote' && (!form.host.trim() || !form.username.trim())) {
            formError.value = t('远程服务器需要填写主机地址和用户名');
            return;
        }
        if (form.type === 'remote' && formMode.value === 'add' && form.auth_type === 'password' && !form.password) {
            formError.value = t('请输入登录密码');
            return;
        }
        if (form.type === 'remote' && form.auth_type === 'key' && !form.key_path.trim()) {
            formError.value = t('请输入私钥文件路径');
            return;
        }

        saving.value = true;
        const isEdit = formMode.value === 'edit';
        const payload = { name: form.name, type: form.type };
        if (form.type === 'remote') {
            Object.assign(payload, {
                host: form.host, port: form.port || 22,
                username: form.username, auth_type: form.auth_type,
            });
            if (form.auth_type === 'password' && form.password) payload.password = form.password;
            if (form.auth_type === 'key' && form.key_path) payload.key_path = form.key_path;
        }

        try {
            const saved = isEdit
                ? await updateServer(form.id, payload)
                : await createServer(payload);
            await ctx.loadServers();
            formMode.value = null;
            ctx.toastNotify(isEdit ? t('服务器配置已更新') : t('服务器已添加'));
            if (!isEdit && saved?.id) ctx.selectServer(saved.id);
            else if (isEdit && ctx.selectedServer.value === form.id) ctx.refreshCurrent();
        } catch (err) {
            formError.value = err.message;
        } finally {
            saving.value = false;
        }
    }

    /** 删除服务器（带确认）。 */
    async function handleDeleteServer(id) {
        const server = ctx.servers.value.find(s => s.id === id);
        if (!server || !await ctx.confirmDialog(t('确定删除「{p1}」吗？删除后连接配置不可恢复。', { p1: server.name }), { title: t('删除服务器'), confirmText: t('删除') })) return;
        try {
            await apiDeleteServer(id);
            if (ctx.selectedServer.value === id) {
                ctx.selectedServer.value = null;
                if (ctx.wsController) { ctx.wsController.disconnect(); ctx.wsController = null; }
            }
            await ctx.loadServers();
            ctx.toastNotify(t('服务器已删除'));
        } catch (err) {
            ctx.toastNotify(err.message, true);
        }
    }

    /** 测试远程服务器连接（不保存）。在添加/编辑页面上调用。 */
    async function testServerConnection() {
        formError.value = '';
        testResult.value = null;

        // 基本校验
        if (!form.name.trim()) { formError.value = t('请输入服务器名称'); return; }
        if (form.type === 'remote' && (!form.host.trim() || !form.username.trim())) {
            formError.value = t('远程服务器需要填写主机地址和用户名');
            return;
        }
        if (form.type === 'remote' && form.auth_type === 'password' && !form.password) {
            formError.value = t('请输入登录密码');
            return;
        }
        if (form.type === 'remote' && form.auth_type === 'key' && !form.key_path.trim()) {
            formError.value = t('请输入私钥文件路径');
            return;
        }

        testing.value = true;
        const payload = { name: form.name, type: form.type };
        if (form.type === 'remote') {
            Object.assign(payload, {
                host: form.host, port: form.port || 22,
                username: form.username, auth_type: form.auth_type,
            });
            if (form.auth_type === 'password' && form.password) payload.password = form.password;
            if (form.auth_type === 'key' && form.key_path) payload.key_path = form.key_path;
        }

        try {
            const result = await testConnection(payload);
            testResult.value = result;
        } catch (err) {
            testResult.value = { success: false, error: err.message };
        } finally {
            testing.value = false;
        }
    }

    /** 清除测试结果（表单字段变更时调用）。 */
    function clearTestResult() {
        testResult.value = null;
    }

    Object.assign(ctx, {
        formMode, formError, saving, form,
        testing, testResult,
        openAdd, openEdit, closeForm, saveServer,
        deleteServer: handleDeleteServer, handleDeleteServer,
        testServerConnection, clearTestResult,
    });
}
