/**
 * auth.js — 登录认证域
 *
 * 登录态探测、登录/退出、首登强制改密与会话失效处理。
 */

import { login as apiLogin, logout as apiLogout, fetchMe, changePassword as apiChangePassword } from '../api/auth.js';

const { ref, reactive } = window.Vue;

export function setupAuth(ctx) {
    const authed = ref(false);
    const authChecked = ref(false);              // 是否完成登录态探测
    const currentUser = ref('');
    const mustChangePassword = ref(false);       // 首次登录（仍为默认密码）强制改密
    const loginForm = reactive({ username: 'admin', password: 'admin123' }); // 默认账密预填写
    const loginError = ref('');
    const loginLoading = ref(false);
    const pwForm = reactive({ oldPassword: '', newPassword: '', confirmPassword: '' });
    const pwError = ref('');
    const pwSaving = ref(false);
    const userMenuOpen = ref(false);       // 右上角用户菜单展开状态
    const pwModalVisible = ref(false);     // 手动打开的修改密码弹窗（非首登强制）

    /** 展开/收起右上角用户菜单。 */
    function toggleUserMenu() {
        userMenuOpen.value = !userMenuOpen.value;
    }

    /** 点击用户菜单外部时收起。 */
    function handleDocClickForUserMenu(e) {
        if (userMenuOpen.value && !e.target.closest('.user-menu')) {
            userMenuOpen.value = false;
        }
    }
    document.addEventListener('click', handleDocClickForUserMenu);

    /** 打开修改密码弹窗（重置表单）。 */
    function openPwModal() {
        pwForm.oldPassword = '';
        pwForm.newPassword = '';
        pwForm.confirmPassword = '';
        pwError.value = '';
        userMenuOpen.value = false;
        pwModalVisible.value = true;
    }

    /** 关闭修改密码弹窗。 */
    function closePwModal() {
        pwModalVisible.value = false;
    }

    /** 探测登录态：已登录直接进入应用，否则展示登录页。 */
    async function initAuth() {
        try {
            const me = await fetchMe();
            enterApp(me.username, me.must_change_password);
        } catch {
            authed.value = false;
        } finally {
            authChecked.value = true;
        }
    }

    /** 登录成功后进入应用并加载业务数据。 */
    function enterApp(username, mustChange) {
        currentUser.value = username;
        mustChangePassword.value = !!mustChange;
        authed.value = true;
        loadAppData();
    }

    /** 登录后加载全部业务数据（未登录前不发任何业务请求）。 */
    function loadAppData() {
        ctx.loadServers();
        ctx.loadSoftwareCatalog();
        ctx.loadSources();
        ctx.loadSettings();
        ctx.loadLanguages();
    }

    /** 提交登录。 */
    async function handleLogin() {
        loginError.value = '';
        if (!loginForm.username || !loginForm.password) {
            loginError.value = '请输入用户名和密码';
            return;
        }
        loginLoading.value = true;
        try {
            const data = await apiLogin(loginForm.username, loginForm.password);
            enterApp(data.username, data.must_change_password);
            if (data.must_change_password) ctx.toastNotify('首次登录，请先修改默认密码');
        } catch (err) {
            loginError.value = err.message;
        } finally {
            loginLoading.value = false;
        }
    }

    /** 退出登录，回到登录页（重置为默认账密预填写）。 */
    async function handleLogout() {
        try { await apiLogout(); } catch { /* 会话可能已失效，忽略 */ }
        leaveApp();
        loginForm.username = 'admin';
        loginForm.password = 'admin123';
    }

    /** 会话失效（401）时回到登录页。 */
    function handleUnauthorized() {
        if (!authed.value) return;
        leaveApp();
        ctx.toastNotify('登录已过期，请重新登录', true);
    }

    /** 清理本地登录态并断开 WebSocket。 */
    function leaveApp() {
        authed.value = false;
        mustChangePassword.value = false;
        currentUser.value = '';
        userMenuOpen.value = false;
        pwModalVisible.value = false;
        if (ctx.wsController) ctx.wsController.disconnect();
    }

    /** 提交修改密码（首登强制改密共用）。 */
    async function handleChangePassword() {
        pwError.value = '';
        if (!pwForm.oldPassword) { pwError.value = '请输入当前密码'; return; }
        if (pwForm.newPassword.length < 8) { pwError.value = '新密码长度至少 8 位'; return; }
        if (pwForm.newPassword !== pwForm.confirmPassword) { pwError.value = '两次输入的新密码不一致'; return; }
        pwSaving.value = true;
        try {
            await apiChangePassword(pwForm.oldPassword, pwForm.newPassword);
            mustChangePassword.value = false;
            pwModalVisible.value = false;
            pwForm.oldPassword = '';
            pwForm.newPassword = '';
            pwForm.confirmPassword = '';
            ctx.toastNotify('密码已修改，后续登录请使用新密码');
        } catch (err) {
            pwError.value = err.message;
        } finally {
            pwSaving.value = false;
        }
    }

    Object.assign(ctx, {
        authed, authChecked, currentUser, mustChangePassword,
        loginForm, loginError, loginLoading,
        pwForm, pwError, pwSaving,
        userMenuOpen, pwModalVisible,
        toggleUserMenu, openPwModal, closePwModal,
        handleLogin, handleLogout, handleChangePassword,
        initAuth, handleUnauthorized,
    });
}
