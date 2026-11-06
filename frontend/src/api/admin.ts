/**
 * 管理接口（admin 角色）—— 对应 router.go admin 分组：
 * 模型供应商/模型配置、MCP Server、A2A 远程 Agent。
 */
import { http } from '@/lib/request';
import type {
  A2AAgent,
  MCPServer,
  ModelConfig,
  ModelProvider,
} from '@/types/admin';

/* ---- 模型供应商 ---- */

export const listProviders = () =>
  http.get<ModelProvider[]>('/admin/models/providers');

export const createProvider = (req: Partial<ModelProvider> & { api_key?: string }) =>
  http.post<ModelProvider>('/admin/models/providers', req);

export const updateProvider = (
  id: number,
  req: Partial<ModelProvider> & { api_key?: string },
) => http.put<ModelProvider>(`/admin/models/providers/${id}`, req);

export const deleteProvider = (id: number) =>
  http.delete(`/admin/models/providers/${id}`);

/* ---- 模型配置 ---- */

/** 列表（可按 type=chat/embedding/rerank 过滤） */
export const listModels = (type?: string) =>
  http.get<ModelConfig[]>('/admin/models', { type });

export const createModel = (req: Partial<ModelConfig>) =>
  http.post<ModelConfig>('/admin/models', req);

export const updateModel = (id: number, req: Partial<ModelConfig>) =>
  http.put<ModelConfig>(`/admin/models/${id}`, req);

export const deleteModel = (id: number) => http.delete(`/admin/models/${id}`);

/* ---- MCP Server ---- */

export const listMCPServers = () => http.get<MCPServer[]>('/admin/mcp/servers');

export const createMCPServer = (req: Partial<MCPServer>) =>
  http.post<MCPServer>('/admin/mcp/servers', req);

export const updateMCPServer = (id: number, req: Partial<MCPServer>) =>
  http.put<MCPServer>(`/admin/mcp/servers/${id}`, req);

export const deleteMCPServer = (id: number) =>
  http.delete(`/admin/mcp/servers/${id}`);

/** 同步工具：拉取远端工具清单写入 tools_cache + tools 表 */
export const syncMCPServer = (id: number) =>
  http.post<{ synced: number }>(`/admin/mcp/servers/${id}/sync`);

/** 健康检查 */
export const checkMCPServer = (id: number) =>
  http.post<{ status: string }>(`/admin/mcp/servers/${id}/health`);

/** 手动调用 MCP 工具调试：{name, args} → {output} */
export const callMCPTool = (id: number, name: string, args: Record<string, unknown>) =>
  http.post<{ output: unknown }>(`/admin/mcp/servers/${id}/call`, { name, args });

/* ---- A2A 远程 Agent ---- */

export const listA2AAgents = () => http.get<A2AAgent[]>('/admin/a2a/agents');

/** 注册：先拉 /.well-known/agent.json 名片验活再入库（name 留空取名片） */
export const registerA2AAgent = (req: { name?: string; base_url: string }) =>
  http.post<A2AAgent>('/admin/a2a/agents', req);

export const deleteA2AAgent = (id: number) =>
  http.delete(`/admin/a2a/agents/${id}`);

export const checkA2AAgent = (id: number) =>
  http.post<{ status: string }>(`/admin/a2a/agents/${id}/health`);

/** 委派任务：{text} → 任务结果 */
export const sendA2ATask = (id: number, text: string) =>
  http.post<Record<string, unknown>>(`/admin/a2a/agents/${id}/tasks`, { text });
