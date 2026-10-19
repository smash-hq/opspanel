/**
 * app.js — Vue 应用入口
 *
 * 职责：导入共享上下文与全部视图组件，创建 Vue 实例并挂载到 #app。
 * 页面模板按区块拆分在 js/views/（页面区块）、js/views/tabs/（节点详情 Tab）、
 * js/views/modals/（弹窗）中；所有组件共享 store/index.js 的单例上下文 appCtx()。
 *
 * Vue 3 以 vendor 方式本地内嵌（vendor/vue/vue.global.prod.js，经 index.html 全局加载），
 * 此处直接使用全局 Vue 对象（模块脚本 deferred 执行，此时全局已就绪）。
 */

import { appCtx } from './store/index.js';
import { LoginView } from './views/login-view.js';
import { SidebarNav } from './views/sidebar-nav.js';
import { TopBar } from './views/top-bar.js';
import { MainView } from './views/main-view.js';
import { ConfigPage } from './views/config-page.js';
import { NotifyCenter } from './views/notify-center.js';
import { EmptyStateView } from './views/empty-state-view.js';
import { NodeDetail } from './views/node-detail.js';
import { ToastHint } from './views/toast-hint.js';
import { TabSystem } from './views/tabs/tab-system.js';
import { TabProcess } from './views/tabs/tab-process.js';
import { TabServices } from './views/tabs/tab-services.js';
import { TabFiles } from './views/tabs/tab-files.js';
import { TabLanguages } from './views/tabs/tab-languages.js';
import { TabSoftware } from './views/tabs/tab-software.js';
import { TabApps } from './views/tabs/tab-apps.js';
import { TabDocker } from './views/tabs/tab-docker.js';
import { TabTerminal } from './views/tabs/tab-terminal.js';
import { TabCron } from './views/tabs/tab-cron.js';
import { TabGpu } from './views/tabs/tab-gpu.js';
import * as serverFormModals from './views/modals/server-form.js';
import * as softwareModals from './views/modals/software.js';
import * as systemConfigModals from './views/modals/system-config.js';
import * as cronModals from './views/modals/cron.js';
import * as notifyModals from './views/modals/notify.js';
import * as filesModals from './views/modals/files.js';
import * as dockerModals from './views/modals/docker.js';
import * as appsModals from './views/modals/apps.js';
import * as miscModals from './views/modals/misc.js';

// 根模板：与拆分前 index.html 的 DOM 结构一致（组件标签 + 根级条件渲染）
const rootTemplate = `
    <login-view v-if="authChecked && !authed"></login-view>
    <sidebar-nav v-if="authed"></sidebar-nav>
    <main-view v-if="authed"></main-view>
    <server-form-modal v-if="formMode"></server-form-modal>
    <software-source-modal v-if="sourceFormMode"></software-source-modal>
    <software-package-modal v-if="packageFormMode"></software-package-modal>
    <package-log-modal v-if="logModalVisible"></package-log-modal>
    <docker-source-form-modal v-if="dockerSourceFormVisible"></docker-source-form-modal>
    <lang-form-modal v-if="langFormMode"></lang-form-modal>
    <config-detail-modal v-if="configDetailVisible"></config-detail-modal>
    <cron-form-modal v-if="cronFormVisible"></cron-form-modal>
    <confirm-modal v-if="confirmState.visible"></confirm-modal>
    <notif-detail-modal v-if="notifDetailVisible"></notif-detail-modal>
    <lang-install-log-modal v-if="langInstallLogVisible"></lang-install-log-modal>
    <file-preview-modal v-if="filePreviewVisible"></file-preview-modal>
    <file-detail-modal v-if="fileDetailVisible"></file-detail-modal>
    <docker-exec-modal v-if="dockerExecVisible"></docker-exec-modal>
    <docker-term-modal v-if="dockerTermVisible"></docker-term-modal>
    <docker-logs-modal v-if="dockerLogsVisible"></docker-logs-modal>
    <docker-files-modal v-if="dockerFilesVisible"></docker-files-modal>
    <docker-preview-modal v-if="dockerPreviewVisible"></docker-preview-modal>
    <docker-run-modal v-if="dockerRunVisible"></docker-run-modal>
    <app-start-modal v-if="appStartVisible"></app-start-modal>
    <app-form-modal v-if="appFormVisible"></app-form-modal>
    <app-logs-modal v-if="appLogsVisible"></app-logs-modal>
    <process-log-modal v-if="processLogVisible"></process-log-modal>
    <service-log-modal v-if="serviceLogVisible"></service-log-modal>
    <unit-edit-modal v-if="unitEditVisible"></unit-edit-modal>
    <doc-modal v-if="showDoc"></doc-modal>
    <password-modal v-if="authed && (mustChangePassword || pwModalVisible)"></password-modal>
    <perm-guide-modal v-if="permGuideVisible"></perm-guide-modal>
    <toast-hint></toast-hint>
`;

const app = window.Vue.createApp({
    setup() {
        return appCtx();
    },
    template: rootTemplate,
});

// 注册全部组件（弹窗分组文件按命名导出展开）
const components = {
    LoginView, SidebarNav, TopBar, MainView, ConfigPage, NotifyCenter, EmptyStateView, NodeDetail, ToastHint,
    TabSystem, TabProcess, TabServices, TabFiles, TabLanguages, TabSoftware, TabApps, TabDocker, TabTerminal, TabCron, TabGpu,
    ...serverFormModals, ...softwareModals, ...systemConfigModals, ...cronModals, ...notifyModals,
    ...filesModals, ...dockerModals, ...appsModals, ...miscModals,
};
for (const [name, comp] of Object.entries(components)) {
    app.component(name, comp);
}

app.mount('#app');
