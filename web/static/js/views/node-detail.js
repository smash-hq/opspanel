import { appCtx } from '../store/index.js';

export const NodeDetail = {
    name: 'NodeDetail',
    setup: () => appCtx(),
    template: `
    <section class="node-detail">
        <!-- 节点面包屑 Tab -->
                <div class="node-breadcrumb">
                    <button class="bc-tab" :class="{ active: activeTab === 'system' }" @click="activeTab = 'system'">{{ t('系统信息') }}</button>
                    <button class="bc-tab" :class="{ active: activeTab === 'process' }" @click="activeTab = 'process'; loadProcesses()">{{ t('进程管理') }}</button>
                    <button class="bc-tab" :class="{ active: activeTab === 'services' }" @click="activeTab = 'services'; loadServices()">systemd</button>
                    <button class="bc-tab" :class="{ active: activeTab === 'files' }" @click="activeTab = 'files'; currentFilePath = '/'; loadFiles('/')">{{ t('文件管理') }}</button>
                    <button class="bc-tab" :class="{ active: activeTab === 'languages' }" @click="activeTab = 'languages'; detectServerLanguages()">{{ t('语言环境') }}</button>
                    <button class="bc-tab" :class="{ active: activeTab === 'software' }" @click="activeTab = 'software'">{{ t('软件管理') }}</button>
                    <button class="bc-tab" :class="{ active: activeTab === 'apps' }" @click="onAppsTabClick()">{{ t('服务部署') }}</button>
                    <button class="bc-tab" :class="{ active: activeTab === 'docker', disabled: !dockerTabEnabled }" @click="onDockerTabClick()" :title="dockerTabEnabled ? '' : t('未检测到 Docker 环境，点击重新检测')">Docker</button>
                    <button class="bc-tab" :class="{ active: activeTab === 'terminal', disabled: selectedServer === 'srv_local' || !selectedServer }" @click="openTerminalTab()" :title="selectedServer === 'srv_local' ? t('本地节点暂不支持终端') : ''">Terminal</button>
                    <button class="bc-tab" :class="{ active: activeTab === 'cron' }" @click="activeTab = 'cron'; loadCronJobs()">{{ t('定时任务') }}</button>
                    <button class="bc-tab" :class="{ active: activeTab === 'gpu' }" @click="activeTab = 'gpu'; loadGpu()">GPU</button>
                    <button class="bc-tab disabled" :title="t('即将推出')">{{ t('集群管理') }}</button>
                    <button class="bc-tab disabled" :title="t('即将推出')">{{ t('安全') }}</button>
                </div>

        <!-- Tab 面板（由 activeTab 控制切换） -->
        <tab-system v-if="activeTab === 'system'"></tab-system>
        <tab-software v-if="activeTab === 'software'"></tab-software>
        <tab-languages v-if="activeTab === 'languages'"></tab-languages>
        <tab-process v-if="activeTab === 'process'"></tab-process>
        <tab-services v-if="activeTab === 'services'"></tab-services>
        <tab-files v-if="activeTab === 'files'"></tab-files>
        <tab-apps v-if="activeTab === 'apps'"></tab-apps>
        <tab-docker v-if="activeTab === 'docker'"></tab-docker>
        <tab-terminal v-if="activeTab === 'terminal'"></tab-terminal>
        <tab-cron v-if="activeTab === 'cron'"></tab-cron>
        <tab-gpu v-if="activeTab === 'gpu'"></tab-gpu>
    </section>
`,
};
