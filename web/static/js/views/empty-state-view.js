import { appCtx } from '../store/index.js';

export const EmptyStateView = {
    name: 'EmptyStateView',
    setup: () => appCtx(),
    template: `
            <section class="empty-state-view">
                <div class="empty-state-icon">🖥️</div>
                <h2>{{ t('选择一台服务器开始管理') }}</h2>
                <p>{{ t('从左侧节点列表中选择一台服务器，查看系统信息、管理软件和服务。') }}</p>
                <div class="empty-state-hint">{{ t('💡 提示：点击左侧节点进行选择') }}</div>
                <div class="welcome-text" v-if="welcomeMessage">{{ welcomeMessage }}</div>
            </section>
`,
};
