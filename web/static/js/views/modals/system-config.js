import { appCtx } from '../../store/index.js';

export const DockerSourceFormModal = {
    name: 'DockerSourceFormModal',
    setup: () => appCtx(),
    template: `
    <div class="modal-overlay" @click.self="closeDockerSourceForm">
        <div class="modal">
            <div class="modal-header"><div><span class="eyebrow">{{ dockerSourceForm.id ? 'EDIT SOURCE' : 'NEW SOURCE' }}</span><h3>{{ dockerSourceForm.id ? t('编辑镜像源') : t('添加镜像源') }}</h3></div><button class="icon-button" @click="closeDockerSourceForm" :title="t('关闭')">×</button></div>
            <div class="modal-body">
                <div class="form-grid">
                    <label class="field"><span>ID <em>*</em></span><input v-model.trim="dockerSourceForm.id" :placeholder="t('例如：custom_mirror')" :disabled="!!dockerSourceForm.id"></label>
                    <label class="field"><span>{{ t('名称') }} <em>*</em></span><input v-model.trim="dockerSourceForm.name" :placeholder="t('例如：公司内部镜像')"></label>
                    <label class="field full"><span>{{ t('URL 地址') }} <em>*</em></span><input v-model.trim="dockerSourceForm.url" placeholder="https://..."></label>
                    <label class="field"><span>{{ t('状态') }}</span><select v-model="dockerSourceForm.enabled"><option :value="true">{{ t('启用') }}</option><option :value="false">{{ t('禁用') }}</option></select></label>
                </div>
            </div>
            <div class="modal-footer">
                <button class="toolbar-button" @click="closeDockerSourceForm">{{ t('取消') }}</button>
                <button class="primary-button" @click="saveDockerSource" :disabled="dockerSourceSaving">{{ dockerSourceSaving ? t('保存中...') : t('保存') }}</button>
            </div>
        </div>
    </div>
`,
};

export const LangFormModal = {
    name: 'LangFormModal',
    setup: () => appCtx(),
    template: `
    <div class="modal-overlay" @click.self="closeLangForm">
        <div class="modal lang-form-modal">
            <div class="modal-header"><div><span class="eyebrow">{{ langFormMode === 'add' ? 'NEW LANGUAGE' : 'EDIT LANGUAGE' }}</span><h3>{{ langFormMode === 'add' ? t('添加语言环境') : t('编辑语言环境') }}</h3></div><button class="icon-button" @click="closeLangForm" :title="t('关闭')">×</button></div>
            <div class="modal-body">
                <div class="form-grid">
                    <label class="field"><span>ID <em>*</em></span><input v-model.trim="langForm.id" :placeholder="t('例如：go')" :disabled="langFormMode === 'edit'"></label>
                    <label class="field"><span>{{ t('名称') }} <em>*</em></span><input v-model.trim="langForm.name" :placeholder="t('例如：Go')"></label>
                    <label class="field"><span>{{ t('分类') }}</span><select v-model="langForm.category"><option value="language">{{ t('编程语言') }}</option><option value="runtime">{{ t('运行时') }}</option><option value="tool">{{ t('工具') }}</option></select></label>
                    <label class="field"><span>{{ t('图标') }}</span><input v-model.trim="langForm.icon" :placeholder="t('例如：🐹')"></label>
                    <label class="field full"><span>{{ t('可选版本') }}</span><input v-model.trim="langForm.versions" :placeholder="t('逗号分隔，例如：1.21, 1.22, 1.23')">
                        <small class="field-hint">{{ t('安装时供下拉选择，同时支持在安装页输入自定义版本') }}</small></label>
                    <label class="field full"><span>{{ t('描述') }}</span><input v-model.trim="langForm.description" :placeholder="t('语言环境简介')"></label>
                    <label class="field full"><span>{{ t('版本检测命令') }} <em>*</em></span><input v-model.trim="langForm.version_cmd" :placeholder="t('例如：go version 2>/dev/null')"></label>
                </div>
                <details class="form-advanced">
                    <summary>{{ t('高级选项（安装 / 卸载脚本）') }}<small>{{ t('通常无需修改，面板自动适配服务器的包管理器') }}</small></summary>
                    <div class="form-grid">
                        <label class="field full"><span>{{ t('安装脚本') }}</span><textarea v-model.trim="langForm.install_cmd" rows="4" spellcheck="false"></textarea>
                            <small class="field-hint">{{ t('安装时自动探测 apt-get / dnf / yum / apk / zypper / brew 并执行首个可用分支，支持') }} <code v-pre>{{VERSION}}</code> {{ t('版本占位符，非 root 自动免密 sudo 重试') }}</small></label>
                        <label class="field full"><span>{{ t('卸载脚本') }}</span><textarea v-model.trim="langForm.uninstall_cmd" rows="4" spellcheck="false"></textarea>
                            <small class="field-hint">{{ t('内置语言的脚本随面板更新自动同步，此处修改重启后会覆盖为最新内置版本') }}</small></label>
                    </div>
                </details>
                <div class="form-error" v-if="langFormError">{{ langFormError }}</div>
            </div>
            <div class="modal-footer">
                <button class="toolbar-button" @click="closeLangForm">{{ t('取消') }}</button>
                <button class="primary-button" @click="saveLanguage" :disabled="langSaving">{{ langSaving ? t('保存中...') : t('保存') }}</button>
            </div>
        </div>
    </div>
`,
};

