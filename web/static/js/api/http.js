/**
 * http.js — HTTP 基础设施
 *
 * API 前缀、通用请求封装与会话失效广播，供各业务 API 模块复用。
 */

export const API_PREFIX = '/api/v1';

/** 广播会话失效事件（store 监听后回到登录页）。 */
export function notifyUnauthorized() {
    window.dispatchEvent(new CustomEvent('smp:unauthorized'));
}

/**
 * 通用 JSON 请求封装。
 * @param {string} path - API 路径（不含前缀）
 * @param {Object} [options] - fetch 选项
 * @returns {Promise<{code: number, message: string, data: any}>}
 * @throws {Error} 网络异常、未登录（401）或响应解析失败
 */
export async function request(path, options = {}) {
    const response = await fetch(API_PREFIX + path, options);
    if (response.status === 401) {
        notifyUnauthorized();
        throw new Error('未登录或会话已过期');
    }
    return response.json();
}
