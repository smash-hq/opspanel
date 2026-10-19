/**
 * utils.js — 纯函数工具库
 *
 * 提供格式化、转换等无副作用的工具函数。
 * 不依赖 Vue 响应式系统，可在任意上下文中使用。
 */

/**
 * 数字格式化：保留 1 位小数。
 * @param {number} value - 原始数值
 * @returns {string} 格式化后的字符串，如 "42.3"
 */
export function number(value) {
    return Number(value || 0).toFixed(1);
}

/**
 * 百分比边界限制：将值限制在 [0, 100] 区间。
 * @param {number} value - 原始百分比
 * @returns {number} 限制后的数值
 */
export function bounded(value) {
    return Math.max(0, Math.min(100, Number(value || 0)));
}

/**
 * 字节数转可读单位（B / KB / MB / GB / TB）。
 * @param {number} bytes - 字节数
 * @returns {string} 可读字符串，如 "12.5 GB"；无法转换时返回 "—"
 */
export function fmtBytes(bytes) {
    if (bytes === undefined || bytes === null || Number.isNaN(Number(bytes))) return '—';
    const units = ['B', 'KB', 'MB', 'GB', 'TB'];
    let value = Number(bytes), index = 0;
    while (value >= 1024 && index < units.length - 1) { value /= 1024; index++; }
    return `${value.toFixed(value < 10 && index > 0 ? 2 : 1)} ${units[index]}`;
}

/**
 * 运行秒数转可读时长。
 * @param {number} seconds - 运行秒数
 * @returns {string} 可读字符串，如 "3天 5时"；无法转换时返回 "—"
 */
export function fmtUptime(seconds) {
    if (!seconds) return '—';
    const days = Math.floor(seconds / 86400);
    const hours = Math.floor((seconds % 86400) / 3600);
    const minutes = Math.floor((seconds % 3600) / 60);
    if (days) return `${days}天 ${hours}时`;
    if (hours) return `${hours}时 ${minutes}分`;
    return `${minutes}分`;
}

/**
 * 磁盘用量等级 → CSS 类名。
 * @param {number} percent - 用量百分比
 * @returns {string} "critical-fill" | "warning-fill" | "green-fill"
 */
export function usageClass(percent) {
    return Number(percent) >= 80 ? 'critical-fill'
        : Number(percent) >= 60 ? 'warning-fill'
            : 'green-fill';
}

/**
 * 连接状态码 → 中文描述。
 * @param {string} status - "ok" | "loading" | "error" | "unknown"
 * @returns {string} 中文状态文本
 */
export function statusText(status) {
    return status === 'ok' ? '在线'
        : status === 'loading' ? '连接中'
            : status === 'error' ? '离线'
                : '未检查';
}

/**
 * 拼接服务器地址字符串。
 * @param {Object} server - 服务器对象
 * @returns {string} 如 "192.168.1.100:22" 或 "localhost"
 */
export function serverAddress(server) {
    return server.type === 'local' ? 'localhost'
        : `${server.host || '未配置'}:${server.port || 22}`;
}

/**
 * 显示 Toast 通知。
 * @param {Object} toast - Vue reactive 对象 { visible, message, error }
 * @param {string} message - 通知文本（可含换行，CSS pre-line 渲染多行）
 * @param {boolean} [error=false] - 是否为错误样式
 * @returns {number} setTimeout ID，供清理使用
 */
export function showToast(toast, message, error = false) {
    toast.message = message;
    toast.error = error;
    toast.visible = true;
    // 错误信息驻留更久，给运维人员足够的阅读时间
    return setTimeout(() => { toast.visible = false; }, error ? 6000 : 2800);
}

/**
 * 解析逗号/空格分隔的版本串为数组（去空、去重）。
 * @param {string} str - 原始版本串，如 "1.20, 1.21  1.20"
 * @returns {string[]} 去重后的版本数组
 */
export function parseVersions(str) {
    const seen = new Set();
    return (str || '').split(/[,，\s]+/).map(s => s.trim()).filter(Boolean).filter(v => !seen.has(v) && seen.add(v));
}