export const ConfigDetailModal = {
    name: 'ConfigDetailModal',
    setup: () => appCtx(),
    template: `
    <div class="modal-overlay" @click.self="closeConfigDetail">
        <div class="modal config-detail-modal">
            <div class="modal-header">
                <div><span class="eyebrow">{{ configDetailType === 'pkg' ? 'PACKAGE DETAILS' : 'LANGUAGE DETAILS' }}</span><h3>{{ t('{p1} · 配置详情', { p1: configDetailData.name }) }}</h3></div>
                <button class="icon-button" @click="closeConfigDetail" :title="t('关闭')">×</button>
            </div>
            <div class="modal-body">
                <div class="cd-head">
                    <span class="cd-icon">{{ configDetailData.icon || '📦' }}</span>
                    <div class="cd-head-info">
                        <strong>{{ configDetailData.name }}</strong>
                        <small>{{ configDetailData.description || t('暂无描述') }}</small>
                    </div>
                    <span class="package-category">{{ configDetailData.category }}</span>
                </div>
                <div class="cd-section" v-if="configDetailVersions.length">
                    <div class="form-section-title">{{ t('可选版本') }}</div>
                    <div class="cd-versions">
                        <span v-for="v in configDetailVersions" :key="v" class="cd-version-chip">{{ v }}</span>
                    </div>
                </div>
                <div class="cd-section" v-if="configDetailData.version_cmd">
                    <div class="form-section-title">{{ t('版本检测') }}</div>
                    <div class="cd-cmd-row"><span class="detail-label">{{ t('检测命令') }}</span><code>{{ configDetailData.version_cmd }}</code></div>
                </div>
                <template v-if="configDetailType === 'pkg'">
                    <div class="cd-section" v-if="configDetailInstall.length">
                        <div class="form-section-title">{{ t('安装命令') }}</div>
                        <div v-for="[mgr, cmds] in configDetailInstall" :key="'i-' + mgr" class="cd-cmd-group">
                            <span class="cd-mgr-badge">{{ mgr }}</span>
                            <pre v-for="(c, i) in cmds" :key="i">{{ c }}</pre>
                        </div>
                    </div>
                    <div class="cd-section" v-if="configDetailUninstall.length">
                        <div class="form-section-title">{{ t('卸载命令') }}</div>
                        <div v-for="[mgr, cmds] in configDetailUninstall" :key="'u-' + mgr" class="cd-cmd-group">
                            <span class="cd-mgr-badge">{{ mgr }}</span>
                            <pre v-for="(c, i) in cmds" :key="i">{{ c }}</pre>
                        </div>
                    </div>
                    <div class="cd-section" v-if="configDetailData.status_cmd || configDetailData.start_cmd || configDetailData.stop_cmd || configDetailData.restart_cmd">
                        <div class="form-section-title">{{ t('生命周期命令') }}</div>
                        <div class="cd-cmd-row" v-if="configDetailData.status_cmd"><span class="detail-label">{{ t('状态查询') }}</span><code>{{ configDetailData.status_cmd }}</code></div>
                        <div class="cd-cmd-row" v-if="configDetailData.start_cmd"><span class="detail-label">{{ t('启动服务') }}</span><code>{{ configDetailData.start_cmd }}</code></div>
                        <div class="cd-cmd-row" v-if="configDetailData.stop_cmd"><span class="detail-label">{{ t('停止服务') }}</span><code>{{ configDetailData.stop_cmd }}</code></div>
                        <div class="cd-cmd-row" v-if="configDetailData.restart_cmd"><span class="detail-label">{{ t('重启服务') }}</span><code>{{ configDetailData.restart_cmd }}</code></div>
                    </div>
                </template>
                <template v-else>
                    <div class="cd-section">
                        <div class="form-section-title">{{ t('安装 / 卸载') }}</div>
                        <p class="cd-note">{{ t('由面板自动处理：安装时自动适配服务器的包管理器（apt-get / yum / dnf / brew），卸载时同步清理依赖与残留文件。') }}</p>
                    </div>
                </template>
                <div class="cd-empty" v-if="configDetailEmpty">{{ t('该配置暂无详细信息') }}</div>
            </div>
            <div class="modal-footer">
                <button class="primary-button" @click="closeConfigDetail">{{ t('关闭') }}</button>
            </div>
        </div>
    </div>
`,
};

