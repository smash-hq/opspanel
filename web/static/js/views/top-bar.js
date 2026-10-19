import { appCtx } from '../store/index.js';

export const TopBar = {
    name: 'TopBar',
    setup: () => appCtx(),
    template: `
        <header class="topbar">
            <div class="breadcrumb">
                <template v-if="activeMenu === 'config'">
                    <span>{{ t('控制台') }}</span><b>/</b><strong>{{ t('通用配置') }}</strong>
                </template>
                <template v-else-if="activeMenu === 'notifycenter'">
                    <span>{{ t('控制台') }}</span><b>/</b><strong>{{ t('通知中心') }}</strong>
                </template>
                <template v-else-if="selectedServer">
                    <span>{{ t('控制台') }}</span><b>/</b><strong>{{ currentServerName }}</strong>
                </template>
                <template v-else>
                    <span>{{ t('控制台') }}</span><b>/</b><strong>{{ t('选择节点') }}</strong>
                </template>
            </div>
            <div class="topbar-actions">
                <!-- 站内通知 -->
                <div class="notify-wrap">
                    <button class="topbar-icon-btn notify-bell" :class="{ active: notifyPanelVisible }" @click="toggleNotifyPanel()" :title="t('通知中心')">
                        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                            <path d="M18 8A6 6 0 0 0 6 8c0 7-3 9-3 9h18s-3-2-3-9"/>
                            <path d="M13.73 21a2 2 0 0 1-3.46 0"/>
                        </svg>
                        <span v-if="unreadCount > 0" class="notify-badge">{{ unreadCount > 99 ? '99+' : unreadCount }}</span>
                    </button>
                    <div v-if="notifyPanelVisible" class="notify-panel">
                        <div class="notify-panel-head">
                            <strong>{{ t('通知中心') }}</strong>
                            <div class="notify-panel-actions">
                                <button v-if="unreadCount > 0" class="notify-link-btn" @click="markAllRead()">{{ t('全部已读') }}</button>
                                <button class="notify-link-btn" @click="clearAllNotifications()">{{ t('清空') }}</button>
                                <button class="notify-link-btn" @click="closeNotifyPanel()" :title="t('关闭')">✕</button>
                            </div>
                        </div>
                        <div v-if="notifyLoading && !notifications.length" class="notify-empty">{{ t('加载中...') }}</div>
                        <div v-else-if="!notifications.length" class="notify-empty">
                            <span class="notify-empty-icon"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"><path d="M18 8A6 6 0 0 0 6 8c0 7-3 9-3 9h18s-3-2-3-9"/><path d="M13.73 21a2 2 0 0 1-3.46 0"/></svg></span>{{ t('暂无通知') }}
                        </div>
                        <div v-else class="notify-list">
                            <div v-for="n in notifications" :key="n.id" class="notify-item" :class="[n.level, { read: n.read }]" @click="openNotification(n)" :title="t('点击查看详情')">
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
                </div>
                <a class="topbar-icon-btn github-link" href="https://github.com/smash-hq/opspanel" target="_blank" rel="noopener noreferrer" :title="t('GitHub 仓库')">
                    <svg viewBox="0 0 16 16" fill="currentColor" aria-hidden="true"><path d="M8 0C3.58 0 0 3.58 0 8c0 3.54 2.29 6.53 5.47 7.59.4.07.55-.17.55-.38 0-.19-.01-.82-.01-1.49-2.01.37-2.53-.49-2.69-.94-.09-.23-.48-.94-.82-1.13-.28-.15-.68-.52-.01-.53.63-.01 1.08.58 1.23.82.72 1.21 1.87.87 2.33.66.07-.52.28-.87.51-1.07-1.78-.2-3.64-.89-3.64-3.95 0-.87.31-1.59.82-2.15-.08-.2-.36-1.02.08-2.12 0 0 .67-.21 2.2.82.64-.18 1.32-.27 2-.27s1.36.09 2 .27c1.53-1.04 2.2-.82 2.2-.82.44 1.1.16 1.92.08 2.12.51.56.82 1.27.82 2.15 0 3.07-1.87 3.75-3.65 3.95.29.25.54.73.54 1.48 0 1.07-.01 1.93-.01 2.2 0 .21.15.46.55.38A8.01 8.01 0 0 0 16 8c0-4.42-3.58-8-8-8Z"/></svg>
                </a>
                <button class="topbar-icon-btn" @click="showDoc = true" :title="t('使用文档')">📖</button>
                <select class="theme-select" v-model="themeVal" @change="setTheme(themeVal)" :title="t('切换主题')">
                    <option value="light">{{ t('☀️ 白天') }}</option>
                    <option value="dark">{{ t('🌙 夜间') }}</option>
                    <option value="system">{{ t('💻 跟随') }}</option>
                </select>
                <select class="theme-select lang-select" :value="lang" @change="setLang($event.target.value)" :title="t('切换语言')" aria-label="Language">
                    <option value="zh">中文</option>
                    <option value="en">English</option>
                </select>
                <div class="user-menu">
                    <button class="user-menu-trigger" :class="{ open: userMenuOpen }" @click="toggleUserMenu()" :title="currentUser">
                        <span class="user-avatar">
                            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                                <path d="M20 21v-2a4 4 0 0 0-4-4H8a4 4 0 0 0-4 4v2"/>
                                <circle cx="12" cy="7" r="4"/>
                            </svg>
                        </span>
                        <span class="user-menu-name">{{ currentUser }}</span>
                        <svg class="user-menu-caret" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="6 9 12 15 18 9"/></svg>
                    </button>
                    <transition name="user-menu-fade">
                        <div v-if="userMenuOpen" class="user-menu-dropdown">
                            <div class="user-menu-head">
                                <span class="user-avatar sm">
                                    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                                        <path d="M20 21v-2a4 4 0 0 0-4-4H8a4 4 0 0 0-4 4v2"/>
                                        <circle cx="12" cy="7" r="4"/>
                                    </svg>
                                </span>
                                <div class="user-menu-head-info">
                                    <strong>{{ currentUser }}</strong>
                                    <small>{{ t('已登录') }}</small>
                                </div>
                            </div>
                            <button class="user-menu-item" @click="openPwModal()">
                                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                                    <rect x="3" y="11" width="18" height="11" rx="2" ry="2"/>
                                    <path d="M7 11V7a5 5 0 0 1 10 0v4"/>
                                </svg>
                                <span>{{ t('修改密码') }}</span>
                            </button>
                            <button class="user-menu-item danger" @click="handleLogout()">
                                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                                    <path d="M9 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h4"/>
                                    <polyline points="16 17 21 12 16 7"/>
                                    <line x1="21" y1="12" x2="9" y2="12"/>
                                </svg>
                                <span>{{ t('退出登录') }}</span>
                            </button>
                        </div>
                    </transition>
                </div>
            </div>
        </header>
`,
};
