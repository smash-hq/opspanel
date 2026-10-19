import { appCtx } from '../../store/index.js';

export const TabGpu = {
    name: 'TabGpu',
    setup: () => appCtx(),
    template: `
                <div class="node-tab-content">
                    <div class="gpu-toolbar">
                        <h3>{{ t('GPU / NPU 设备') }}
                            <span v-if="gpuSnapshot && gpuSnapshot.vendor_name" class="gpu-vendor-badge">{{ gpuSnapshot.vendor_name }}</span>
                        </h3>
                        <span class="gpu-toolbar-hint">{{ t('SSH 实时按需拉取，非周期采样') }}</span>
                        <span class="gpu-toolbar-spacer"></span>
                        <button class="toolbar-button" @click="loadGpu(true)" :disabled="gpuLoading">{{ t('↻ 刷新') }}</button>
                    </div>
                    <div v-if="gpuLoading" class="loading-state" style="padding:20px;">{{ t('采集中（经 SSH 拉取厂商工具输出，约需数秒）...') }}</div>
                    <div v-else-if="gpuError" class="gpu-error-state">
                        <span class="empty-icon">⚠</span>
                        <strong>{{ gpuError }}</strong>
                        <p>{{ t('请确认目标服务器已安装厂商管理工具（npu-smi / cnmon），且账号有执行权限') }}</p>
                        <button class="toolbar-button" @click="loadGpu(true)">{{ t('重试') }}</button>
                    </div>
                    <div v-else-if="!gpuDevices.length" class="list-empty">
                        <span class="empty-icon">🧩</span>
                        <strong>{{ t('未在该服务器上发现 GPU / NPU 设备') }}</strong>
                        <p>{{ t('已探测 npu-smi（华为昇腾）与 cnmon（寒武纪），更多厂商适配中') }}</p>
                    </div>
                    <template v-else>
                        <div class="gpu-grid">
                            <div v-for="dev in gpuDevices" :key="dev.index" class="gpu-card">
                                <div class="gpu-card-head">
                                    <span class="gpu-index">#{{ dev.index }}</span>
                                    <span class="gpu-name" :title="dev.name">{{ dev.name || t('未知型号') }}</span>
                                    <span v-if="dev.health" class="gpu-health" :class="gpuHealthClass(dev.health)">{{ dev.health }}</span>
                                </div>
                                <div class="gpu-rows">
                                    <div class="gpu-row">
                                        <span class="gpu-row-label">{{ t('核心利用率') }}</span>
                                        <div class="gpu-bar"><i :class="gpuUtilClass(dev.core_util)" :style="{ width: gpuPercent(dev.core_util) + '%' }"></i></div>
                                        <span class="gpu-row-value">{{ dev.core_util !== '' ? dev.core_util + '%' : '—' }}</span>
                                    </div>
                                    <div class="gpu-row">
                                        <span class="gpu-row-label">{{ t('显存') }}</span>
                                        <div class="gpu-bar"><i :style="{ width: gpuMemPercent(dev) + '%' }"></i></div>
                                        <span class="gpu-row-value" :title="gpuMemText(dev)">{{ gpuMemText(dev) }}</span>
                                    </div>
                                </div>
                                <div class="gpu-card-foot">
                                    <span>{{ t('功耗 {p1}', { p1: dev.power_w ? dev.power_w + ' W' : '—' }) }}</span>
                                    <span :class="gpuTempClass(dev.temp_c)">{{ t('温度 {p1}', { p1: dev.temp_c ? dev.temp_c + '°C' : '—' }) }}</span>
                                </div>
                            </div>
                        </div>
                        <div v-if="gpuSnapshot && gpuSnapshot.error" class="gpu-warn-bar">{{ gpuSnapshot.error }}</div>
                    </template>
                </div>
`,
};
