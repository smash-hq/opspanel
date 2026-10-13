/**
 * api/index.js — REST API 客户端聚合出口
 *
 * 按业务域拆分的各模块在此统一聚合，
 * 调用方 `import { fetchServers, ... } from './api/index.js'` 即可获得与
 * 原 api.js 完全一致的命名导出。
 */

export * from './http.js';
export * from './auth.js';
export * from './servers.js';
export * from './software.js';
export * from './packages.js';
export * from './sources.js';
export * from './languages.js';
export * from './files.js';
export * from './docker.js';
export * from './mirrors.js';
export * from './processes.js';
