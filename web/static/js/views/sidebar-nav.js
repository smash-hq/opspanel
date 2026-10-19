import { appCtx } from '../store/index.js';

export const SidebarNav = {
    name: 'SidebarNav',
    setup: () => appCtx(),
    template: `
    <aside class="sidebar" :class="{ collapsed: sidebarCollapsed }">
        <div class="sidebar-header">
            <div class="brand">
                <span class="brand-mark"><img :src="'/api/v1/settings/favicon?' + faviconTs" :alt="t('站点图标')"></span>
                <div class="brand-copy" v-show="!sidebarCollapsed">
                    <strong>{{ brandName }}</strong>
                    <span>{{ t('服务器运维面板') }}</span>
                </div>
            </div>
            <button class="icon-button collapse-toggle" @click="sidebarCollapsed = !sidebarCollapsed" :title="sidebarCollapsed ? t('展开侧栏') : t('收起侧栏')">
                <span class="collapse-icon">{{ sidebarCollapsed ? '⟩' : '⟨' }}</span>
            </button>
        </div>

        <!-- ═══ 上半部分：节点管理 ═══ -->
        <div class="sidebar-section sidebar-section-top">
            <div class="sidebar-content" v-show="!sidebarCollapsed">
                <div class="search-box">
                    <span class="search-icon">⌕</span>
                    <input v-model="searchQuery" :placeholder="t('搜索名称、地址...')" :aria-label="t('搜索服务器')">
                    <kbd v-if="!searchQuery">/</kbd>
                    <button v-else class="clear-search" @click="searchQuery = ''" :title="t('清除搜索')">×</button>
                </div>
                <div class="filter-tabs">
                    <button v-for="filter in filters" :key="filter.value" :class="{ active: activeFilter === filter.value }" @click="activeFilter = filter.value">
                        {{ filter.label }} <span>{{ filterCount(filter.value) }}</span>
                    </button>
                </div>
            </div>
            <div class="server-list" :class="{ compact: sidebarCollapsed }">
                <div v-for="s in filteredServers" :key="s.id" class="server-item" :class="{ active: activeMenu === 'main' && selectedServer === s.id, 'local-item': s.type === 'local' }" @click="selectServer(s.id)">
                    <span class="server-avatar" :class="s.type">{{ s.type === 'remote' ? '↗' : '⌂' }}</span>
                    <span class="server-item-copy" v-show="!sidebarCollapsed"><strong>{{ s.name }}</strong><small>{{ s.type === 'local' ? t('当前管理节点') : serverAddress(s) }}</small></span>
                    <span class="status-dot" :class="serverStatus[s.id] || 'unknown'"></span>
                    <span class="item-actions" v-show="!sidebarCollapsed" @click.stop>
                        <button @click="openEdit(s)" :title="t('编辑节点')">✎</button>
                        <button class="danger" @click="deleteServer(s.id)" :title="t('删除节点')" :disabled="s.type === 'local'">×</button>
                    </span>
                    <div class="server-item-tooltip" v-if="sidebarCollapsed">{{ s.name }} · {{ s.type === 'local' ? t('当前管理节点') : serverAddress(s) }} {{ statusText(serverStatus[s.id]) }}</div>
                </div>
                <div v-if="!filteredServers.length && !sidebarCollapsed" class="list-empty">
                    <span class="empty-icon">⌁</span>
                    <strong>{{ t('没有匹配的节点') }}</strong>
                    <small>{{ t('尝试修改搜索条件') }}</small>
                </div>
            </div>
            <button class="add-button" @click="openAdd" :class="{ 'compact-add': sidebarCollapsed }" :title="t('添加节点')">
                <span class="add-icon">＋</span>
                <b v-show="!sidebarCollapsed">{{ t('添加节点') }}</b>
            </button>
        </div>

        <!-- ═══ 下半部分：通知中心 / 通用配置 ═══ -->
        <div class="sidebar-bottom">
            <button class="sidebar-config-btn" :class="{ active: activeMenu === 'notifycenter' }" @click="openNotifyCenter">
                <span class="config-icon">
                    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                        <path d="M18 8A6 6 0 0 0 6 8c0 7-3 9-3 9h18s-3-2-3-9"/>
                        <path d="M13.73 21a2 2 0 0 1-3.46 0"/>
                    </svg>
                </span>
                <span class="config-label" v-show="!sidebarCollapsed">{{ t('通知中心') }}</span>
                <span v-show="!sidebarCollapsed && unreadCount > 0" class="sidebar-nc-badge">{{ unreadCount > 99 ? '99+' : unreadCount }}</span>
                <span class="config-arrow" v-show="!sidebarCollapsed">›</span>
            </button>
            <button class="sidebar-config-btn" :class="{ active: activeMenu === 'config' }" @click="activeMenu = 'config'">
                <span class="config-icon">
                    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                        <circle cx="12" cy="12" r="3"/>
                        <path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 0 1 0 2.83 2 2 0 0 1-2.83 0l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-2 2 2 2 0 0 1-2-2v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 0 1-2.83 0 2 2 0 0 1 0-2.83l.06-.06a1.65 1.65 0 0 0 .33-1.82 1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1-2-2 2 2 0 0 1 2-2h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 0 1 0-2.83 2 2 0 0 1 2.83 0l.06.06a1.65 1.65 0 0 0 1.82.33H9a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 2-2 2 2 0 0 1 2 2v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 0 1 2.83 0 2 2 0 0 1 0 2.83l-.06.06a1.65 1.65 0 0 0-.33 1.82V9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 2 2 2 2 0 0 1-2 2h-.09a1.65 1.65 0 0 0-1.51 1z"/>
                    </svg>
                </span>
                <span class="config-label" v-show="!sidebarCollapsed">{{ t('通用配置') }}</span>
                <span class="config-arrow" v-show="!sidebarCollapsed">›</span>
            </button>
            <div class="sidebar-meta" v-show="!sidebarCollapsed"><span class="live-dot"></span><span>{{ t('本地控制面在线') }}</span><span class="version">v0.1</span></div>
        </div>
    </aside>
`,
};
