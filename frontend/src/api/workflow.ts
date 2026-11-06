/**
 * 工作流接口 —— 对应 router.go 工作流分组。
 * DSL 为 YAML 文本；执行为同步接口；HITL 走 runs/:id/approve。
 */
import { http } from '@/lib/request';
import type { Paged, PageQuery } from '@/types/api';
import type { Workflow, WorkflowRun } from '@/types/workflow';

/** 工作流列表 */
export const listWorkflows = (q?: PageQuery) =>
  http.get<Paged<Workflow>>('/workflows', q);

/** 工作流详情 */
export const getWorkflow = (id: number) => http.get<Workflow>(`/workflows/${id}`);

/** 创建工作流（name/description/dsl） */
export const createWorkflow = (req: Partial<Workflow>) =>
  http.post<Workflow>('/workflows', req);

/** 更新工作流 */
export const updateWorkflow = (id: number, req: Partial<Workflow>) =>
  http.put<Workflow>(`/workflows/${id}`, req);

/** 删除工作流 */
export const deleteWorkflow = (id: number) => http.delete(`/workflows/${id}`);

/** DSL 静态校验：{dsl} → {valid:true}；不合法抛 ApiError(message 为校验错误) */
export const validateWorkflow = (dsl: string) =>
  http.post<{ valid: boolean }>('/workflows/validate', { dsl });

/** 同步执行：{workflow_id, input} → run（succeeded/failed/waiting_approval） */
export const runWorkflow = (workflowId: number, input: Record<string, unknown>) =>
  http.post<WorkflowRun>('/workflows/run', {
    workflow_id: workflowId,
    input,
  });

/** 执行记录列表（可按 workflow_id 过滤） */
export const listRuns = (q?: PageQuery & { workflow_id?: number }) =>
  http.get<Paged<WorkflowRun>>('/workflows/runs', q);

/** 执行详情（含 steps 轨迹） */
export const getRun = (id: number) => http.get<WorkflowRun>(`/workflows/runs/${id}`);

/** HITL 审批：通过/驳回 */
export const approveRun = (id: number, approved: boolean, comment?: string) =>
  http.post<WorkflowRun>(`/workflows/runs/${id}/approve`, {
    approved,
    comment: comment ?? '',
  });