export const LangInstallLogModal = {
    name: 'LangInstallLogModal',
    setup: () => appCtx(),
    template: `
    <div class="modal-overlay" @click.self="langInstallLogVisible = false">
        <div class="modal log-modal">
            <div class="modal-header">
                <div><span class="eyebrow">TASK LOG</span><h3>{{ langInstallName }} · {{ langInstallAction === 'uninstall' ? t('卸载日志') : t('安装日志') }}</h3></div>
                <button class="icon-button" @click="langInstallLogVisible = false" :title="t('关闭')">×</button>
            </div>
            <div class="modal-body">
                <div class="log-toolbar">
                    <span class="log-status" :class="langInstallStatus">{{ langStatusText || t('等待任务开始...') }}</span>
                    <span class="log-hint" v-if="langInstallStatus === 'installing'">{{ t('日志每 1.5 秒自动刷新，可直接关闭弹窗，任务在后台继续') }}</span>
                    <button class="toolbar-button small" @click="refreshInstallLog()">{{ t('↻ 立即刷新') }}</button>
                </div>
                <div class="log-viewer" ref="langLogViewer">
                    <pre>{{ langInstallLog || t('等待任务开始...') }}</pre>
                </div>
            </div>
        </div>
    </div>
`,
};

export const UnitEditModal = {
    name: 'UnitEditModal',
    setup: () => appCtx(),
    template: `
    <div class="modal-overlay" @click.self="closeUnitEditor">
        <div class="modal log-modal">
            <div class="modal-header">
                <div><span class="eyebrow">UNIT FILE</span><h3>{{ t('{p1} · 单元文件', { p1: unitEditData.name }) }}</h3></div>
                <button class="icon-button" @click="closeUnitEditor" :title="t('关闭')">×</button>
            </div>
            <div class="modal-body">
                <div class="log-toolbar">
                    <span class="log-file-path">{{ unitEditData.path || '—' }}</span>
                    <span class="log-toolbar-spacer"></span>
                    <button class="toolbar-button" @click="saveUnitFile()" :disabled="unitEditSaving || unitEditLoading || !unitEditData.path">{{ unitEditSaving ? t('保存中...') : t('保存') }}</button>
                </div>
                <textarea class="unit-textarea" v-model="unitEditData.content" :disabled="unitEditLoading" spellcheck="false"></textarea>
                <div class="unit-tip">{{ t('保存后自动备份原文件为 *.bak，并执行 systemctl daemon-reload 使配置生效。') }}</div>
            </div>
        </div>
    </div>
`,
};
