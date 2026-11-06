/**
 * 管理域类型 —— 对应 model/llm.go 与 model/agent.go（MCP/A2A）。
 * 仅 admin 角色可访问对应接口。
 */

/** 模型供应商（APIKey AES-GCM 加密存储，仅回显打码摘要） */
export interface ModelProvider {
  id: number;
  /** deepseek / qwen / zhipu / moonshot / doubao / ollama */
  code: string;
  name: string;
  base_url: string;
  /** 打码摘要（前2位+****+后4位）；"" = 未配置 Key */
  api_key_masked: string;
  status: number;
  created_at: string;
  updated_at: string;
}

/** 模型类型 */
export type ModelType = 'chat' | 'embedding' | 'rerank';

/** 模型配置 */
export interface ModelConfig {
  id: number;
  provider_id: number;
  /** 上游模型名 */
  model_name: string;
  /** 平台展示别名 */
  alias: string;
  type: ModelType;
  context_window: number;
  max_output: number;
  /** 元 / 1M tokens */
  input_price: number;
  output_price: number;
  /** 故障转移顺序，小者优先 */
  priority: number;
  /** ["vision","function_call","reasoning"] */
  capabilities: string[];
  is_default: boolean;
  status: number;
  created_at: string;
  updated_at: string;
  provider?: ModelProvider;
}

/** MCP 服务端配置 */
export interface MCPServer {
  id: number;
  name: string;
  /** streamable_http / stdio / sse */
  transport: string;
  url: string;
  command: string;
  args: string[];
  env: Record<string, string>;
  headers: Record<string, string>;
  /** disconnected / connected / error ... */
  status: string;
  last_health_at?: string | null;
  tools_cache: unknown[];
  created_at: string;
  updated_at: string;
}

/** A2A 远程 Agent（名片快照在 card 字段） */
export interface A2AAgent {
  id: number;
  name: string;
  base_url: string;
  /** Agent Card（A2A 协议 /.well-known/agent.json） */
  card: Record<string, unknown> | null;
  status: string;
  last_health_at?: string | null;
  created_at: string;
  updated_at: string;
}
