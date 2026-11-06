/**
 * 统一响应包络 —— 对应 backend/internal/api/response.go。
 *
 * 所有 REST 接口返回 {code, message, data}：
 * - code = 0   成功，data 为业务数据
 * - code != 0  业务错误（errcode 分段：20xxx 权限 / 30xxx 模型 等）
 */

/** 通用响应包络 */
export interface Envelope<T = unknown> {
  code: number;
  message: string;
  data?: T;
}

/** 分页列表响应：handler 里统一为 {list, total} */
export interface Paged<T> {
  list: T[];
  total: number;
}

/**
 * 分页查询参数 —— 对应 pagination.Parse：page/page_size/keyword。
 * 用 type 而非 interface：类型别名的对象字面量才具有隐式索引签名，
 * 可直接传给请求层的 query 参数。
 */
export type PageQuery = {
  page?: number;
  page_size?: number;
  keyword?: string;
};
