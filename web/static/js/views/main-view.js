import { appCtx } from '../store/index.js';

export const MainView = {
    name: 'MainView',
    setup: () => appCtx(),
    template: `
    <main class="main">
        <top-bar></top-bar>
        <div class="content">
            <config-page v-if="activeMenu === 'config'"></config-page>
            <notify-center v-else-if="activeMenu === 'notifycenter'"></notify-center>
            <empty-state-view v-else-if="!selectedServer"></empty-state-view>
            <node-detail v-else></node-detail>
        </div>
    </main>
`,
};
