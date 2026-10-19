import { appCtx } from '../../store/index.js';

export const TabTerminal = {
    name: 'TabTerminal',
    setup: () => appCtx(),
    template: `
                <div class="node-tab-content">
                    <div class="terminal-toolbar">
                        <span class="terminal-host">⌨ {{ currentServerName }}</span>
                        <span class="terminal-status" :class="terminalStatus"><i></i>{{ terminalStatusText }}</span>
                        <span class="terminal-toolbar-spacer"></span>
                        <button class="toolbar-button" v-if="terminalStatus === 'connected'" @click="disconnectTerminal">{{ t('⏻ 断开') }}</button>
                        <button class="toolbar-button" v-else @click="reconnectTerminal">{{ t('↻ 重新连接') }}</button>
                    </div>
                    <div class="terminal-layout">
                        <div class="terminal-wrap" ref="terminalMount"></div>
                        <aside class="cmd-panel">
                            <div class="cmd-panel-title">{{ t('常用命令') }}</div>
                            <input class="cmd-search" v-model="cmdSearch" :placeholder="t('搜索命令或说明...')" />
                            <div class="cmd-groups">
                                <div class="cmd-group" v-for="g in visibleCmdGroups" :key="g.id">
                                    <button class="cmd-group-head" @click="toggleCmdGroup(g.id)">
                                        <span>{{ g.name }}</span>
                                        <span class="cmd-group-arrow" :class="{ open: g.open }">▸</span>
                                    </button>
                                    <div class="cmd-group-body" v-show="g.open">
                                        <template v-if="g.subgroups">
                                            <div class="cmd-group cmd-subgroup" v-for="s in g.subgroups" :key="s.id">
                                                <button class="cmd-group-head" @click="toggleCmdGroup(s.id)">
                                                    <span>{{ s.name }}</span>
                                                    <span class="cmd-group-arrow" :class="{ open: s.open }">▸</span>
                                                </button>
                                                <div class="cmd-group-body" v-show="s.open">
                                                    <button class="cmd-item" v-for="item in s.commands" :key="item.c" @click="sendCommand(item.c)" :title="item.c">
                                                        <code>{{ item.c }}</code>
                                                        <span class="cmd-item-desc">{{ item.d }}</span>
                                                    </button>
                                                </div>
                                            </div>
                                        </template>
                                        <template v-else>
                                            <button class="cmd-item" v-for="item in g.commands" :key="item.c" @click="sendCommand(item.c)" :title="item.c">
                                                <code>{{ item.c }}</code>
                                                <span class="cmd-item-desc">{{ item.d }}</span>
                                            </button>
                                        </template>
                                    </div>
                                </div>
                                <div class="cmd-empty" v-if="!visibleCmdGroups.length">{{ t('无匹配命令') }}</div>
                            </div>
                        </aside>
                    </div>
                </div>
`,
};
