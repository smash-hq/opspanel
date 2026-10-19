import { appCtx } from '../../store/index.js';

export const ServerFormModal = {
    name: 'ServerFormModal',
    setup: () => appCtx(),
    template: `
    <div class="modal-overlay" @click.self="closeForm">
        <div class="modal">
            <div class="modal-header"><div><span class="eyebrow">{{ formMode === 'add' ? 'NEW ASSET' : 'EDIT ASSET' }}</span><h3>{{ formMode === 'add' ? t('添加服务器') : t('编辑服务器配置') }}</h3></div><button class="icon-button" @click="closeForm" :title="t('关闭')">×</button></div>
            <div class="modal-body">
                <div class="form-intro"><span class="form-avatar">{{ form.type === 'remote' ? '↗' : '⌂' }}</span><div><strong>{{ formMode === 'add' ? t('配置新的管理节点') : t('更新连接信息') }}</strong><small>{{ t('保存后即可从左侧资产列表连接服务器') }}</small></div></div>
                <div class="form-section-title">{{ t('基本信息') }}</div>
                <div class="form-grid"><label class="field full"><span>{{ t('显示名称') }} <em>*</em></span><input v-model.trim="form.name" @input="clearTestResult" :placeholder="t('例如：生产环境 Web-01')"></label><label class="field"><span>{{ t('服务器类型') }}</span><select v-model="form.type" @change="clearTestResult"><option value="remote">{{ t('远程 SSH') }}</option></select></label><label class="field" v-if="form.type === 'remote'"><span>{{ t('主机地址') }} <em>*</em></span><input v-model.trim="form.host" @input="clearTestResult" :placeholder="t('IP 地址或域名')"></label></div>
                <template v-if="form.type === 'remote'">
                    <div class="form-section-title">{{ t('SSH 连接') }}</div>
                    <div class="form-grid"><label class="field"><span>{{ t('SSH 端口') }}</span><input v-model.number="form.port" @input="clearTestResult" type="number" min="1" max="65535" placeholder="22"></label><label class="field"><span>{{ t('用户名') }} <em>*</em></span><input v-model.trim="form.username" @input="clearTestResult" placeholder="root"></label><label class="field"><span>{{ t('认证方式') }}</span><select v-model="form.auth_type" @change="clearTestResult"><option value="password">{{ t('密码认证') }}</option><option value="key">{{ t('私钥认证') }}</option></select></label><label class="field" v-if="form.auth_type === 'password'"><span>{{ t('登录密码') }} <em v-if="formMode === 'add'">*</em></span><input v-model="form.password" @input="clearTestResult" type="password" :placeholder="formMode === 'edit' ? t('留空则保留原密码') : t('输入 SSH 密码')"></label><label class="field full" v-if="form.auth_type === 'key'"><span>{{ t('私钥文件路径') }} <em>*</em></span><input v-model.trim="form.key_path" @input="clearTestResult" :placeholder="t('例如：/home/admin/.ssh/id_ed25519')"></label></div>
                </template>
                <div class="form-notice">{{ t('凭据仅用于服务器连接，不会在服务器列表中展示。编辑时留空密码表示保留已保存的凭据。') }}</div>

                <!-- 连接测试结果 -->
                <div v-if="testResult" class="test-result" :class="{ success: testResult.success, error: !testResult.success }">
                    <div class="test-result-icon">{{ testResult.success ? '✓' : '✗' }}</div>
                    <div class="test-result-content">
                        <strong v-if="testResult.success">{{ t('连接成功') }}</strong>
                        <strong v-else>{{ t('连接失败') }}</strong>
                        <div v-if="testResult.success && testResult.host" class="test-host-info">
                            <span><b>{{ t('主机名') }}</b>{{ testResult.host.hostname || '—' }}</span>
                            <span><b>{{ t('操作系统') }}</b>{{ testResult.host.os || '—' }}</span>
                            <span><b>{{ t('内核') }}</b>{{ testResult.host.kernel || '—' }}</span>
                            <span><b>{{ t('架构') }}</b>{{ testResult.host.arch || '—' }}</span>
                        </div>
                        <span v-if="!testResult.success && testResult.error" class="test-error-msg">{{ testResult.error }}</span>
                    </div>
                </div>

                <div class="form-error" v-if="formError">{{ formError }}</div>
            </div>
            <div class="modal-footer">
                <button class="toolbar-button" @click="closeForm">{{ t('取消') }}</button>
                <button class="toolbar-button test-button" @click="testServerConnection" :disabled="testing || saving" v-if="form.type === 'remote'">
                    {{ testing ? t('测试中…') : t('⚡ 测试连接') }}
                </button>
                <button class="primary-button" @click="saveServer" :disabled="saving || testing">{{ saving ? t('保存中...') : t('保存配置') }}</button>
            </div>
        </div>
    </div>
`,
};
