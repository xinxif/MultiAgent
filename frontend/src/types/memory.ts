/**
 * 记忆域类型 —— 对应 model/memory.go。
 */

/** 长期记忆 */
export interface Memory {
  id: number;
  user_id: number;
  agent_id?: number | null;
  content: string;
  vector_id: string;
  /** user / agent */
  scope: 'user' | 'agent';
  /** 1-10 重要度 */
  importance: number;
  hit_count: number;
  last_hit_at?: string | null;
  /** 1 生效 / 2 归档 */
  status: 1 | 2;
  created_at: string;
  updated_at: string;
}

/** 手动抽取记忆请求（POST /memories/extract） */
export interface ExtractMemoryReq {
  session_id: string;
}
