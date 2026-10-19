import { appCtx } from '../store/index.js';

export const LoginView = {
    name: 'LoginView',
    setup: () => appCtx(),
    template: `
    <div class="auth-screen">
        <div class="auth-card">
            <div class="brand auth-brand">
                <span class="brand-mark"><img :src="'/api/v1/settings/favicon?' + faviconTs" :alt="t('站点图标')"></span>
                <div class="brand-copy">
                    <strong>{{ brandName }}</strong>
                    <span>{{ t('服务器运维面板') }}</span>
                </div>
            </div>
            <form class="auth-form" @submit.prevent="handleLogin">
                <label class="field"><span>{{ t('用户名') }}</span><input v-model="loginForm.username" autocomplete="username"></label>
                <label class="field"><span>{{ t('密码') }}</span><input type="password" v-model="loginForm.password" autocomplete="current-password"></label>
                <div class="auth-error" v-if="loginError">{{ loginError }}</div>
                <button class="primary-button auth-submit" type="submit" :disabled="loginLoading">{{ loginLoading ? t('登录中...') : t('登 录') }}</button>
            </form>
            <p class="auth-hint">{{ t('首次部署默认账户 admin / admin123') }}</p>
        </div>
    </div>
`,
};
