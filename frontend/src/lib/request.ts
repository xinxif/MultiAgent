/**
 * HTTP 请求层 —— 基于 fetch 的统一封装。
 *
 * 职责：
 * 1. 拼统一响应包络（Envelope），code != 0 抛 ApiError；
 * 2. 自动携带 Authorization: Bearer <accessToken>；
 * 3. 401 时静默调 /auth/refresh 换新并重放原请求（单飞，防并发刷新风暴）；
 * 4. 刷新失败 → 清登录态并跳登录页。
 *
 * 与后端语义对齐：access 放内存，refresh 放 localStorage。
 */
import type { Envelope } from '@/types/api';

const BASE = '/api/v1';

/* ---- Token 存取：access 内存态由 auth store 持有，这里通过注入回调解耦 ---- */

let getAccessToken: () => string | undefined = () => undefined;
let onAuthExpired: () => void = () => undefined;
let onTokenRefreshed: (token: string) => void = () => undefined;

/** 由 auth store 在启动时注入，避免循环依赖 */
export function bindAuthHooks(
  getter: () => string | undefined,
  expired: () => void,
  refreshed: (token: string) => void,
) {
  getAccessToken = getter;
  onAuthExpired = expired;
  onTokenRefreshed = refreshed;
}

/* ---- 刷新单飞：并发 401 只发一次 refresh，其余排队等结果 ---- */

let refreshing: Promise<string | null> | null = null;

/** 用 refresh_token 换新 access token；失败返回 null */
async function doRefresh(): Promise<string | null> {
  const refreshToken = localStorage.getItem('agent.refresh');
  if (!refreshToken) return null;
  try {
    const res = await fetch(`${BASE}/auth/refresh`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ refresh_token: refreshToken }),
    });
    const env = (await res.json()) as Envelope<{
      access_token: string;
      refresh_token: string;
    }>;
    if (res.status !== 200 || env.code !== 0 || !env.data) return null;
    localStorage.setItem('agent.refresh', env.data.refresh_token);
    // 同步更新 auth store 的内存 token，后续请求直接用新值
    onTokenRefreshed(env.data.access_token);
    return env.data.access_token;
  } catch {
    return null;
  }
}

/** 单飞入口 */
function refreshOnce(): Promise<string | null> {
  refreshing ??= doRefresh().finally(() => {
    refreshing = null;
  });
  return refreshing;
}

/* ---- 业务错误 ---- */

export class ApiError extends Error {
  /** 业务错误码（非 0），与后端 errcode 分段一致 */
  readonly code: number;
  /** HTTP 状态码 */
  readonly status: number;

  constructor(code: number, message: string, status: number) {
    super(message);
    this.name = 'ApiError';
    this.code = code;
    this.status = status;
  }
}

/* ---- 核心请求 ---- */

interface RequestOptions {
  method?: 'GET' | 'POST' | 'PUT' | 'DELETE';
  /** JSON 请求体（自动序列化；上传场景传 FormData） */
  body?: unknown;
  /** 追加查询参数 */
  query?: Record<string, string | number | boolean | undefined>;
  /** 401 时是否尝试刷新重放（登录/刷新接口本身要关掉） */
  retry401?: boolean;
  /** 上传：跳过 Content-Type 让浏览器带 boundary */
  formData?: FormData;
  /** 中断信号（SSE 之外的普通请求一般用不到） */
  signal?: AbortSignal;
}

/** 拼查询串：跳过 undefined */
function buildQuery(query?: RequestOptions['query']): string {
  if (!query) return '';
  const qs = new URLSearchParams();
  for (const [k, v] of Object.entries(query)) {
    if (v !== undefined && v !== '') qs.set(k, String(v));
  }
  const s = qs.toString();
  return s ? `?${s}` : '';
}

/**
 * 统一请求函数。
 * @returns data 字段内容（包络已拆开）
 */
export async function request<T>(
  path: string,
  options: RequestOptions = {},
): Promise<T> {
  const { method = 'GET', body, query, retry401 = true, formData, signal } = options;

  const headers: Record<string, string> = {};
  const token = getAccessToken();
  if (token) headers.Authorization = `Bearer ${token}`;

  let payload: BodyInit | undefined;
  if (formData) {
    payload = formData; // 浏览器自动设置 multipart boundary
  } else if (body !== undefined) {
    headers['Content-Type'] = 'application/json';
    payload = JSON.stringify(body);
  }

  const res = await fetch(
    `${BASE}${path}${buildQuery(query)}`,
    { method, headers, body: payload, signal },
  );

  // 401 → 静默刷新并重放一次（仅带凭证的业务请求）
  if (res.status === 401 && retry401) {
    const newToken = await refreshOnce();
    if (newToken) {
      return request<T>(path, { ...options, retry401: false });
    }
    onAuthExpired();
    throw new ApiError(20101, '登录已过期', 401);
  }

  // 后端统一返回 JSON 包络；网络层错误兜底
  let env: Envelope<T>;
  try {
    env = (await res.json()) as Envelope<T>;
  } catch {
    throw new ApiError(-1, `服务异常（HTTP ${res.status}）`, res.status);
  }

  if (env.code !== 0) {
    throw new ApiError(env.code, env.message || '请求失败', res.status);
  }
  return env.data as T;
}

/* ---- 语义化快捷方法 ---- */

export const http = {
  get: <T>(path: string, query?: RequestOptions['query']) =>
    request<T>(path, { method: 'GET', query }),
  post: <T>(path: string, body?: unknown) =>
    request<T>(path, { method: 'POST', body }),
  put: <T>(path: string, body?: unknown) =>
    request<T>(path, { method: 'PUT', body }),
  delete: <T>(path: string) => request<T>(path, { method: 'DELETE' }),
  upload: <T>(path: string, formData: FormData) =>
    request<T>(path, { method: 'POST', formData }),
};

/** 从 ApiError 提取可展示文案 */
export function errMsg(e: unknown): string {
  if (e instanceof ApiError) return e.message;
  if (e instanceof Error) return e.message;
  return '未知错误';
}
