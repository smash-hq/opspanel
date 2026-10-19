import { appCtx } from '../../store/index.js';

export const SoftwareSourceModal = {
    name: 'SoftwareSourceModal',
    setup: () => appCtx(),
    template: `
    <div class="modal-overlay" @click.self="closeSourceForm">
        <div class="modal">
            <div class="modal-header"><div><span class="eyebrow">{{ sourceFormMode === 'add' ? 'NEW SOURCE' : 'EDIT SOURCE' }}</span><h3>{{ sourceFormMode === 'add' ? t('添加软件源') : t('编辑软件源') }}</h3></div><button class="icon-button" @click="closeSourceForm" :title="t('关闭')">×</button></div>
            <div class="modal-body">
                <div class="form-grid">
                    <label class="field"><span>ID <em>*</em></span><input v-model.trim="sourceForm.id" :placeholder="t('例如：src_aliyun_mirror')" :disabled="sourceFormMode === 'edit'"></label>
                    <label class="field"><span>{{ t('名称') }} <em>*</em></span><input v-model.trim="sourceForm.name" :placeholder="t('例如：阿里云镜像源')"></label>
                    <label class="field"><span>{{ t('类型') }}</span><select v-model="sourceForm.type"><option value="mirror">{{ t('镜像源') }}</option><option value="repo">{{ t('软件仓库') }}</option></select></label>
                    <label class="field"><span>{{ t('优先级') }}</span><input v-model.number="sourceForm.priority" type="number" min="0" max="999"></label>
                    <label class="field full"><span>{{ t('URL 地址') }} <em>*</em></span><input v-model.trim="sourceForm.url" placeholder="https://mirrors.example.com/..."></label>
                    <label class="field"><span>{{ t('状态') }}</span><select v-model="sourceForm.enabled"><option :value="true">{{ t('启用') }}</option><option :value="false">{{ t('禁用') }}</option></select></label>
                </div>
                <div class="form-error" v-if="sourceFormError">{{ sourceFormError }}</div>
            </div>
            <div class="modal-footer">
                <button class="toolbar-button" @click="closeSourceForm">{{ t('取消') }}</button>
                <button class="primary-button" @click="saveSource" :disabled="sourceSaving">{{ sourceSaving ? t('保存中...') : t('保存') }}</button>
            </div>
        </div>
    </div>
`,
};

export const SoftwarePackageModal = {
    name: 'SoftwarePackageModal',
    setup: () => appCtx(),
    template: `
    <div class="modal-overlay" @click.self="closePackageForm">
        <div class="modal package-modal">
            <div class="modal-header"><div><span class="eyebrow">{{ packageFormMode === 'add' ? 'NEW PACKAGE' : 'EDIT PACKAGE' }}</span><h3>{{ packageFormMode === 'add' ? t('添加软件包') : t('编辑软件包') }}</h3></div><button class="icon-button" @click="closePackageForm" :title="t('关闭')">×</button></div>
            <div class="modal-body">
                <div class="form-grid">
                    <label class="field"><span>ID <em>*</em></span><input v-model.trim="packageForm.id" :placeholder="t('例如：mysql')" :disabled="packageFormMode === 'edit'"></label>
                    <label class="field"><span>{{ t('名称') }} <em>*</em></span><input v-model.trim="packageForm.name" :placeholder="t('例如：MySQL')"></label>
                    <label class="field"><span>{{ t('分类') }}</span><select v-model="packageForm.category"><option value="database">{{ t('数据库') }}</option><option value="web">Web</option><option value="runtime">{{ t('运行时') }}</option><option value="tool">{{ t('工具') }}</option></select></label>
                    <label class="field"><span>{{ t('版本') }}</span><input v-model.trim="packageForm.version" :placeholder="t('例如：8.0')"></label>
                    <label class="field"><span>{{ t('图标') }}</span><input v-model.trim="packageForm.icon" :placeholder="t('例如：🐬')"></label>
                    <label class="field full"><span>{{ t('可选版本') }}</span><input v-model.trim="packageForm.versions" :placeholder="t('逗号分隔，例如：8.0, 8.4, 9.0')">
                        <small class="field-hint">{{ t('安装时供下拉选择，同时支持在安装页输入自定义版本') }}</small></label>
                    <label class="field full"><span>{{ t('描述') }}</span><input v-model.trim="packageForm.description" :placeholder="t('软件包简介')"></label>
                </div>
                <div class="form-section-title">{{ t('命令配置') }}</div>
                <div class="form-grid">
                    <label class="field full"><span>{{ t('安装命令 (JSON: {"apt": [...], "yum": [...]})') }}</span><textarea v-model.trim="packageForm.install_cmd" rows="2" placeholder='{"apt": ["apt-get install -y mysql-server"]}'></textarea></label>
                    <label class="field full"><span>{{ t('卸载命令 (JSON)') }}</span><textarea v-model.trim="packageForm.uninstall_cmd" rows="2" placeholder='{"apt": ["apt-get remove -y mysql-server"]}'></textarea></label>
                    <label class="field full"><span>{{ t('检测命令') }}</span><input v-model.trim="packageForm.status_cmd" :placeholder="t('例如：which mysql')"></label>
                    <label class="field full"><span>{{ t('版本命令') }}</span><input v-model.trim="packageForm.version_cmd" :placeholder="t('例如：mysql --version 2>/dev/null')"></label>
                    <label class="field full"><span>{{ t('启动命令') }}</span><input v-model.trim="packageForm.start_cmd" :placeholder="t('例如：systemctl start mysql')"></label>
                    <label class="field full"><span>{{ t('停止命令') }}</span><input v-model.trim="packageForm.stop_cmd" :placeholder="t('例如：systemctl stop mysql')"></label>
                    <label class="field full"><span>{{ t('重启命令') }}</span><input v-model.trim="packageForm.restart_cmd" :placeholder="t('例如：systemctl restart mysql')"></label>
                </div>
                <div class="form-error" v-if="packageFormError">{{ packageFormError }}</div>
            </div>
            <div class="modal-footer">
                <button class="toolbar-button" @click="closePackageForm">{{ t('取消') }}</button>
                <button class="primary-button" @click="savePackage" :disabled="packageSaving">{{ packageSaving ? t('保存中...') : t('保存') }}</button>
            </div>
        </div>
    </div>
`,
};

export const PackageLogModal = {
    name: 'PackageLogModal',
    setup: () => appCtx(),
    template: `
    <div class="modal-overlay" @click.self="closeLogModal">
        <div class="modal log-modal">
            <div class="modal-header">
                <div><span class="eyebrow">LOG VIEWER</span><h3>{{ t('{p1} · 运行日志', { p1: logData.package }) }}</h3></div>
                <button class="icon-button" @click="closeLogModal" :title="t('关闭')">×</button>
            </div>
            <div class="modal-body">
                <div class="log-toolbar">
                    <label>{{ t('行数：') }}
                        <select v-model.number="logLines" @change="viewLog(logData.package)">
                            <option :value="50">50</option>
                            <option :value="100">100</option>
                            <option :value="200">200</option>
                            <option :value="500">500</option>
                        </select>
                    </label>
                    <button class="toolbar-button small" @click="viewLog(logData.package)" :disabled="logLoading">{{ t('↻ 刷新') }}</button>
                </div>
                <div class="log-viewer" v-if="!logLoading">
                    <pre>{{ logData.content || t('暂无日志') }}</pre>
                </div>
                <div class="log-loading" v-else>{{ t('加载中...') }}</div>
            </div>
        </div>
    </div>
`,
};
