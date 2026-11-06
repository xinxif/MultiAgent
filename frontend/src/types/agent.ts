/**
 * Agent 与工具域类型 —— 对应 model/agent.go。
 */

/** Agent 配置（host 路由聚合 / expert 专项执行） */
export interface Agent {
  id: number;
  name: string;
  description: string;
  /** host / expert */
  type: 'host' | 'expert';
  system_prompt: string;
  model_config_id: number;
  fallback_model_config_id?: number | null;
  temperature: number;
  top_p: number;
  max_tokens: number;
  max_iterations: number;
  memory_enabled: boolean;
  memory_window: number;
  memory_long_enabled: boolean;
  /** 绑定知识库 ID 数组（JSONB） */
  kb_ids: number[];
  /** 工具绑定：[{tool_id, config}] */
  tools: { tool_id: number; config?: Record<string, unknown> }[];
  is_preset: boolean;
  config: Record<string, unknown>;
  status: number;
  created_at: string;
  updated_at: string;
}

/** 创建/更新 Agent 请求（partial 即可，后端整对象绑定） */
export type AgentReq = Partial<
  Omit<Agent, 'id' | 'created_at' | 'updated_at' | 'is_preset'>
>;

/** 工具（builtin / mcp / custom） */
export interface Tool {
  id: number;
  code: string;
  name: string;
  type: 'builtin' | 'mcp' | 'custom';
  description: string;
  /** JSON Schema */
  parameters: Record<string, unknown> | null;
  handler?: string;
  mcp_server_id?: number | null;
  endpoint?: string;
  auth_config?: Record<string, unknown>;
  default_config: Record<string, unknown>;
  timeout_ms: number;
  /** HITL 审批依据：危险工具执行前需人工确认 */
  is_dangerous: boolean;
  status: number;
  created_at: string;
  updated_at: string;
}
