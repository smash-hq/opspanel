/**
 * i18n/index.js — 国际化核心
 *
 * 设计：以中文原文为键（source-as-key）。
 * - zh：原样返回（中文即源文案，零字典开销）；
 * - en：查 en 字典精确匹配，未命中再试模式规则，仍未命中回退原文（永不阻断显示）。
 *
 * locale 为响应式 ref：模板与计算属性中的 t() 随一键切换自动刷新，
 * 无需刷新页面。切换结果持久化到 localStorage 与后端设置（尽力而为）。
 *
 * 占位符：t('共 {n} 台节点', { n: 3 })，两种语言共用同一占位符名。
 */

const { ref } = window.Vue;

import { en, enPatterns } from './en.js';

const STORE_KEY = 'opspanel_lang';

function initLocale() {
    const saved = localStorage.getItem(STORE_KEY);
    // 默认英文：本机无显式选择时使用 en（登录后若后端设置有语言且本机未选择过，则跟随后端）
    return saved === 'zh' || saved === 'en' ? saved : 'en';
}

export const locale = ref(initLocale());

/** 用户是否在本机显式选择过语言（localStorage 有合法记录）。 */
export function hasLocalePreference() {
    const saved = localStorage.getItem(STORE_KEY);
    return saved === 'zh' || saved === 'en';
}

document.documentElement.lang = locale.value === 'en' ? 'en' : 'zh-CN';

const ZH_RE = /[\u4e00-\u9fff\u3000-\u303f\uff00-\uffef]/;

/** 依次尝试模式规则（处理含动态片段、无法精确枚举的文案），$1..$9 引用捕获组。 */
function applyPatterns(s) {
    for (const [re, tpl] of enPatterns) {
        const m = s.match(re);
        if (m) return tpl.replace(/\$(\d)/g, (_, i) => m[Number(i)] ?? '');
    }
    return s;
}

/**
 * 翻译：中文原文 → 当前语言文案。
 * @param {string} str - 中文源文案（含 {name} 占位符）
 * @param {Record<string, any>} [params] - 占位符取值
 * @returns {string}
 */
export function t(str, params) {
    if (str === null || str === undefined || str === '') return str;
    let s = String(str);
    if (locale.value === 'en' && ZH_RE.test(s)) {
        const hit = en[s];
        s = hit !== undefined ? hit : applyPatterns(s);
    }
    if (params) {
        s = s.replace(/\{(\w+)\}/g, (m, k) => (k in params ? (params[k] ?? m) : m));
    }
    return s;
}

/**
 * 切换语言并全页生效。
 * @param {string} lang - "zh" | "en"
 * @param {{persist?: boolean}} [opts] - persist=false 时不写 localStorage / 后端（用于跟随后端设置初始化）
 */
export function setLocale(lang, opts = {}) {
    if (lang !== 'zh' && lang !== 'en') return;
    const persist = opts.persist !== false;
    locale.value = lang;
    document.documentElement.lang = lang === 'en' ? 'en' : 'zh-CN';
    if (persist) {
        localStorage.setItem(STORE_KEY, lang);
        // 同步到后端设置（与主题切换同策略：本地已生效，失败不打断交互）
        import('../api/settings.js')
            .then(({ updateSettings }) => updateSettings({ language: lang }).catch(() => {}))
            .catch(() => {});
    }
    // 广播语言变化（如 topbar 文档标题等一次性渲染的内容需自行刷新）
    window.dispatchEvent(new CustomEvent('opspanel:langchange', { detail: lang }));
}
