/**
 * packages.js — 软件包管理域
 *
 * 自定义软件包的增删改（配置详情弹窗在 ui 域）。
 */

import { createPackage, updatePackage, deletePackage as apiDeletePackage } from '../api/packages.js';
import { parseVersions } from '../utils.js';

const { ref, reactive } = window.Vue;

export function setupPackages(ctx) {
    // ─── 软件包管理 ───
    const packageFormMode = ref(null);
    const packageForm = reactive({ id: '', name: '', category: 'tool', description: '', version: '', versions: '', icon: '', install_cmd: '{}', uninstall_cmd: '{}', status_cmd: '', version_cmd: '', start_cmd: '', stop_cmd: '', restart_cmd: '' });
    const packageSaving = ref(false);
    const packageFormError = ref('');

    function openPackageForm(mode, pkg) {
        packageFormMode.value = mode;
        packageFormError.value = '';
        if (mode === 'edit' && pkg) {
            Object.assign(packageForm, {
                id: pkg.id, name: pkg.name, category: pkg.category,
                description: pkg.description || '', version: pkg.version || '',
                versions: (pkg.versions || []).join(', '), icon: pkg.icon || '',
                install_cmd: JSON.stringify(pkg.InstallCmd || {}, null, 2),
                uninstall_cmd: JSON.stringify(pkg.UninstallCmd || {}, null, 2),
                status_cmd: pkg.StatusCmd || '', version_cmd: pkg.VersionCmd || '',
                start_cmd: pkg.StartCmd || '', stop_cmd: pkg.StopCmd || '', restart_cmd: pkg.RestartCmd || '',
            });
        } else {
            Object.assign(packageForm, { id: '', name: '', category: 'tool', description: '', version: '', versions: '', icon: '', install_cmd: '{}', uninstall_cmd: '{}', status_cmd: '', version_cmd: '', start_cmd: '', stop_cmd: '', restart_cmd: '' });
        }
    }

    function closePackageForm() {
        if (!packageSaving.value) packageFormMode.value = null;
    }

    async function savePackage() {
        packageFormError.value = '';
        if (!packageForm.name.trim()) { packageFormError.value = '请输入名称'; return; }
        if (!packageForm.id.trim()) { packageFormError.value = '请输入 ID'; return; }
        let installCmd, uninstallCmd;
        try {
            installCmd = JSON.parse(packageForm.install_cmd || '{}');
            uninstallCmd = JSON.parse(packageForm.uninstall_cmd || '{}');
        } catch (e) {
            packageFormError.value = '安装/卸载命令必须是有效的 JSON';
            return;
        }
        packageSaving.value = true;
        const payload = {
            id: packageForm.id, name: packageForm.name, category: packageForm.category,
            description: packageForm.description, version: packageForm.version,
            versions: parseVersions(packageForm.versions), icon: packageForm.icon,
            install_cmd: installCmd, uninstall_cmd: uninstallCmd,
            status_cmd: packageForm.status_cmd, version_cmd: packageForm.version_cmd,
            start_cmd: packageForm.start_cmd, stop_cmd: packageForm.stop_cmd, restart_cmd: packageForm.restart_cmd,
        };
        try {
            const isEdit = packageFormMode.value === 'edit';
            if (isEdit) {
                await updatePackage(packageForm.id, payload);
            } else {
                await createPackage(payload);
            }
            await ctx.loadSoftwareCatalog();
            packageFormMode.value = null;
            ctx.toastNotify(isEdit ? '软件包已更新' : '软件包已添加');
        } catch (err) {
            packageFormError.value = err.message;
        } finally {
            packageSaving.value = false;
        }
    }

    async function handleDeletePackage(id) {
        if (!await ctx.confirmDialog(`确定删除软件包「${id}」吗？`, { title: '删除软件包' })) return;
        try {
            await apiDeletePackage(id);
            await ctx.loadSoftwareCatalog();
            ctx.toastNotify('软件包已删除');
        } catch (err) {
            ctx.toastNotify(err.message, true);
        }
    }

    Object.assign(ctx, {
        packageFormMode, packageForm, packageSaving, packageFormError,
        openPackageForm, closePackageForm, savePackage,
        deletePackage: handleDeletePackage, handleDeletePackage,
    });
}
