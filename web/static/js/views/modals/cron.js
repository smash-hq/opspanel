import { appCtx } from '../../store/index.js';

export const CronFormModal = {
    name: 'CronFormModal',
    setup: () => appCtx(),
    template: `
    <div class="modal-overlay" @click.self="closeCronForm">
        <div class="modal cron-form-modal">
            <div class="modal-header"><div><span class="eyebrow">{{ cronForm.id ? 'EDIT JOB' : 'NEW JOB' }}</span><h3>{{ cronForm.id ? t('编辑定时任务') : t('新建定时任务') }}</h3></div><button class="icon-button" @click="closeCronForm" :title="t('关闭')">×</button></div>
            <div class="modal-body cron-modal-body">
                <div class="field">
                    <span>{{ t('任务名称') }}</span>
                    <input type="text" v-model="cronForm.name" :placeholder="t('可选，如：每日日志清理（以注释行保存在 crontab）')" />
                </div>
                <div class="field">
                    <span>{{ t('执行周期') }} <em>*</em></span>
                    <div class="cron-kind-chips">
                        <button v-for="k in cronKinds" :key="k.key" type="button" class="cron-kind-chip"
                                :class="{ active: cronForm.kind === k.key }" @click="applyCronKind(k.key)">{{ k.label }}</button>
                    </div>
                    <div class="cron-schedule-row" v-if="cronForm.kind === 'minutes'">
                        {{ t('每隔') }} <input type="number" min="1" max="59" v-model.number="cronForm.interval" @input="updateCronSchedule()" /> {{ t('分钟执行一次') }}
                    </div>
                    <div class="cron-schedule-row" v-else-if="cronForm.kind === 'hourly'">
                        {{ t('每小时的第') }} <input type="number" min="0" max="59" v-model.number="cronForm.minute" @input="updateCronSchedule()" /> {{ t('分执行') }}
                    </div>
                    <div class="cron-schedule-row" v-else-if="cronForm.kind === 'daily'">
                        {{ t('每天') }} <input type="time" v-model="cronForm.time" @input="updateCronSchedule()" /> {{ t('执行') }}
                    </div>
                    <div class="cron-schedule-row" v-else-if="cronForm.kind === 'weekly'">
                        {{ t('每周') }}
                        <select v-model="cronForm.week" @change="updateCronSchedule()">
                            <option v-for="d in cronWeekdays" :key="d.value" :value="d.value">{{ d.label }}</option>
                        </select>
                        <input type="time" v-model="cronForm.time" @input="updateCronSchedule()" /> {{ t('执行') }}
                    </div>
                    <div class="cron-schedule-row" v-else-if="cronForm.kind === 'monthly'">
                        {{ t('每月') }} <input type="number" min="1" max="31" v-model.number="cronForm.monthDay" @input="updateCronSchedule()" /> {{ t('日') }}
                        <input type="time" v-model="cronForm.time" @input="updateCronSchedule()" /> {{ t('执行') }}
                    </div>
                    <div class="cron-schedule-row cron-schedule-hint" v-else-if="cronForm.kind === 'reboot'">
                        {{ t('服务器每次开机时执行一次（@reboot）') }}
                    </div>
                    <input v-else type="text" v-model="cronForm.expr" @input="refreshCronPreview()"
                           :placeholder="t('分 时 日 月 周，支持 @daily、@hourly 等宏')" spellcheck="false" />
                    <div class="cron-expr-bar" v-if="cronForm.kind !== 'custom'">
                        <span class="cron-expr-label">CRON</span><code>{{ cronForm.expr || '—' }}</code>
                    </div>
                    <div class="cron-preview" :class="{ invalid: !cronPreview.valid }">
                        <template v-if="cronPreview.loading">{{ t('解析中…') }}</template>
                        <template v-else-if="cronPreview.valid">
                            <span class="cron-preview-chip" v-for="(t, i) in cronPreview.next" :key="i">{{ t }}</span>
                            <span v-if="!cronPreview.next.length" class="cron-preview-chip">{{ t('@reboot 开机时执行一次') }}</span>
                        </template>
                        <template v-else>✗ {{ cronPreview.error || t('表达式无效') }}</template>
                    </div>
                </div>
                <div class="field">
                    <span>{{ t('执行命令') }} <em>*</em></span>
                    <textarea v-model="cronForm.command" rows="3" :placeholder="t('如：find /var/log -name \\'*.log\\' -mtime +7 -delete')" spellcheck="false"></textarea>
                </div>
            </div>
            <div class="modal-footer">
                <button class="toolbar-button" @click="closeCronForm">{{ t('取消') }}</button>
                <button class="primary-button" @click="submitCronForm()" :disabled="cronFormSaving">{{ cronFormSaving ? t('保存中…') : (cronForm.id ? t('保存修改') : t('写入 crontab')) }}</button>
            </div>
        </div>
    </div>
`,
};

export const ConfirmModal = {
    name: 'ConfirmModal',
    setup: () => appCtx(),
    template: `
    <div class="modal-overlay" @click.self="cancelConfirm">
        <div class="modal confirm-modal">
            <div class="modal-header">
                <div><span class="eyebrow">CONFIRM</span><h3>{{ confirmState.title }}</h3></div>
                <button class="icon-button" @click="cancelConfirm" :title="t('关闭')">×</button>
            </div>
            <div class="modal-body confirm-body">{{ confirmState.message }}</div>
            <div class="modal-footer">
                <button class="toolbar-button" @click="cancelConfirm">{{ t('取消') }}</button>
                <button class="primary-button" :class="{ 'danger-button': confirmState.danger }" @click="okConfirm">{{ confirmState.confirmText }}</button>
            </div>
        </div>
    </div>
`,
};
