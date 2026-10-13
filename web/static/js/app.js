/**
 * app.js — Vue 应用入口
 *
 * 唯一的职责：导入 store 工厂，创建 Vue 实例，挂载到 #app。
 * 业务逻辑按域拆分在 js/store/ 目录中，工具函数在 js/utils.js 中。
 *
 * Vue 3 以 vendor 方式本地内嵌（vendor/vue/vue.global.prod.js，经 index.html 全局加载），
 * 此处直接使用全局 Vue 对象（模块脚本 deferred 执行，此时全局已就绪）。
 */

import { setupStore } from './store/index.js';

window.Vue.createApp({
    setup() {
        return setupStore();
    },
}).mount('#app');
