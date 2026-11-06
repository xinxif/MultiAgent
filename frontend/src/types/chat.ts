/**
 * 对话域类型 —— 对应 model/auth.go(Session/Message)、agent/event.go(SSE)、
 * rag/engine Ref、runtime/planner Plan。
 */
import type { PageQuery } from './api';

/** 会话（ID 为 UUID，防遍历） */
export interface Session {
  id: string;
  user_id: number;
  agent_id?: number | null;
  title: string;
  status: 'active' | 'archived';
  pinned: boolean;
  message_count: number;
  token_usage: number;
  last_msg_at?: string | null;
  metadata?: Record<string, unknown>;
  created_at: string;
  updated_at: string;
}

/** 消息角色 */
export type MessageRole = 'user' | 'assistant' | 'system' | 'tool';

/**
 * 决策链步骤 —— messages[].metadata.agent_trace 数组元素，
 * 字段与 observability.AgentTraceCollector 完全对应。
 */
export interface TraceStep {
  seq: number;
  /** plan / thought / action / observe / delegate / merge / answer */
  type: string;
  agent: string;
  content: string;
  tool?: string;
  args?: Record<string, unknown>;
  result?: string;
  elapsed_ms: number;
  /** ok / error / timeout */
  status?: string;
  time: string;
}

/** RAG 引用（engine.Ref） */
export interface RAGRef {
  chunk_id: number;
  document_id: number;
  filename: string;
  page: number;
  content: string;
  score: number;
}

/** 历史消息（GET /sessions/:id/messages 元素） */
export interface Message {
  id: number;
  session_id: string;
  role: MessageRole;
  content: string;
  reasoning_content?: string;
  tool_calls?: unknown;
  tool_call_id?: string;
  refs?: RAGRef[] | null;
  model_name?: string;
  prompt_tokens: number;
  completion_tokens: number;
  first_token_ms: number;
  latency_ms: number;
  /** 1 完成 / 2 中断 / 3 失败 */
  status: 1 | 2 | 3;
  trace_id: string;
  /** 含 agent_trace: TraceStep[] 决策链 */
  metadata?: { agent_trace?: TraceStep[] } & Record<string, unknown>;
  created_at: string;
}

/** 会话列表查询 */
export type SessionQuery = PageQuery;

/* ============================================================
   SSE 事件协议 —— 与 agent/event.go 十二种事件一一对应
   序列：meta → (reasoning/plan/tool_call/tool_result/agent_event/refs/usage/approval)*
       → delta* → done | error
   ============================================================ */

export type SSEEventType =
  | 'meta'
  | 'delta'
  | 'reasoning'
  | 'plan'
  | 'tool_call'
  | 'tool_result'
  | 'agent_event'
  | 'refs'
  | 'usage'
  | 'approval'
  | 'done'
  | 'error';

/** meta：会话元信息（首个事件） */
export interface MetaData {
  session_id: string;
  agent: string;
  message_id: number;
  /** 多智能体 host 路由时携带命中的专家 */
  expert?: string;
}

/** delta / reasoning：增量文本 */
export interface DeltaData {
  content: string;
}

/** plan：Plan-and-Execute 计划 */
export interface PlanData {
  goal: string;
  steps: PlanStep[];
}

export interface PlanStep {
  id: string;
  description: string;
  depends_on: string[];
}

/** tool_call：开始调用工具 */
export interface ToolCallData {
  id: string;
  name: string;
  args: string; // JSON 字符串
}

/** tool_result：工具返回 */
export interface ToolResultData {
  id: string;
  result: string;
  ms: number;
  /** ok / error / timeout */
  status: string;
}

/** agent_event：多智能体委派/进度/聚合 */
export interface AgentEventData {
  /** delegate / progress / merge */
  type: string;
  expert: string;
  detail: string;
}

/** refs：RAG 引用 */
export interface RefsData {
  items: RAGRef[];
}

/** usage：Token 用量与成本 */
export interface UsageData {
  prompt_tokens: number;
  completion_tokens: number;
  cost: string;
  model: string;
}

/** done：正常结束 */
export interface DoneData {
  finish_reason: string;
}

/** error：失败结束 */
export interface ErrorData {
  code: number;
  message: string;
}

/** 对话请求体（POST /chat/stream） */
export interface ChatReq {
  session_id?: string;
  agent_id?: number;
  content: string;
  /** 本次发送使用的模型；0/未传 = 沿用 Agent 绑定模型 */
  model_config_id?: number;
}

/** 对话页模型选择器条目（GET /models） */
export interface ChatModelItem {
  id: number;
  /** 展示名（alias 优先） */
  label: string;
  /** 上游模型名 */
  model_name: string;
  /** 供应商展示名 */
  provider: string;
  is_default: boolean;
}
