/**
 * 工作流域类型 —— 对应 model/workflow.go 与 workflow/dsl.go。
 */

/** 工作流定义（DSL 为 YAML 文本） */
export interface Workflow {
  id: number;
  name: string;
  description: string;
  dsl: string;
  version: number;
  is_active: boolean;
  creator_id: number;
  status: number;
  created_at: string;
  updated_at: string;
}

/** 执行状态 */
export type RunStatus =
  | 'running'
  | 'waiting_approval'
  | 'succeeded'
  | 'failed'
  | 'canceled';

/** 节点级执行快照 */
export interface WorkflowStepRun {
  id: number;
  run_id: number;
  node_key: string;
  /** llm / tool / kb / condition / parallel / human / subflow */
  node_type: string;
  status: string;
  input?: Record<string, unknown>;
  output?: Record<string, unknown>;
  error: string;
  tokens: number;
  started_at?: string | null;
  finished_at?: string | null;
}

/** 工作流执行记录 */
export interface WorkflowRun {
  id: number;
  workflow_id: number;
  trigger_type: string;
  status: RunStatus;
  input: Record<string, unknown>;
  output?: Record<string, unknown>;
  error: string;
  trace_id: string;
  started_at?: string | null;
  finished_at?: string | null;
  created_at: string;
  steps?: WorkflowStepRun[];
}

/* ---- DSL 结构（YAML 解析目标，供编辑器提示与校验错误对照） ---- */

export interface DSLNode {
  key: string;
  type: 'llm' | 'tool' | 'kb' | 'condition' | 'parallel' | 'human' | 'subflow';
  prompt?: string;
  model?: string;
  kb_id?: number;
  query?: string;
  tool_code?: string;
  args?: Record<string, unknown>;
  expr?: string;
  then?: string;
  else?: string;
  branches?: string[];
  workflow?: number;
  timeout_s?: number;
}

export interface DSLEdge {
  from: string;
  to: string;
  label?: string;
}

export interface WorkflowDSL {
  name: string;
  desc?: string;
  input?: string[];
  nodes: DSLNode[];
  edges?: DSLEdge[];
  out?: string;
}
