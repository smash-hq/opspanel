/**
 * auth.js — 登录认证 API
 */

import { request } from './http.js';

/** 登录，成功返回 { username, must_change_password }。 */
export async function login(username, password) {
    const json = await request('/auth/login', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ username, password }),
    });
    return json.data;
}

/** 注销当前会话。 */
export async function logout() {
    const json = await request('/auth/logout', { method: 'POST' });
    return json.data;
}

/** 探测当前登录态，返回 { username, must_change_password }，未登录抛错。 */
export async function fetchMe() {
    const json = await request('/auth/me');
    return json.data;
}

/** 修改密码（校验旧密码），成功后清除首登改密标记。 */
export async function changePassword(oldPassword, newPassword) {
    const json = await request('/auth/change-password', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ old_password: oldPassword, new_password: newPassword }),
    });
    return json.data;
}
