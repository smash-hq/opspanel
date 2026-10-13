/**
 * store/index.js — 响应式状态中心聚合出口
 *
 * 按业务域拆分的各 setup 模块在此按序组装：
 * 每个模块导出 setupXxx(ctx)，向共享上下文 ctx 注册自己的状态与方法；
 * 域内使用本地闭包变量，跨域引用统一通过 ctx（晚绑定，注册顺序无关）。
 *
 * 导出 setupStore() 工厂函数，供 app.js 在 Vue setup 中调用，
 * 返回值即暴露给模板的完整上下文（键名与拆分前完全一致）。
 */

import { setupTheme } from './theme.js';
import { setupAuth } from './auth.js';
import { setupUi } from './ui.js';
import { setupServers } from './servers.js';
import { setupServerForm } from './server-form.js';
import { setupSoftware } from './software.js';
import { setupPackages } from './packages.js';
import { setupLanguages } from './languages.js';
import { setupFiles } from './files.js';
import { setupDocker } from './docker.js';
import { setupMirrors } from './mirrors.js';
import { setupSources } from './sources.js';
import { setupProcesses } from './processes.js';
import { setupTerminal } from './terminal.js';
import { setupLifecycle } from './lifecycle.js';
import { number, bounded, fmtBytes, fmtUptime, usageClass, statusText, serverAddress } from '../utils.js';

const { nextTick } = window.Vue;

export function setupStore() {
    const ctx = {};

    setupTheme(ctx);
    setupAuth(ctx);
    setupUi(ctx);
    setupServers(ctx);
    setupServerForm(ctx);
    setupSoftware(ctx);
    setupPackages(ctx);
    setupLanguages(ctx);
    setupFiles(ctx);
    setupDocker(ctx);
    setupMirrors(ctx);
    setupSources(ctx);
    setupProcesses(ctx);
    setupTerminal(ctx);
    setupLifecycle(ctx);

    // 模板直接调用的工具函数
    Object.assign(ctx, {
        number, bounded, fmtBytes, fmtUptime, usageClass, statusText, serverAddress, nextTick,
    });

    return ctx;
}
