/**
 * http.js — HTTP 基础设施
 *
 * API 前缀、统一错误类型（ApiError）与通用请求封装。
 * 所有接口错误在此收口：业务错误、非 JSON 响应（网关错误页/panic）、
 * 网络中断、会话失效均被规范化为 ApiError 抛出，
 * message = 规范化提示 + 底层 detail（运维排障所需的原始错误）。
 */
import { t } from '../i18n/index.js';


export const API_PREFIX = '/api/v1';

/** 广播会话失效事件（store 监听后回到登录页）。 */
export function notifyUnauthorized() {
    window.dispatchEvent(new CustomEvent('opspanel:unauthorized'));
}

/**
 * API 错误类型。
 * message 兼容 Error 标准用法（toast 直接可用），
 * detail 保留底层原始错误（SSH/命令输出等），两者在 message 中以换行组合。
 */
export class ApiError extends Error {
    /**
     * @param {string} message - 面向用户的规范化提示
     * @param {string} [detail] - 底层原始错误详情
     * @param {number} [status] - HTTP 状态码（网络异常为 0）
     * @param {number} [code] - 后端业务错误码
     */
    constructor(message, detail = '', status = 0, code = 0) {
        super(detail ? `${message}\n${detail}` : message);
        this.name = 'ApiError';
        this.status = status;
        this.code = code;
        this.detail = detail;
    }
}

/** HTTP 状态码 → 专业提示。 */
const HTTP_MESSAGES = {
    400: t('请求参数不合法'),
    401: t('未登录或会话已过期'),
    403: t('没有执行该操作的权限'),
    404: t('接口不存在，请检查请求地址或升级面板'),
    405: t('请求方法不支持'),
    408: t('请求超时，请稍后重试'),
    413: t('请求数据超出大小限制'),
    429: t('请求过于频繁，请稍后重试'),
    500: t('服务器内部错误，请稍后重试或查看服务日志'),
    502: t('网关或上游服务异常'),
    503: t('服务暂时不可用，请稍后重试'),
    504: t('上游服务响应超时，请稍后重试'),
};

/**
 * 将非信封响应（网关 HTML 错误页、纯文本 panic 等）包装为 ApiError。
 * detail 清洗 HTML 标签并截断，避免错误页刷屏。
 */
async function toApiError(response, text) {
    const status = response ? response.status : 0;
    const message = (response && HTTP_MESSAGES[status])
        || (status ? t('服务响应异常（HTTP {status}）', { status: status }) : t('网络连接失败，请检查面板服务是否正常运行'));
    let detail = (text || '').replace(/<[^>]+>/g, ' ').replace(/\s+/g, ' ').trim();
    if (detail.length > 300) detail = detail.slice(0, 300) + '…';
    return new ApiError(message, detail, status);
}

/**
 * 通用 JSON 请求封装：所有接口错误统一在此收口。
 * @param {string} path - API 路径（不含前缀）
 * @param {Object} [options] - fetch 选项
 * @returns {Promise<{code: number, message: string, detail?: string, data: any}>} 成功时返回响应信封
 * @throws {ApiError} 业务错误（code !== 0）、网络异常、非 JSON 响应或会话失效
 */
export async function request(path, options = {}) {
    let response;
    try {
        response = await fetch(API_PREFIX + path, options);
    } catch (err) {
        if (err && err.name === 'AbortError') throw err; // 主动取消不包装
        throw new ApiError(t('网络连接失败，请检查面板服务是否正常运行'));
    }

    if (response.status === 401) {
        notifyUnauthorized();
        let text = '';
        try { text = await response.text(); } catch { /* 忽略读取失败 */ }
        throw await toApiError(response, text);
    }

    // 统一先读文本再解析：兜住网关 HTML 错误页、panic 纯文本等非 JSON 响应
    let text = '';
    try { text = await response.text(); } catch { /* 忽略读取失败 */ }

    let json = null;
    try { json = text ? JSON.parse(text) : null; } catch { json = null; }

    if (!json || typeof json.code !== 'number') {
        throw await toApiError(response, text);
    }
    if (json.code !== 0) {
        throw new ApiError(t(json.message) || t('操作失败'), json.detail || '', response.status, json.code);
    }
    return json;
}
