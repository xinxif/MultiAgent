/**
 * 认证域类型 —— 对应 model/auth.go 与 security/jwt.go。
 */

/** 当前用户信息（GET /api/v1/me 返回字段） */
export interface User {
  id: number;
  username: string;
  email?: string;
  role: 'admin' | 'user';
  avatar_url?: string;
}

/** 双 Token（POST /auth/login 与 /auth/refresh 的 data） */
export interface TokenPair {
  access_token: string;
  refresh_token: string;
  /** access token 有效期（秒） */
  expires_in?: number;
}

/** 注册请求 */
export interface RegisterReq {
  username: string;
  email: string;
  password: string;
}

/** 登录请求 */
export interface LoginReq {
  username: string;
  password: string;
}
