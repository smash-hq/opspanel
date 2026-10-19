import { appCtx } from '../store/index.js';

export const NotifyCenter = {
    name: 'NotifyCenter',
    setup: () => appCtx(),
    template: `
            <section class="notifycenter-page">
                <!-- 通知列表：左侧主栏 -->
                <div class="nc-card nc-list-card">
                    <div class="nc-toolbar">
                        <h3>{{ t('通知列表') }}</h3>
                        <div class="nc-filters">
                            <select class="nc-filter" v-model="ncFilterRead" :title="t('按已读筛选')">
                                <option value="all">{{ t('全部状态') }}</option>
                                <option value="unread">{{ t('未读') }}</option>
                                <option value="read">{{ t('已读') }}</option>
                            </select>
                            <select class="nc-filter" v-model="ncFilterLevel" :title="t('按类型筛选')">
                                <option value="all">{{ t('全部类型') }}</option>
                                <option value="info">{{ t('信息') }}</option>
                                <option value="warn">{{ t('警告') }}</option>
                                <option value="error">{{ t('错误') }}</option>
                            </select>
                            <select class="nc-filter" v-model="ncFilterModule" :title="t('按模块筛选')">
                                <option value="all">{{ t('全部模块') }}</option>
                                <option v-for="m in ncModuleOptions" :key="m.value" :value="m.value">{{ m.label }}</option>
                            </select>
                            <button class="toolbar-button" @click="loadNotifications()" :disabled="notifyLoading">{{ t('↻ 刷新') }}</button>
                            <button v-if="unreadCount > 0" class="toolbar-button" @click="markAllRead()">{{ t('全部已读') }}</button>
                            <button class="toolbar-button danger-text" @click="clearAllNotifications()">{{ t('清空') }}</button>
                        </div>
                    </div>
                    <div v-if="notifyLoading && !notifications.length" class="notify-empty">{{ t('加载中...') }}</div>
                    <div v-else-if="!ncFiltered.length" class="notify-empty">
                        <span class="notify-empty-icon"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"><path d="M18 8A6 6 0 0 0 6 8c0 7-3 9-3 9h18s-3-2-3-9"/><path d="M13.73 21a2 2 0 0 1-3.46 0"/></svg></span>{{ notifications.length ? t('没有符合筛选条件的通知') : t('暂无通知') }}
                    </div>
                    <div v-else class="nc-list">
                        <div v-for="n in ncFiltered" :key="n.id" class="notify-item" :class="[n.level, { read: n.read }]" @click="openNotification(n)" :title="t('点击查看详情')">
                            <span class="notify-dot"></span>
                            <div class="notify-item-body">
                                <div class="notify-item-meta">
                                    <span class="notify-type-tag" :class="n.level">{{ notifyTypeLabel(n.level) }}</span>
                                    <span class="notify-module-tag">{{ notifyModuleLabel(n.module) }}</span>
                                    <span v-if="!n.read" class="notify-unread-flag">{{ t('未读') }}</span>
                                </div>
                                <div class="notify-item-title">{{ n.title }}</div>
                                <div v-if="n.message" class="notify-item-msg" :title="n.message">{{ n.message }}</div>
                            </div>
                            <span class="notify-item-time">{{ formatNotifyTime(n.created_at) }}</span>
                        </div>
                    </div>
                </div>

                <!-- 邮件通知（SMTP）设置：固定在右侧栏 -->
                <div class="nc-card nc-mail-card">
                    <h3>{{ t('邮件通知（SMTP）') }}</h3>
                    <p class="settings-desc">{{ t('配置 SMTP 服务器后，新产生的站内通知会同步通过邮件发送；邮件异步发送，失败不影响站内通知。') }}</p>
                    <label class="nc-switch-row">
                        <input type="checkbox" v-model="emailForm.enabled">
                        <span>{{ t('启用邮件通知') }}</span>
                        <span v-if="emailHasPassword" class="nc-mail-saved">{{ t('✓ 已保存密码') }}</span>
                    </label>
                    <div class="nc-form-grid">
                        <div class="field">
                            <span>{{ t('SMTP 服务器') }}</span>
                            <input type="text" v-model.trim="emailForm.host" placeholder="smtp.example.com">
                        </div>
                        <div class="field">
                            <span>{{ t('端口') }}</span>
                            <input type="number" v-model.number="emailForm.port" min="1" max="65535" placeholder="465">
                        </div>
                        <div class="field">
                            <span>{{ t('加密方式') }}</span>
                            <select v-model="emailForm.encryption">
                                <option value="ssl">{{ t('SSL/TLS（常用端口 465）') }}</option>
                                <option value="starttls">{{ t('STARTTLS（常用端口 587）') }}</option>
                                <option value="none">{{ t('不加密（常用端口 25）') }}</option>
                            </select>
                        </div>
                        <div class="field">
                            <span>{{ t('账号') }}</span>
                            <input type="text" v-model.trim="emailForm.user" placeholder="user@example.com">
                        </div>
                        <div class="field">
                            <span>{{ t('密码 / 授权码') }} <small v-if="emailHasPassword" class="field-hint-inline">{{ t('留空保持不变') }}</small></span>
                            <input type="password" v-model="emailForm.password" :placeholder="emailHasPassword ? '••••••••' : t('SMTP 授权码')" autocomplete="new-password">
                        </div>
                        <div class="field">
                            <span>{{ t('发件人 From') }}</span>
                            <input type="text" v-model.trim="emailForm.from" :placeholder="t('panel@example.com（留空则使用账号）')">
                        </div>
                        <div class="field full">
                            <span>{{ t('收件人 To（多个用逗号分隔）') }}</span>
                            <input type="text" v-model.trim="emailForm.to" placeholder="a@example.com, b@example.com">
                        </div>
                    </div>
                    <div class="nc-mail-actions">
                        <button class="primary-button small" @click="saveEmailConfig()" :disabled="emailSaving || emailTesting">{{ emailSaving ? t('保存中...') : t('保存配置') }}</button>
                        <button class="toolbar-button" @click="testEmail()" :disabled="emailSaving || emailTesting">{{ emailTesting ? t('发送中...') : t('✉ 发送测试邮件') }}</button>
                    </div>
                </div>
            </section>
`,
};
