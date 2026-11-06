/**
 * Agent 与工具接口 —— 对应 router.go Agent 分组。
 */
import { http } from '@/lib/request';
import type { Paged, PageQuery } from '@/types/api';
import type { Agent, AgentReq, Tool } from '@/types/agent';

/** Agent 列表（分页） */
export const listAgents = (q?: PageQuery) =>
  http.get<Paged<Agent>>('/agents', q);

/** Agent 详情 */
export const getAgent = (id: number) => http.get<Agent>(`/agents/${id}`);

/** 创建 Agent */
export const createAgent = (req: AgentReq) =>
  http.post<Agent>('/agents', req);

/** 更新 Agent */
export const updateAgent = (id: number, req: AgentReq) =>
  http.put<Agent>(`/agents/${id}`, req);

/** 删除 Agent */
export const deleteAgent = (id: number) => http.delete(`/agents/${id}`);

/** 工具清单（分页） */
export const listTools = (q?: PageQuery) =>
  http.get<Paged<Tool>>('/tools', q);
