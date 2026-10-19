import { appCtx } from '../../store/index.js';

export const DocModal = {
    name: 'DocModal',
    setup: () => appCtx(),
    template: `
    <div class="modal-overlay" @click.self="showDoc = false">
        <div class="modal doc-modal">
            <div class="modal-header">
                <div><span class="eyebrow">DOCUMENTATION</span><h3>{{ t('OpsPanel 使用文档') }}</h3></div>
                <button class="icon-button" @click="showDoc = false" :title="t('关闭')">×</button>
            </div>
            <div class="doc-content">
                <h3>{{ t('🚀 快速开始') }}</h3>
                <p>{{ t('打开面板后，从左侧节点列表选择一台服务器开始使用。') }}</p>
                <ul>
                    <li><strong>{{ t('本机节点') }}</strong>{{ t('：自动检测，无需配置') }}</li>
                    <li><strong>{{ t('远程节点') }}</strong>{{ t('：点击「＋ 添加节点」配置 SSH 连接') }}</li>
                </ul>

                <h3>{{ t('📊 系统概览') }}</h3>
                <p>{{ t('查看服务器的 CPU、内存、磁盘、运行时间等系统指标。') }}</p>
                <ul>
                    <li>{{ t('点击') }} <code>{{ t('↻ 刷新') }}</code> {{ t('获取最新数据') }}</li>
                    <li>{{ t('数据通过 WebSocket 实时推送，每 3 秒自动更新') }}</li>
                </ul>

                <h3>{{ t('📦 软件管理') }}</h3>
                <p>{{ t('在「软件管理」Tab 中管理服务器上的软件。') }}</p>
                <ul>
                    <li><strong>{{ t('安装软件') }}</strong>{{ t('：点击「安装」按钮一键安装') }}</li>
                    <li><strong>{{ t('服务控制') }}</strong>{{ t('：启动 ⏸ 停止 ↻ 重启 ✗ 卸载') }}</li>
                    <li><strong>{{ t('查看日志') }}</strong>{{ t('：点击 📋 查看服务运行日志') }}</li>
                </ul>

                <h3>{{ t('📋 日志查看') }}</h3>
                <p>{{ t('支持查看 systemd 服务日志和 Docker 容器日志。') }}</p>
                <ul>
                    <li>{{ t('可选择显示 50/100/200/500 行') }}</li>
                    <li>{{ t('点击「↻ 刷新」获取最新日志') }}</li>
                </ul>

                <h3>{{ t('⚙️ 系统设置') }}</h3>
                <p>{{ t('点击左下角齿轮图标进入设置。') }}</p>
                <ul>
                    <li><strong>{{ t('外观主题') }}</strong>{{ t('：白天 / 夜间 / 跟随系统') }}</li>
                    <li><strong>{{ t('网站图标') }}</strong>{{ t('：自定义浏览器标签页图标') }}</li>
                    <li><strong>{{ t('软件源') }}</strong>{{ t('：管理镜像源和仓库地址') }}</li>
                </ul>

                <h3>{{ t('🔗 连接测试') }}</h3>
                <p>{{ t('添加远程节点时，可先点击「⚡ 测试连接」验证 SSH 凭据。') }}</p>
                <ul>
                    <li>{{ t('支持密码认证和私钥认证') }}</li>
                    <li>{{ t('连接成功后会显示远程主机基本信息') }}</li>
                </ul>

                <h3>{{ t('⌨️ 快捷键') }}</h3>
                <ul>
                    <li><code>/</code> {{ t('— 聚焦搜索框') }}</li>
                </ul>
            </div>
        </div>
    </div>
`,
};

export const PasswordModal = {
    name: 'PasswordModal',
    setup: () => appCtx(),
    template: `
    <div class="modal-overlay" @click.self="!mustChangePassword && closePwModal()">
        <div class="modal auth-pw-modal">
            <div class="modal-header">
                <div><span class="eyebrow">SECURITY</span><h3>{{ mustChangePassword ? t('修改默认密码') : t('修改密码') }}</h3></div>
                <button v-if="!mustChangePassword" class="icon-button" @click="closePwModal()" :title="t('关闭')">×</button>
            </div>
            <div class="modal-body">
                <p class="settings-desc" v-if="mustChangePassword">{{ t('检测到仍在使用默认密码，为保障安全请先设置新密码。修改后持续生效，重新部署后恢复默认。') }}</p>
                <form class="form-grid" @submit.prevent="handleChangePassword">
                    <label class="field full"><span>{{ t('当前密码') }}</span><input type="password" v-model="pwForm.oldPassword" autocomplete="current-password"></label>
                    <label class="field full"><span>{{ t('新密码（至少 8 位）') }}</span><input type="password" v-model="pwForm.newPassword" autocomplete="new-password"></label>
                    <label class="field full"><span>{{ t('确认新密码') }}</span><input type="password" v-model="pwForm.confirmPassword" autocomplete="new-password"></label>
                </form>
                <div class="auth-error" v-if="pwError">{{ pwError }}</div>
            </div>
            <div class="modal-footer">
                <button class="primary-button" @click="handleChangePassword" :disabled="pwSaving">{{ pwSaving ? t('提交中...') : t('确认修改') }}</button>
            </div>
        </div>
    </div>
`,
};

export const PermGuideModal = {
    name: 'PermGuideModal',
    setup: () => appCtx(),
    template: `
    <div class="modal-overlay" @click.self="closePermGuide">
        <div class="modal perm-guide-modal">
            <div class="modal-header">
                <div><span class="eyebrow">PERMISSION</span><h3>{{ t('权限说明') }}</h3></div>
                <button class="icon-button" @click="closePermGuide" :title="t('关闭')">×</button>
            </div>
            <div class="modal-body perm-guide-body">
                <p class="settings-desc">{{ t('服务以部署时指定的运行用户身份执行操作，无需 root 也能查看大部分信息。如遇权限不足，可参考以下方式提升权限。') }}</p>

                <div class="perm-section">
                    <h4>{{ t('本机节点') }}</h4>
                    <p>{{ t('本机节点由服务进程直接读取') }} <code>/proc</code>、<code>/sys</code> {{ t('采集系统信息，通常') }}<strong>{{ t('无需任何额外权限') }}</strong>{{ t('。以下操作可能受限：') }}</p>
                    <ul>
                        <li>{{ t('结束其他用户的进程') }}</li>
                        <li>{{ t('浏览受保护的文件/目录（如') }} <code>/etc/shadow</code>{{ t('、其他用户主目录）') }}</li>
                        <li>{{ t('管理 Docker 容器（运行用户不在 docker 组时）') }}</li>
                    </ul>
                    <p><strong>{{ t('方式一：配置免密 sudo（推荐）') }}</strong>{{ t('——服务执行失败时会自动尝试') }} <code>sudo -n</code> {{ t('免密提权：') }}</p>
                    <pre><code>sudo visudo -f /etc/sudoers.d/opspanel
{{ t('# 写入以下内容（opspaneluser 替换为服务运行用户）：') }}
opspaneluser ALL=(ALL) NOPASSWD: ALL
{{ t('# 保存后设置安全权限：') }}
sudo chmod 0440 /etc/sudoers.d/opspanel</code></pre>
                    <p><strong>{{ t('方式二：以 root 运行服务') }}</strong>{{ t('——修改 systemd unit 中') }} <code>User=root</code> {{ t('后') }} <code>systemctl daemon-reload &amp;&amp; systemctl restart</code>{{ t('。简单直接，但权限过大，不建议在生产环境使用。') }}</p>
                    <p><strong>{{ t('Docker 管理：') }}</strong>{{ t('将运行用户加入 docker 组（重新登录后生效）：') }}</p>
                    <pre><code>sudo usermod -aG docker opspaneluser</code></pre>
                </div>

                <div class="perm-section">
                    <h4>{{ t('远程节点') }}</h4>
                    <p>{{ t('远程节点通过 SSH 用户执行命令，非 root 用户同样存在上述限制。建议为 SSH 用户配置') }} <code>NOPASSWD</code> {{ t('免密 sudo，详细步骤参见项目文档') }} <code>docs/sudo-setup.md</code>。</p>
                </div>
            </div>
        </div>
    </div>
`,
};
